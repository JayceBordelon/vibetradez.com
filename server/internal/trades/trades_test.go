package trades

import (
	"math"
	"testing"

	"vibetradez.com/internal/store"
)

func fp(v float64) *float64 { return &v }

func almost(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("%s: got %v, want %v", label, got, want)
	}
}

// Derive must prefer the recorded fill price over the submitted limit, so
// realized P&L reflects what the broker actually did.
func TestDerive_PrefersFillPriceOverLimit(t *testing.T) {
	rows := []store.PortfolioDecisionRow{
		{Action: "buy_equity", AssetType: "EQUITY", Symbol: "NVDA", Quantity: 10, LimitPrice: 100, FillPrice: fp(99), Date: "2026-06-01"},
		{Action: "sell_equity", AssetType: "EQUITY", Symbol: "NVDA", Quantity: 10, LimitPrice: 110, FillPrice: fp(111), Date: "2026-06-05"},
	}
	trips := Derive(rows)
	if len(trips) != 1 {
		t.Fatalf("expected 1 round trip, got %d", len(trips))
	}
	almost(t, trips[0].RealizedPnl, (111-99)*10, "realized pnl")
	almost(t, trips[0].EntryPrice, 99, "entry price")
	almost(t, trips[0].ExitPrice, 111, "exit price")
	if trips[0].HoldDays != 4 {
		t.Fatalf("hold days: got %d, want 4", trips[0].HoldDays)
	}
}

func TestSummarize_WinLossSplit(t *testing.T) {
	trips := []RoundTrip{
		{RealizedPnl: 100, HoldDays: 2},
		{RealizedPnl: 50, HoldDays: 4},
		{RealizedPnl: -200, HoldDays: 6},
		{RealizedPnl: 0, HoldDays: 0}, // flat trade counts as a loss (paid the spread)
	}
	s := Summarize(trips)
	if s.Trades != 4 || s.Wins != 2 || s.Losses != 2 {
		t.Fatalf("counts: got %d/%d/%d, want 4/2/2", s.Trades, s.Wins, s.Losses)
	}
	almost(t, s.WinRatePct, 50, "win rate")
	almost(t, s.TotalPnl, -50, "total pnl")
	almost(t, s.AvgWinPnl, 75, "avg win")
	almost(t, s.AvgLossPnl, -100, "avg loss")
	almost(t, s.AvgHoldDays, 3, "avg hold days")
	almost(t, s.BiggestWin, 100, "biggest win")
	almost(t, s.BiggestLoss, -200, "biggest loss")
}

func TestSummarize_Empty(t *testing.T) {
	s := Summarize(nil)
	if s.Trades != 0 || s.WinRatePct != 0 {
		t.Fatalf("empty summary should be zeroed, got %+v", s)
	}
}

// The benchmark race must skip SPY gaps (0 close on quote failure) and
// compute drawdown peak-to-trough on account equity.
func TestSummarizeCurve_RaceAndDrawdown(t *testing.T) {
	points := []store.EquityCurvePoint{
		{Date: "2026-06-01", AccountEquity: 1000, SPYClose: 0}, // SPY gap day
		{Date: "2026-06-02", AccountEquity: 1200, SPYClose: 500},
		{Date: "2026-06-03", AccountEquity: 900, SPYClose: 0}, // SPY gap day
		{Date: "2026-06-04", AccountEquity: 1100, SPYClose: 550},
	}
	c := SummarizeCurve(points)
	if c.Days != 4 || c.FirstDate != "2026-06-01" || c.LastDate != "2026-06-04" {
		t.Fatalf("window: got %d days %s..%s", c.Days, c.FirstDate, c.LastDate)
	}
	almost(t, c.AccountReturnPct, 10, "account return")
	almost(t, c.SpyReturnPct, 10, "spy return") // 500 -> 550 across the gaps
	almost(t, c.VsSpyPct, 0, "vs spy")
	almost(t, c.MaxDrawdownPct, 25, "max drawdown") // 1200 -> 900
	almost(t, c.CurrentEquity, 1100, "current equity")
}

func TestSummarizeCurve_Empty(t *testing.T) {
	c := SummarizeCurve(nil)
	if c.Days != 0 || c.AccountReturnPct != 0 {
		t.Fatalf("empty curve should be zeroed, got %+v", c)
	}
}
