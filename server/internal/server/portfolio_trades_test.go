package server

import (
	"math"
	"testing"

	"vibetradez.com/internal/store"
)

func f(v float64) *float64 { return &v }

func TestDeriveClosedTrades_EquityRoundTrip(t *testing.T) {
	decisions := []store.PortfolioDecisionRow{
		{ID: 1, Date: "2026-05-01", Action: "buy_equity", AssetType: "EQUITY", Symbol: "MDT", Underlying: "MDT", Quantity: 10, LimitPrice: 80, Rationale: "entry"},
		{ID: 2, Date: "2026-05-10", Action: "sell_equity", AssetType: "EQUITY", Symbol: "MDT", Underlying: "MDT", Quantity: 10, LimitPrice: 88, Rationale: "exit"},
	}
	out := deriveClosedTrades(decisions)
	if len(out) != 1 {
		t.Fatalf("expected 1 closed trade, got %d", len(out))
	}
	tr := out[0]
	if tr.Kind != "stock" || tr.Symbol != "MDT" {
		t.Fatalf("unexpected identity: %+v", tr)
	}
	if !approx(tr.EntryPrice, 80) || !approx(tr.ExitPrice, 88) {
		t.Fatalf("entry/exit wrong: %+v", tr)
	}
	if !approx(tr.RealizedPnl, 80) { // (88-80)*10
		t.Fatalf("realized pnl wrong: got %f want 80", tr.RealizedPnl)
	}
	if !approx(tr.RealizedPct, 10) { // 80/800
		t.Fatalf("realized pct wrong: got %f want 10", tr.RealizedPct)
	}
	if tr.HoldDays != 9 {
		t.Fatalf("hold days wrong: got %d want 9", tr.HoldDays)
	}
	if tr.ID != "MDT-2026-05-10" {
		t.Fatalf("id wrong: %q", tr.ID)
	}
}

func TestDeriveClosedTrades_OptionMultiplier(t *testing.T) {
	occ := "MDT   260417C00079000"
	decisions := []store.PortfolioDecisionRow{
		{ID: 1, Date: "2026-03-01", Action: "buy_option", AssetType: "OPTION", Symbol: occ, Underlying: "MDT", ContractType: "CALL", Strike: f(79), Expiration: "2026-04-17", Quantity: 2, LimitPrice: 1.50},
		{ID: 2, Date: "2026-03-08", Action: "sell_option", AssetType: "OPTION", Symbol: occ, Underlying: "MDT", ContractType: "CALL", Strike: f(79), Expiration: "2026-04-17", Quantity: 2, LimitPrice: 2.50},
	}
	out := deriveClosedTrades(decisions)
	if len(out) != 1 {
		t.Fatalf("expected 1 closed trade, got %d", len(out))
	}
	tr := out[0]
	if tr.Kind != "option" {
		t.Fatalf("expected option kind, got %q", tr.Kind)
	}
	// 2 contracts * 100 * (2.50 - 1.50) = 200
	if !approx(tr.RealizedPnl, 200) {
		t.Fatalf("option pnl wrong: got %f want 200", tr.RealizedPnl)
	}
	if !approx(tr.EntryPrice, 1.50) || !approx(tr.ExitPrice, 2.50) {
		t.Fatalf("per-contract entry/exit wrong: %+v", tr)
	}
	if tr.ID != compactSymbol(occ)+"-2026-03-08" {
		t.Fatalf("option id should be space-stripped + close date: %q", tr.ID)
	}
}

func TestDeriveClosedTrades_PartialThenFlatAndOpenStaysOpen(t *testing.T) {
	decisions := []store.PortfolioDecisionRow{
		{ID: 1, Date: "2026-05-01", Action: "buy_equity", AssetType: "EQUITY", Symbol: "AMD", Underlying: "AMD", Quantity: 10, LimitPrice: 100},
		{ID: 2, Date: "2026-05-05", Action: "sell_equity", AssetType: "EQUITY", Symbol: "AMD", Underlying: "AMD", Quantity: 4, LimitPrice: 110},
		{ID: 3, Date: "2026-05-09", Action: "sell_equity", AssetType: "EQUITY", Symbol: "AMD", Underlying: "AMD", Quantity: 6, LimitPrice: 120},
		// A still-open buy that must NOT appear as closed.
		{ID: 4, Date: "2026-05-11", Action: "buy_equity", AssetType: "EQUITY", Symbol: "NVDA", Underlying: "NVDA", Quantity: 5, LimitPrice: 120},
	}
	out := deriveClosedTrades(decisions)
	if len(out) != 1 {
		t.Fatalf("expected 1 closed trade (AMD), got %d: %+v", len(out), out)
	}
	tr := out[0]
	if tr.Symbol != "AMD" || !approx(tr.Quantity, 10) {
		t.Fatalf("expected full 10-share AMD round trip, got %+v", tr)
	}
	// proceeds 4*110 + 6*120 = 1160; cost 10*100 = 1000; pnl 160
	if !approx(tr.RealizedPnl, 160) {
		t.Fatalf("partial-close pnl wrong: got %f want 160", tr.RealizedPnl)
	}
	if tr.ClosedDate != "2026-05-09" {
		t.Fatalf("close date should be the flattening sell: %q", tr.ClosedDate)
	}
}

func TestDeriveClosedTrades_SellWithoutOpenIgnored(t *testing.T) {
	decisions := []store.PortfolioDecisionRow{
		{ID: 1, Date: "2026-05-01", Action: "sell_equity", AssetType: "EQUITY", Symbol: "XOM", Underlying: "XOM", Quantity: 5, LimitPrice: 110},
	}
	if out := deriveClosedTrades(decisions); len(out) != 0 {
		t.Fatalf("a sell with no open lot must not form a closed trade, got %+v", out)
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestDecoratePnlSeries_CumulativeRealizedAndDailyUnrealized(t *testing.T) {
	points := []equityCurvePointView{
		{Date: "2026-06-01"},
		{Date: "2026-06-02"},
		{Date: "2026-06-04"}, // gap: 06-03 has no curve row
		{Date: "2026-06-05"},
	}
	unreal := map[string]float64{
		"2026-06-01": 12.5,
		"2026-06-04": -40,
		// 06-02 and 06-05 absent: flat book days stay zero
	}
	closed := []closedTradeView{
		{ClosedDate: "2026-05-20", RealizedPnl: 999}, // before window: excluded
		{ClosedDate: "2026-06-02", RealizedPnl: 100},
		{ClosedDate: "2026-06-03", RealizedPnl: -30}, // closed on a non-curve day: lands on 06-04
		{ClosedDate: "2026-06-04", RealizedPnl: 10},
	}
	got := decoratePnlSeries(points, unreal, closed)
	wantRealized := []float64{0, 100, 80, 80}
	wantUnreal := []float64{12.5, 0, -40, 0}
	for i := range got {
		if got[i].RealizedCum != wantRealized[i] {
			t.Errorf("point %d (%s) realized_cum: want %v, got %v", i, got[i].Date, wantRealized[i], got[i].RealizedCum)
		}
		if got[i].Unrealized != wantUnreal[i] {
			t.Errorf("point %d (%s) unrealized: want %v, got %v", i, got[i].Date, wantUnreal[i], got[i].Unrealized)
		}
	}
}

func TestDecoratePnlSeries_EmptyInputs(t *testing.T) {
	if got := decoratePnlSeries(nil, nil, nil); len(got) != 0 {
		t.Errorf("nil points must stay empty, got %d", len(got))
	}
	points := []equityCurvePointView{{Date: "2026-06-01"}}
	got := decoratePnlSeries(points, nil, nil)
	if got[0].RealizedCum != 0 || got[0].Unrealized != 0 {
		t.Errorf("no trades / no snapshots must decorate zeros: %+v", got[0])
	}
}
