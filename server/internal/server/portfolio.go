package server

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/store"
)

/*
Portfolio API — the v2 dashboard surface. GET /api/portfolio returns the
live book (positions + cash + equity from the broker) joined with today's
agent stance + committed moves from the store. GET /api/portfolio/equity-curve
returns the daily equity-vs-SPY series for the chart.

This is a single personal brokerage account, not a multi-user product: the
endpoints expose ONE book. They are read-only and safe when portfolio mode
is off — `enabled:false` with empty data, so the dashboard renders an
honest "not running" state instead of erroring.
*/

type portfolioPositionView struct {
	Symbol        string  `json:"symbol"`
	Underlying    string  `json:"underlying"`
	AssetType     string  `json:"asset_type"`
	ContractType  string  `json:"contract_type,omitempty"`
	Strike        float64 `json:"strike,omitempty"`
	Expiration    string  `json:"expiration,omitempty"`
	DTE           int     `json:"dte,omitempty"`
	Quantity      float64 `json:"quantity"`
	MarketValue   float64 `json:"market_value"`
	CostBasis     float64 `json:"cost_basis"`
	UnrealizedPnl float64 `json:"unrealized_pnl"`
	// Day-split anchors. OpenValue is the position valued at today's
	// opening print (live quote, works for stocks and OCC option symbols
	// alike). PrevCloseValue is the position's mark in OUR last EOD
	// snapshot before today; its presence is also the held-overnight
	// signal (a position opened today has none, so it gets no fabricated
	// overnight component). Either is omitted when its source has nothing.
	OpenValue      float64 `json:"open_value,omitempty"`
	PrevCloseValue float64 `json:"prev_close_value,omitempty"`
	// TodayCloseValue is the position's mark in TODAY's EOD snapshot,
	// present only after the 16:00 cron: its presence tells the UI the
	// session is over and live drift is overnight movement.
	TodayCloseValue float64 `json:"today_close_value,omitempty"`
}

type portfolioDecisionView struct {
	Action       string   `json:"action"`
	AssetType    string   `json:"asset_type,omitempty"`
	Symbol       string   `json:"symbol,omitempty"`
	Underlying   string   `json:"underlying,omitempty"`
	ContractType string   `json:"contract_type,omitempty"`
	Strike       *float64 `json:"strike,omitempty"`
	Expiration   string   `json:"expiration,omitempty"`
	Quantity     float64  `json:"quantity,omitempty"`
	LimitPrice   float64  `json:"limit_price,omitempty"`
	Notional     float64  `json:"notional,omitempty"`
	OrderID      string   `json:"order_id,omitempty"`
	Status       string   `json:"status,omitempty"`
	Rationale    string   `json:"rationale"`
	// Navigation link for the executions tape: the trade page this move
	// belongs to. A move on a still-held symbol points at the holding
	// (trade_kind "holding", trade_id = the opening decision's uuid); the
	// sell that completed a round trip points at the closed trade.
	TradeID   string `json:"trade_id,omitempty"`
	TradeKind string `json:"trade_kind,omitempty"`
}

type portfolioResponse struct {
	// Enabled is false when the account/broker isn't wired (trading
	// disabled at startup). The dashboard shows a "not running" state.
	Enabled       bool                    `json:"enabled"`
	Mode          string                  `json:"mode"` // always "live"
	Date          string                  `json:"date"`
	Equity        float64                 `json:"equity"`
	SettledCash   float64                 `json:"settled_cash"`
	UnsettledCash float64                 `json:"unsettled_cash"`
	HighWaterMark float64                 `json:"high_water_mark"`
	SPYClose      float64                 `json:"spy_close"`
	Positions     []portfolioPositionView `json:"positions"`
	// PositionsSource is "live" (from the broker) or "snapshot" (the last
	// recorded book, shown when the live broker is flat/unreachable).
	PositionsSource string                  `json:"positions_source,omitempty"`
	PositionsAsOf   string                  `json:"positions_as_of,omitempty"`
	Stance          string                  `json:"stance"`
	Summary         string                  `json:"summary,omitempty"`
	ActionItems     string                  `json:"action_items,omitempty"`
	Decisions       []portfolioDecisionView `json:"decisions"`
}

type equityCurvePointView struct {
	Date          string  `json:"date"`
	AccountEquity float64 `json:"account_equity"`
	SettledCash   float64 `json:"settled_cash"`
	UnsettledCash float64 `json:"unsettled_cash"`
	HighWaterMark float64 `json:"high_water_mark"`
	SPYClose      float64 `json:"spy_close"`
	// P&L decomposition for the dashboard chart: cumulative realized P&L of
	// round trips closed inside the requested window, and the open book's
	// unrealized P&L from that day's EOD snapshot (zero on flat days).
	RealizedCum float64 `json:"realized_cum"`
	Unrealized  float64 `json:"unrealized"`
	// Live marks the synthetic intraday point the endpoint appends before
	// today's EOD snapshot lands, so clients can tell "today's real close
	// exists" (post-close) from "this is the live placeholder" (intraday).
	Live bool `json:"live,omitempty"`
}

type equityCurveResponse struct {
	Points []equityCurvePointView `json:"points"`
}

// positionValuePointView is one day of a single position's snapshot value,
// for the trade-detail chart.
type positionValuePointView struct {
	Date        string  `json:"date"`
	MarketValue float64 `json:"market_value"`
	Quantity    float64 `json:"quantity"`
	CostBasis   float64 `json:"cost_basis"`
}

type positionHistoryResponse struct {
	Symbol string                   `json:"symbol"`
	Points []positionValuePointView `json:"points"`
}

// pricePointView is one daily close from the market-data price history,
// for the trade-detail chart's underlying line.
type pricePointView struct {
	Date  string  `json:"date"`
	Close float64 `json:"close"`
}

type priceHistoryResponse struct {
	Symbol string           `json:"symbol"`
	Points []pricePointView `json:"points"`
}

/*
handlePortfolio serves the live book + today's agent activity. Positions
and cash come live from the broker (via the executor); the stance and the
day's committed moves come from the store. When no executor is configured
(trading disabled), returns enabled:false with empty data.
*/
func (s *Server) handlePortfolio(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().In(easternLoc()).Format("2006-01-02")
	}

	resp := portfolioResponse{
		Date:      date,
		Positions: []portfolioPositionView{},
		Decisions: []portfolioDecisionView{},
	}

	// Store-backed fields work regardless of trading wiring.
	if mode, stance, summary, actionItems, ok, err := s.db.GetPortfolioSession(date); err == nil && ok {
		resp.Mode = mode
		resp.Stance = stance
		resp.Summary = summary
		resp.ActionItems = actionItems
	}
	if decisions, err := s.db.GetPortfolioDecisions(date); err == nil {
		for _, d := range decisions {
			resp.Decisions = append(resp.Decisions, decisionView(d))
		}
	}
	// Latest equity-curve point seeds SPY + high-water, and is the aggregate
	// fallback for equity/cash when the live broker isn't the source.
	var curveCash float64
	haveCurve := false
	if pts, err := s.db.GetEquityCurve("0000-00-00", date); err == nil && len(pts) > 0 {
		last := pts[len(pts)-1]
		resp.SPYClose = last.SPYClose
		resp.HighWaterMark = last.HighWaterMark
		curveCash = last.SettledCash
		haveCurve = true
	}

	// Live book from the broker, best-effort.
	liveBook := false
	if s.executor != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if positions, err := s.executor.GetPositionsAgent(ctx); err == nil {
			resp.Enabled = true
			resp.Mode = s.executor.Mode()
			var positionsValue float64
			for _, bp := range positions {
				pv := positionView(bp)
				positionsValue += pv.MarketValue
				resp.Positions = append(resp.Positions, pv)
			}
			if settled, ferr := s.executor.AvailableFundsAgent(ctx); ferr == nil {
				resp.SettledCash = settled
				resp.Equity = positionsValue + settled
			}
			if len(positions) > 0 {
				liveBook = true
				resp.PositionsSource = "live"
			}
		}
	}

	// Fall back to the last recorded book when the live broker is flat or
	// unreachable, so the holdings table still shows the last known book.
	if !liveBook {
		if rows, err := s.db.GetLatestPositions(); err == nil && len(rows) > 0 {
			resp.Enabled = true
			resp.PositionsSource = "snapshot"
			var positionsValue float64
			for _, p := range rows {
				v := snapshotPositionView(p)
				positionsValue += v.MarketValue
				resp.Positions = append(resp.Positions, v)
				resp.PositionsAsOf = p.Date
			}
			// Derive equity from the actual holdings plus the recorded cash so
			// the summary reconciles with the holdings table (positions + cash).
			if haveCurve {
				resp.SettledCash = curveCash
				resp.Equity = positionsValue + curveCash
			}
		}
	}

	if haveCurve {
		resp.Enabled = true
	}
	if resp.Equity > resp.HighWaterMark {
		resp.HighWaterMark = resp.Equity
	}
	s.attachDayAnchors(resp.Positions, date)

	// Link each of today's moves to its trade page, AFTER the book has
	// resolved: a "holding" link is only valid for a symbol actually in
	// the current positions (an unfilled or rejected buy has an opening
	// decision but no holding to open).
	if len(resp.Decisions) > 0 {
		held := make(map[string]bool, len(resp.Positions))
		for _, p := range resp.Positions {
			held[p.Symbol] = true
		}
		linkDecisionTrades(resp.Decisions, date, held, openingMoves(s.db), s.closedTrades())
	}

	writeJSON(w, http.StatusOK, resp)
}

// snapshotPositionView maps a persisted book snapshot row to the holdings
// view (used when the live broker book is unavailable).
func snapshotPositionView(p store.PortfolioPositionRow) portfolioPositionView {
	strike := 0.0
	if p.Strike != nil {
		strike = *p.Strike
	}
	return portfolioPositionView{
		Symbol:        p.Symbol,
		Underlying:    p.Underlying,
		AssetType:     p.AssetType,
		ContractType:  p.ContractType,
		Strike:        strike,
		Expiration:    p.Expiration,
		Quantity:      p.Quantity,
		MarketValue:   p.MarketValue,
		CostBasis:     p.CostBasis,
		UnrealizedPnl: p.MarketValue - p.CostBasis,
	}
}

/*
handlePortfolioEquityCurve serves the daily equity-vs-SPY series for the
dashboard chart. Optional ?start=&end= (YYYY-MM-DD); defaults to the
trailing year through today.
*/
func (s *Server) handlePortfolioEquityCurve(w http.ResponseWriter, r *http.Request) {
	end := r.URL.Query().Get("end")
	if end == "" {
		end = time.Now().In(easternLoc()).Format("2006-01-02")
	}
	start := r.URL.Query().Get("start")
	if start == "" {
		start = time.Now().In(easternLoc()).AddDate(-1, 0, 0).Format("2006-01-02")
	}
	resp := equityCurveResponse{Points: []equityCurvePointView{}}
	if pts, err := s.db.GetEquityCurve(start, end); err == nil {
		for _, p := range pts {
			resp.Points = append(resp.Points, equityCurvePointView{
				Date:          p.Date,
				AccountEquity: p.AccountEquity,
				SettledCash:   p.SettledCash,
				UnsettledCash: p.UnsettledCash,
				HighWaterMark: p.HighWaterMark,
				SPYClose:      p.SPYClose,
			})
		}
	}
	// Append a synthetic LIVE point for today when the broker book is
	// readable and the EOD snapshot hasn't landed yet, so the chart (and
	// its headline stats) agree with the live stat strip instead of
	// trailing it by a day. The decoration below picks up trades closed
	// today automatically because the point is dated today.
	today := time.Now().In(easternLoc()).Format("2006-01-02")
	if s.executor != nil && end >= today && (len(resp.Points) == 0 || resp.Points[len(resp.Points)-1].Date < today) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if positions, err := s.executor.GetPositionsAgent(ctx); err == nil {
			if settled, ferr := s.executor.AvailableFundsAgent(ctx); ferr == nil {
				var value, unrealLive float64
				for _, bp := range positions {
					pv := positionView(bp)
					value += pv.MarketValue
					unrealLive += pv.UnrealizedPnl
				}
				live := equityCurvePointView{
					Date:          today,
					AccountEquity: value + settled,
					SettledCash:   settled,
					Unrealized:    unrealLive,
					Live:          true,
				}
				// Live SPY for the benchmark line; falls back to the prior
				// point's close (flat segment) if the quote is unavailable.
				if s.schwab != nil {
					if quotes, qerr := s.schwab.GetQuotes([]string{"SPY"}); qerr == nil {
						q := quotes["SPY"]
						if q.Mark > 0 {
							live.SPYClose = q.Mark
						} else if q.LastPrice > 0 {
							live.SPYClose = q.LastPrice
						}
					}
				}
				if live.SPYClose == 0 && len(resp.Points) > 0 {
					live.SPYClose = resp.Points[len(resp.Points)-1].SPYClose
				}
				resp.Points = append(resp.Points, live)
			}
		}
	}

	// Decorate the curve with the P&L decomposition: daily unrealized from
	// the EOD book snapshots and cumulative realized from the decision log.
	// Both degrade to zeros so a store hiccup never blanks the chart.
	unreal, _ := s.db.GetDailyUnrealized(start, end)
	var closed []closedTradeView
	if decisions, err := s.db.AllPortfolioDecisions(); err == nil {
		closed = deriveClosedTrades(decisions)
	}
	resp.Points = decoratePnlSeries(resp.Points, unreal, closed)
	writeJSON(w, http.StatusOK, resp)
}

/*
decoratePnlSeries fills each curve point's RealizedCum and Unrealized.
RealizedCum accumulates round trips closed inside the window (a trade
closed on a non-curve day, e.g. a half-session, lands on the next curve
point), starting from zero at the window's first point so the series
decomposes the window's P&L the same way the SPY comparison does.
*/
func decoratePnlSeries(points []equityCurvePointView, unrealizedByDate map[string]float64, closed []closedTradeView) []equityCurvePointView {
	if len(points) == 0 {
		return points
	}
	// Oldest-first realized events, then a two-pointer walk over the curve.
	sort.SliceStable(closed, func(i, j int) bool { return closed[i].ClosedDate < closed[j].ClosedDate })
	realized := 0.0
	ci := 0
	for ; ci < len(closed) && closed[ci].ClosedDate < points[0].Date; ci++ {
		// Trades closed before the window start are not part of this
		// window's decomposition.
	}
	for i := range points {
		for ; ci < len(closed) && closed[ci].ClosedDate <= points[i].Date; ci++ {
			realized += closed[ci].RealizedPnl
		}
		points[i].RealizedCum = realized
		// A synthetic live point arrives carrying its own (live) unrealized
		// value; only EOD points read from the snapshot map.
		if v, ok := unrealizedByDate[points[i].Date]; ok || points[i].Unrealized == 0 {
			points[i].Unrealized = v
		}
	}
	return points
}

func positionView(bp exec.BrokerPosition) portfolioPositionView {
	mult := 1.0
	if bp.AssetType == "OPTION" {
		mult = 100
	}
	cost := bp.AverageCost * bp.Quantity * mult
	return portfolioPositionView{
		Symbol:        bp.Symbol,
		Underlying:    bp.Underlying,
		AssetType:     bp.AssetType,
		ContractType:  bp.ContractType,
		Strike:        bp.Strike,
		Expiration:    bp.Expiration,
		Quantity:      bp.Quantity,
		MarketValue:   bp.MarketValue,
		CostBasis:     cost,
		UnrealizedPnl: bp.MarketValue - cost,
	}
}

/*
handlePortfolioPositionHistory serves a single position's per-day snapshot
values (?symbol=, the broker symbol: ticker or OCC option symbol) for the
trade-detail value-over-time chart. Always 200 with whatever exists, so a
just-opened position simply charts from its first snapshot.
*/
func (s *Server) handlePortfolioPositionHistory(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	resp := positionHistoryResponse{Symbol: symbol, Points: []positionValuePointView{}}
	if symbol == "" {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if pts, err := s.db.GetPositionHistory(symbol); err == nil {
		for _, p := range pts {
			resp.Points = append(resp.Points, positionValuePointView(p))
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

/*
handlePriceHistory serves daily closes for an equity symbol (?symbol=,
optional ?start= / ?end= YYYY-MM-DD bounds), for the trade-detail chart's
underlying line. Degrades to an empty series when Schwab is unreachable or
the symbol has no history, so the chart's other series still render.
*/
func (s *Server) handlePriceHistory(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("symbol")))
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	resp := priceHistoryResponse{Symbol: symbol, Points: []pricePointView{}}
	if symbol == "" || s.schwab == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	ph, err := s.schwab.GetDailyPriceHistory(symbol)
	if err != nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	loc := easternLoc()
	for _, c := range ph.Candles {
		date := time.UnixMilli(c.Datetime).In(loc).Format("2006-01-02")
		if (start != "" && date < start) || (end != "" && date > end) {
			continue
		}
		resp.Points = append(resp.Points, pricePointView{Date: date, Close: c.Close})
	}
	writeJSON(w, http.StatusOK, resp)
}

// closedTrades derives the round-trip log once; degrades to nil on a store
// hiccup so a link miss never blanks the response.
func (s *Server) closedTrades() []closedTradeView {
	all, err := s.db.AllPortfolioDecisions()
	if err != nil {
		return nil
	}
	return deriveClosedTrades(all)
}

/*
linkDecisionTrades points each of today's moves at its trade page so the
executions tape can navigate. A sell whose symbol + date matches a round
trip's close links to the closed trade; any other move on a CURRENTLY HELD
symbol with a logged opening buy links to the holding (whose id IS that
opening decision's uuid). Everything else stays unlinked: an unfilled or
rejected buy has an opening decision but no holding page to open.
*/
func linkDecisionTrades(decisions []portfolioDecisionView, date string, held map[string]bool, opens map[string]openMove, closed []closedTradeView) {
	closedByKey := make(map[string]string, len(closed))
	for _, c := range closed {
		closedByKey[c.Symbol+"|"+c.ClosedDate] = c.ID
	}
	for i := range decisions {
		d := &decisions[i]
		if strings.HasPrefix(d.Action, "sell") {
			if id, ok := closedByKey[d.Symbol+"|"+date]; ok && id != "" {
				d.TradeID, d.TradeKind = id, "closed"
				continue
			}
		}
		if oi, ok := opens[d.Symbol]; ok && oi.uuid != "" && held[d.Symbol] {
			d.TradeID, d.TradeKind = oi.uuid, "holding"
		}
	}
}

/*
attachDayAnchors decorates position views with the two anchors the UI
needs to split a day into overnight (prior close to today's open) and
session (open to now): per-unit opens from one batched live quote call
(equities and OCC option symbols share the endpoint), and prior-close
position values from the EOD snapshot ledger. Best-effort: a missing
quote or snapshot just leaves the field omitted and the UI shows the
unsplit day change.
*/
func (s *Server) dayAnchors(symbols []string, today string) (openPerUnit, prevCloseValue, todayCloseValue map[string]float64) {
	prevCloseValue, err := s.db.GetPrevCloseValues(today)
	if err != nil {
		prevCloseValue = map[string]float64{}
	}
	todayCloseValue, err = s.db.GetCloseValues(today)
	if err != nil {
		todayCloseValue = map[string]float64{}
	}
	openPerUnit = map[string]float64{}
	if s.schwab != nil && len(symbols) > 0 {
		if quotes, qerr := s.schwab.GetQuotes(symbols); qerr == nil {
			for sym, q := range quotes {
				if q.OpenPrice > 0 {
					openPerUnit[sym] = q.OpenPrice
				}
			}
		}
	}
	return openPerUnit, prevCloseValue, todayCloseValue
}

func (s *Server) attachDayAnchors(views []portfolioPositionView, today string) {
	if len(views) == 0 {
		return
	}
	symbols := make([]string, 0, len(views))
	for _, v := range views {
		symbols = append(symbols, v.Symbol)
	}
	open, prevClose, todayClose := s.dayAnchors(symbols, today)
	for i := range views {
		v := &views[i]
		mult := 1.0
		if v.AssetType == "OPTION" {
			mult = 100
		}
		if o, ok := open[v.Symbol]; ok {
			v.OpenValue = o * v.Quantity * mult
		}
		if pc, ok := prevClose[v.Symbol]; ok && pc > 0 {
			v.PrevCloseValue = pc
		}
		if tc, ok := todayClose[v.Symbol]; ok && tc > 0 {
			v.TodayCloseValue = tc
		}
	}
}

func decisionView(d store.PortfolioDecisionRow) portfolioDecisionView {
	return portfolioDecisionView{
		Action:       d.Action,
		AssetType:    d.AssetType,
		Symbol:       d.Symbol,
		Underlying:   d.Underlying,
		ContractType: d.ContractType,
		Strike:       d.Strike,
		Expiration:   d.Expiration,
		Quantity:     d.Quantity,
		LimitPrice:   d.LimitPrice,
		Notional:     d.Notional,
		OrderID:      d.SchwabOrderID,
		Status:       d.Status,
		Rationale:    d.Rationale,
	}
}

func easternLoc() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.UTC
	}
	return loc
}
