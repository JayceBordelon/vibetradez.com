package server

import (
	"context"
	"net/http"
	"time"

	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/portfolio"
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
}

type portfolioResponse struct {
	// Enabled is false when the account/broker isn't wired (trading
	// disabled at startup). The dashboard shows a "not running" state.
	Enabled        bool                    `json:"enabled"`
	Mode           string                  `json:"mode"` // always "live"
	Date           string                  `json:"date"`
	Equity         float64                 `json:"equity"`
	SettledCash    float64                 `json:"settled_cash"`
	UnsettledCash  float64                 `json:"unsettled_cash"`
	HighWaterMark  float64                 `json:"high_water_mark"`
	SPYClose       float64                 `json:"spy_close"`
	DrawdownHalted bool                    `json:"drawdown_halted"`
	Positions      []portfolioPositionView `json:"positions"`
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
}

type equityCurveResponse struct {
	Points []equityCurvePointView `json:"points"`
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
	caps := portfolio.DefaultCaps()
	resp.DrawdownHalted = caps.DrawdownHalted(portfolio.Snapshot{Equity: resp.Equity, HighWaterMark: resp.HighWaterMark})

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
			resp.Points = append(resp.Points, equityCurvePointView(p))
		}
	}
	writeJSON(w, http.StatusOK, resp)
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
