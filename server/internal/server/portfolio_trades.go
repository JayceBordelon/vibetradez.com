package server

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"vibetradez.com/internal/store"
)

/*
Holdings + closed-trades API — the per-position surfaces behind /holdings
and /closed on the client.

Open holdings come from the current book (live broker positions, falling
back to the latest snapshot, same source the dashboard uses), enriched with
the opening move's rationale from the decision log. Closed trades are
DERIVED from that same decision log: we walk it forward and pair each buy
with the sells that flatten it (average-cost), emitting one closed trade per
completed round trip. There is no separate closed-trades table, the decision
log is the source of truth.

Every trade carries a stable id (the symbol with spaces stripped, so an OCC
option symbol is URL-safe; closed trades append the close date so a name
that was traded more than once stays unique) and a kind ("stock"|"option")
so the client can route to /<holdings|closed>/<kind>/<id> and render the two
asset classes differently.
*/

type holdingView struct {
	ID            string  `json:"id"`
	Kind          string  `json:"kind"` // equity | option
	Symbol        string  `json:"symbol"`
	Underlying    string  `json:"underlying"`
	Label         string  `json:"label"`
	ContractType  string  `json:"contract_type,omitempty"`
	Strike        float64 `json:"strike,omitempty"`
	Expiration    string  `json:"expiration,omitempty"`
	DTE           int     `json:"dte,omitempty"`
	Quantity      float64 `json:"quantity"`
	MarketValue   float64 `json:"market_value"`
	CostBasis     float64 `json:"cost_basis"`
	UnrealizedPnl float64 `json:"unrealized_pnl"`
	OpenedDate    string  `json:"opened_date,omitempty"`
	OpenRationale string  `json:"open_rationale,omitempty"`
}

type holdingsResponse struct {
	Holdings        []holdingView `json:"holdings"`
	PositionsSource string        `json:"positions_source,omitempty"`
	PositionsAsOf   string        `json:"positions_as_of,omitempty"`
}

type closedTradeView struct {
	ID             string  `json:"id"`
	Kind           string  `json:"kind"` // equity | option
	Symbol         string  `json:"symbol"`
	Underlying     string  `json:"underlying"`
	Label          string  `json:"label"`
	ContractType   string  `json:"contract_type,omitempty"`
	Strike         float64 `json:"strike,omitempty"`
	Expiration     string  `json:"expiration,omitempty"`
	Quantity       float64 `json:"quantity"`
	OpenedDate     string  `json:"opened_date"`
	ClosedDate     string  `json:"closed_date"`
	EntryPrice     float64 `json:"entry_price"`
	ExitPrice      float64 `json:"exit_price"`
	CostBasis      float64 `json:"cost_basis"`
	Proceeds       float64 `json:"proceeds"`
	RealizedPnl    float64 `json:"realized_pnl"`
	RealizedPct    float64 `json:"realized_pct"`
	HoldDays       int     `json:"hold_days"`
	OpenRationale  string  `json:"open_rationale,omitempty"`
	CloseRationale string  `json:"close_rationale,omitempty"`
}

type closedTradesResponse struct {
	Trades []closedTradeView `json:"trades"`
}

/*
handlePortfolioHoldings serves every open position for the /holdings page,
enriched with the opening move's date + rationale from the decision log.
*/
func (s *Server) handlePortfolioHoldings(w http.ResponseWriter, r *http.Request) {
	views, source, asOf := s.currentPositions(r.Context())

	opens := openingMoves(s.db)
	resp := holdingsResponse{Holdings: []holdingView{}, PositionsSource: source, PositionsAsOf: asOf}
	for _, v := range views {
		h := holdingView{
			ID:            compactSymbol(v.Symbol), // fallback when no opening buy is logged
			Kind:          kindOf(v.AssetType),
			Symbol:        v.Symbol,
			Underlying:    v.Underlying,
			Label:         instrumentLabel(v.AssetType, v.Underlying, v.Symbol, v.ContractType, v.Strike, v.Expiration),
			ContractType:  v.ContractType,
			Strike:        v.Strike,
			Expiration:    v.Expiration,
			DTE:           dteFromExpiration(v.Expiration),
			Quantity:      v.Quantity,
			MarketValue:   v.MarketValue,
			CostBasis:     v.CostBasis,
			UnrealizedPnl: v.UnrealizedPnl,
		}
		if oi, ok := opens[v.Symbol]; ok {
			if oi.uuid != "" {
				h.ID = oi.uuid
			}
			h.OpenedDate = oi.date
			h.OpenRationale = oi.rationale
		}
		resp.Holdings = append(resp.Holdings, h)
	}
	writeJSON(w, http.StatusOK, resp)
}

/*
handlePortfolioClosed serves every completed round trip for the /closed
page, derived from the decision log.
*/
func (s *Server) handlePortfolioClosed(w http.ResponseWriter, r *http.Request) {
	resp := closedTradesResponse{Trades: []closedTradeView{}}
	if decisions, err := s.db.AllPortfolioDecisions(); err == nil {
		resp.Trades = deriveClosedTrades(decisions)
	}
	writeJSON(w, http.StatusOK, resp)
}

/*
currentPositions returns the current book: live from the broker when one is
wired and reporting, otherwise the latest persisted snapshot. Mirrors the
source-selection the dashboard handler uses.
*/
func (s *Server) currentPositions(ctx context.Context) (views []portfolioPositionView, source, asOf string) {
	if s.executor != nil {
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if positions, err := s.executor.GetPositionsAgent(cctx); err == nil && len(positions) > 0 {
			for _, bp := range positions {
				views = append(views, positionView(bp))
			}
			return views, "live", ""
		}
	}
	if rows, err := s.db.GetLatestPositions(); err == nil && len(rows) > 0 {
		for _, p := range rows {
			views = append(views, snapshotPositionView(p))
			asOf = p.Date
		}
		return views, "snapshot", asOf
	}
	return nil, "", ""
}

type openMove struct {
	date      string
	rationale string
	uuid      string
}

// openingMoves maps each symbol to its most recent opening buy (date +
// rationale + the buy's stable uuid), so a holding can show why the model
// entered it and route to a detail page keyed by that uuid.
func openingMoves(db *store.Store) map[string]openMove {
	out := map[string]openMove{}
	decisions, err := db.AllPortfolioDecisions()
	if err != nil {
		return out
	}
	for _, d := range decisions {
		if d.Action == "buy_equity" || d.Action == "buy_option" {
			// chronological order means the last write wins = most recent buy
			out[d.Symbol] = openMove{date: d.Date, rationale: d.Rationale, uuid: d.Uuid}
		}
	}
	return out
}

// tradeAcc accumulates an open round trip until a sell flattens it.
type tradeAcc struct {
	openQty      float64
	openCost     float64
	soldQty      float64
	proceeds     float64
	costConsumed float64
	openDate     string
	closeDate    string
	openReason   string
	closeReason  string
	uuid         string // the opening buy's stable uuid; becomes the trade's id
	assetType    string
	underlying   string
	contractType string
	expiration   string
	strike       *float64
}

const qtyEpsilon = 1e-6

/*
deriveClosedTrades walks the decision log forward, pairing buys with the
sells that flatten them (average-cost), and emits one closed trade per
completed round trip. Sells with no matching open lot are ignored (they
can't form a round trip we have the entry for). Result is newest-close
first.
*/
func deriveClosedTrades(decisions []store.PortfolioDecisionRow) []closedTradeView {
	open := map[string]*tradeAcc{}
	var out []closedTradeView

	for _, d := range decisions {
		switch d.Action {
		case "buy_equity", "buy_option":
			a := open[d.Symbol]
			if a == nil {
				a = &tradeAcc{}
				open[d.Symbol] = a
			}
			if a.openQty <= qtyEpsilon {
				// start of a fresh cycle: capture entry context
				a.openDate = d.Date
				a.openReason = d.Rationale
				a.uuid = d.Uuid
				a.assetType = d.AssetType
				a.underlying = d.Underlying
				a.contractType = d.ContractType
				a.expiration = d.Expiration
				a.strike = d.Strike
				a.soldQty, a.proceeds, a.costConsumed = 0, 0, 0
			}
			mult := assetMult(d.AssetType)
			cost := d.Quantity * decisionPrice(d) * mult
			a.openQty += d.Quantity
			a.openCost += cost

		case "sell_equity", "sell_option":
			a := open[d.Symbol]
			if a == nil || a.openQty <= qtyEpsilon {
				continue // no open lot to close against
			}
			mult := assetMult(d.AssetType)
			sellQty := d.Quantity
			if sellQty > a.openQty {
				sellQty = a.openQty
			}
			avgCost := a.openCost / a.openQty
			costPortion := avgCost * sellQty
			a.openQty -= sellQty
			a.openCost -= costPortion
			a.soldQty += sellQty
			a.proceeds += sellQty * decisionPrice(d) * mult
			a.costConsumed += costPortion
			a.closeReason = d.Rationale
			a.closeDate = d.Date
			if a.openQty <= qtyEpsilon {
				out = append(out, a.build(d.Symbol))
				delete(open, d.Symbol)
			}
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].ClosedDate > out[j].ClosedDate })
	return out
}

func (a *tradeAcc) build(symbol string) closedTradeView {
	mult := assetMult(a.assetType)
	units := a.soldQty * mult
	entry, exit := 0.0, 0.0
	if units > 0 {
		entry = a.costConsumed / units
		exit = a.proceeds / units
	}
	realized := a.proceeds - a.costConsumed
	pct := 0.0
	if a.costConsumed > 0 {
		pct = realized / a.costConsumed * 100
	}
	strike := 0.0
	if a.strike != nil {
		strike = *a.strike
	}
	id := a.uuid
	if id == "" {
		id = compactSymbol(symbol) + "-" + a.closeDate // legacy rows predating the uuid column
	}
	return closedTradeView{
		ID:             id,
		Kind:           kindOf(a.assetType),
		Symbol:         symbol,
		Underlying:     a.underlying,
		Label:          instrumentLabel(a.assetType, a.underlying, symbol, a.contractType, strike, a.expiration),
		ContractType:   a.contractType,
		Strike:         strike,
		Expiration:     a.expiration,
		Quantity:       a.soldQty,
		OpenedDate:     a.openDate,
		ClosedDate:     a.closeDate,
		EntryPrice:     entry,
		ExitPrice:      exit,
		CostBasis:      a.costConsumed,
		Proceeds:       a.proceeds,
		RealizedPnl:    realized,
		RealizedPct:    pct,
		HoldDays:       daysBetween(a.openDate, a.closeDate),
		OpenRationale:  a.openReason,
		CloseRationale: a.closeReason,
	}
}

// decisionPrice prefers the recorded fill, falling back to the limit price
// (the fire-and-forget LIMIT the order was submitted at).
func decisionPrice(d store.PortfolioDecisionRow) float64 {
	if d.FillPrice != nil && *d.FillPrice > 0 {
		return *d.FillPrice
	}
	return d.LimitPrice
}

func assetMult(assetType string) float64 {
	if assetType == "OPTION" {
		return 100
	}
	return 1
}

func kindOf(assetType string) string {
	if assetType == "OPTION" {
		return "option"
	}
	return "stock"
}

// compactSymbol strips the spaces an OCC option symbol carries so it is
// safe in a URL path. Equity tickers are unchanged.
func compactSymbol(symbol string) string {
	return strings.ReplaceAll(symbol, " ", "")
}

// instrumentLabel renders the human label: just the ticker, for both equity
// and options. An option's side, strike, and expiry surface in its own badge
// and fact rows, so the heading stays the clean underlying symbol (e.g. AVGO).
func instrumentLabel(assetType, underlying, symbol, contractType string, strike float64, expiration string) string {
	if underlying != "" {
		return underlying
	}
	if assetType == "OPTION" {
		return strings.TrimSpace(symbol[:min(6, len(symbol))])
	}
	return symbol
}

// dteFromExpiration returns calendar days to expiration from today (ET).
// 0 for equity (no expiration) or unparseable dates.
func dteFromExpiration(expiration string) int {
	if expiration == "" {
		return 0
	}
	exp, err := time.ParseInLocation("2006-01-02", expiration, easternLoc())
	if err != nil {
		return 0
	}
	today := time.Now().In(easternLoc())
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, easternLoc())
	d := int(exp.Sub(today).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

func daysBetween(start, end string) int {
	s, err1 := time.Parse("2006-01-02", start)
	e, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil {
		return 0
	}
	d := int(e.Sub(s).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}
