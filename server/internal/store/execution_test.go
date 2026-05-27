package store

import (
	"database/sql"
	"testing"

	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/trades"
)

/*
TestDeriveExecutionState_CloseFailedStrandedAtBroker guards the
audit fix: a position whose open filled but whose close attempt
ended in failed / rejected / canceled is no longer collapsed into
"holding" — the dashboard now surfaces "close_failed" so the
operator knows the position is stranded at the broker after the
3:55 retry-cancel-replace window exhausted.
*/
func TestDeriveExecutionState_CloseFailedStrandedAtBroker(t *testing.T) {
	cases := []struct {
		openStatus  string
		closeStatus sql.NullString
		want        string
	}{
		// Open + no close yet → holding.
		{"filled", sql.NullString{Valid: false}, "holding"},
		// Open + close filled → closed.
		{"filled", sql.NullString{Valid: true, String: "filled"}, "closed"},
		// Open + close in-flight → still holding.
		{"filled", sql.NullString{Valid: true, String: "pending"}, "holding"},
		{"filled", sql.NullString{Valid: true, String: "working"}, "holding"},
		// THE FIX: close attempt reached a non-success terminal.
		{"filled", sql.NullString{Valid: true, String: "failed"}, "close_failed"},
		{"filled", sql.NullString{Valid: true, String: "rejected"}, "close_failed"},
		{"filled", sql.NullString{Valid: true, String: "canceled"}, "close_failed"},
		// Open never filled.
		{"working", sql.NullString{}, "submitted"},
		{"pending", sql.NullString{}, "submitted"},
		{"failed", sql.NullString{}, "failed"},
		{"rejected", sql.NullString{}, "failed"},
		// Unknown open status → empty (don't surface).
		{"", sql.NullString{}, ""},
		{"garbage", sql.NullString{}, ""},
	}

	for _, tc := range cases {
		got := deriveExecutionState(tc.openStatus, tc.closeStatus)
		if got != tc.want {
			t.Errorf("deriveExecutionState(%q, %+v) = %q, want %q", tc.openStatus, tc.closeStatus, got, tc.want)
		}
	}
}

/*
TestBasketSummaryForDate_SkippedTradeWithNullStrike guards the
2026-05-26 incident: the 9:35 ET cancel-dangling + summary cron
crashed with "scan basket summary row: sql: Scan error on column
index 3, name "strike_price": converting NULL to float64 is
unsupported" because skipped candidates leave trades.strike_price
NULL (the agent never resolved a contract for them), but the
summary query was scanning straight into float64. Every basket
where at least one candidate gets skipped (i.e. most days) would
silently drop the summary email.
*/
func TestBasketSummaryForDate_SkippedTradeWithNullStrike(t *testing.T) {
	s := setupTestDB(t)

	const date = "2099-01-15"
	// Two trades: one resolved (rank 2, OKLO-shaped) and one skipped
	// (rank 1, MU-shaped with nil strike) — mirrors the production
	// shape that triggered the crash.
	strike := 75.0
	exp := "2099-01-18"
	dte := 3
	est := 0.67
	stop := 0.50
	tradeList := []trades.Trade{
		{Symbol: "MU", ContractType: "CALL", Rank: 1, Thesis: "rank 1 skipped"},
		{Symbol: "OKLO", ContractType: "CALL", Rank: 2, Thesis: "rank 2 resolved",
			StrikePrice: &strike, Expiration: &exp, DTE: &dte, EstimatedPrice: &est, StopLoss: &stop},
	}
	if err := s.SaveMorningTrades(date, tradeList); err != nil {
		t.Fatalf("SaveMorningTrades: %v", err)
	}

	for _, tr := range tradeList {
		status := "filled"
		if tr.Symbol == "MU" {
			status = "skipped"
		}
		if _, err := s.InsertExecution(exec.Execution{
			TradeID: tr.ID, Mode: "live", Side: "open", Status: status,
			RequestedQuantity: 1, ErrorMessage: "test",
		}); err != nil {
			t.Fatalf("InsertExecution(%s): %v", tr.Symbol, err)
		}
	}

	rows, err := s.BasketSummaryForDate(date)
	if err != nil {
		t.Fatalf("BasketSummaryForDate returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	// Rows ordered by rank ASC, so rank-1 MU (skipped, null strike) is first.
	if rows[0].Symbol != "MU" || rows[0].Status != "skipped" || rows[0].StrikePrice != 0 {
		t.Errorf("rank-1 skipped row: got %+v, want MU/skipped/strike=0", rows[0])
	}
	if rows[1].Symbol != "OKLO" || rows[1].StrikePrice != 75 {
		t.Errorf("rank-2 resolved row: got %+v, want OKLO/strike=75", rows[1])
	}
}

/*
TestGetExecutionsForDate_SkippedTradeWithNullStrike guards the
dashboard-side counterpart of the BasketSummary fix: the live
dashboard query joined trades and scanned t.strike_price into a
float64, so any day with a skipped candidate (NULL strike) silently
returned an error and the dashboard saw zero executions — the
"finding contracts..." placeholder kept rendering for the skipped
pick instead of the agent's skip reason.
*/
func TestGetExecutionsForDate_SkippedTradeWithNullStrike(t *testing.T) {
	s := setupTestDB(t)

	const date = "2099-01-16"
	strike := 86.0
	exp := "2099-01-18"
	dte := 2
	est := 1.25
	stop := 0.60
	tradeList := []trades.Trade{
		{Symbol: "MU", ContractType: "CALL", Rank: 3, Thesis: "skipped at open"},
		{Symbol: "PDD", ContractType: "PUT", Rank: 2, Thesis: "filled",
			StrikePrice: &strike, Expiration: &exp, DTE: &dte, EstimatedPrice: &est, StopLoss: &stop},
	}
	if err := s.SaveMorningTrades(date, tradeList); err != nil {
		t.Fatalf("SaveMorningTrades: %v", err)
	}

	for _, tr := range tradeList {
		status := "filled"
		if tr.Symbol == "MU" {
			status = "skipped"
		}
		if _, err := s.InsertExecution(exec.Execution{
			TradeID: tr.ID, Mode: "live", Side: "open", Status: status,
			RequestedQuantity: 1, ErrorMessage: "agent declined",
		}); err != nil {
			t.Fatalf("InsertExecution(%s): %v", tr.Symbol, err)
		}
	}

	rows, err := s.GetExecutionsForDate(date)
	if err != nil {
		t.Fatalf("GetExecutionsForDate: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	var skipped, filled *ExecutionView
	for _, r := range rows {
		switch r.Symbol {
		case "MU":
			skipped = r
		case "PDD":
			filled = r
		}
	}
	if skipped == nil || skipped.State != "skipped" || skipped.StrikePrice != 0 {
		t.Errorf("skipped MU row: got %+v, want state=skipped strike=0", skipped)
	}
	if filled == nil || filled.StrikePrice != 86 {
		t.Errorf("filled PDD row: got %+v, want strike=86", filled)
	}
}

func TestGetExecutionsForDateRange_SkippedTradeWithNullStrike(t *testing.T) {
	s := setupTestDB(t)

	const date = "2099-01-17"
	tradeList := []trades.Trade{
		{Symbol: "MU", ContractType: "CALL", Rank: 1, Thesis: "skipped"},
	}
	if err := s.SaveMorningTrades(date, tradeList); err != nil {
		t.Fatalf("SaveMorningTrades: %v", err)
	}
	if _, err := s.InsertExecution(exec.Execution{
		TradeID: tradeList[0].ID, Mode: "live", Side: "open", Status: "skipped",
		RequestedQuantity: 0, ErrorMessage: "agent declined",
	}); err != nil {
		t.Fatalf("InsertExecution: %v", err)
	}

	byDate, err := s.GetExecutionsForDateRange(date, date)
	if err != nil {
		t.Fatalf("GetExecutionsForDateRange: %v", err)
	}
	if len(byDate[date]) != 1 || byDate[date][0].State != "skipped" {
		t.Fatalf("expected 1 skipped row for %s, got %+v", date, byDate[date])
	}
}
