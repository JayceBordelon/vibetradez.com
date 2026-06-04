package store

import "testing"

func ptrF(f float64) *float64 { return &f }

func cleanPortfolio(s *Store) {
	_, _ = s.db.Exec("DELETE FROM portfolio_decisions")
	_, _ = s.db.Exec("DELETE FROM portfolio_sessions")
	_, _ = s.db.Exec("DELETE FROM portfolio_equity_curve")
}

func TestPortfolioDecisions_RoundTrip(t *testing.T) {
	s := setupTestDB(t)
	cleanPortfolio(s)

	// An option buy (nullable strike/fill set), an equity buy (strike null),
	// and a hold (all order fields null/zero).
	optBuy := PortfolioDecisionRow{
		Date: "2026-06-03", Source: "agent", Action: "buy_option", AssetType: "OPTION",
		Symbol: "MDT   260417C00079000", Underlying: "MDT", ContractType: "CALL",
		Strike: ptrF(79), Expiration: "2026-04-17", Quantity: 2, LimitPrice: 1.20, Notional: 240,
		Mode: "live", SchwabOrderID: "live-1", Status: "FILLED", FillPrice: ptrF(1.18),
		ExecutionID: 11, Rationale: "leveraged upside into the catalyst",
	}
	eqBuy := PortfolioDecisionRow{
		Date: "2026-06-03", Source: "agent", Action: "buy_equity", AssetType: "EQUITY",
		Symbol: "AAPL", Underlying: "AAPL", Quantity: 5, LimitPrice: 200, Notional: 1000,
		Mode: "live", SchwabOrderID: "live-2", Status: "FILLED", FillPrice: ptrF(199.5),
		ExecutionID: 12, Rationale: "core hold",
	}
	hold := PortfolioDecisionRow{
		Date: "2026-06-03", Source: "agent", Action: "hold", Rationale: "nothing else clears the bar",
	}
	for _, d := range []PortfolioDecisionRow{optBuy, eqBuy, hold} {
		if _, err := s.SavePortfolioDecision(d); err != nil {
			t.Fatalf("save %s: %v", d.Action, err)
		}
	}

	got, err := s.GetPortfolioDecisions("2026-06-03")
	if err != nil {
		t.Fatalf("get decisions: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 decisions, got %d", len(got))
	}
	// Commit order preserved.
	if got[0].Action != "buy_option" || got[2].Action != "hold" {
		t.Fatalf("order not preserved: %s ... %s", got[0].Action, got[2].Action)
	}
	// Option strike + fill survived as non-nil.
	if got[0].Strike == nil || *got[0].Strike != 79 || got[0].FillPrice == nil {
		t.Fatalf("option strike/fill not round-tripped: %+v", got[0])
	}
	// Equity buy has null strike.
	if got[1].Strike != nil {
		t.Fatalf("equity strike should be nil, got %v", *got[1].Strike)
	}
	// Hold has null strike/fill and empty order id.
	if got[2].Strike != nil || got[2].FillPrice != nil || got[2].SchwabOrderID != "" {
		t.Fatalf("hold should have null order fields: %+v", got[2])
	}

	// Other dates are isolated.
	none, err := s.GetPortfolioDecisions("2026-06-02")
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no decisions on a different date, got %d", len(none))
	}
}

func TestPortfolioSession_Upsert(t *testing.T) {
	s := setupTestDB(t)
	cleanPortfolio(s)

	if _, _, _, _, ok, err := s.GetPortfolioSession("2026-06-03"); err != nil || ok {
		t.Fatalf("expected no session, got ok=%v err=%v", ok, err)
	}
	if err := s.SavePortfolioSession("2026-06-03", "live", "defensive, mostly cash", "Quiet day, raised a little cash.", "Confirm the cash settles."); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePortfolioSession("2026-06-03", "live", "rotated into MDT calls, leaning long", "Rotated into MDT calls and added to the core.", "Watch MDT into earnings; add on a dip."); err != nil {
		t.Fatal(err)
	}
	mode, stance, summary, actionItems, ok, err := s.GetPortfolioSession("2026-06-03")
	if err != nil || !ok {
		t.Fatalf("expected session, ok=%v err=%v", ok, err)
	}
	if mode != "live" || stance != "rotated into MDT calls, leaning long" {
		t.Fatalf("upsert did not replace stance: mode=%s stance=%q", mode, stance)
	}
	if summary != "Rotated into MDT calls and added to the core." {
		t.Fatalf("upsert did not replace summary: %q", summary)
	}
	if actionItems != "Watch MDT into earnings; add on a dip." {
		t.Fatalf("upsert did not replace action items: %q", actionItems)
	}

	// LatestPortfolioSession returns the most recent synopsis + action items.
	syn, act, ok2, err := s.LatestPortfolioSession()
	if err != nil || !ok2 || syn != "Rotated into MDT calls and added to the core." || act != "Watch MDT into earnings; add on a dip." {
		t.Fatalf("latest session wrong: syn=%q act=%q ok=%v err=%v", syn, act, ok2, err)
	}
}

func TestEquityCurveAndHighWaterMark(t *testing.T) {
	s := setupTestDB(t)
	cleanPortfolio(s)

	if _, ok, err := s.GetHighWaterMark(); err != nil || ok {
		t.Fatalf("empty curve should report ok=false, got ok=%v err=%v", ok, err)
	}

	points := []EquityCurvePoint{
		{Date: "2026-06-01", AccountEquity: 6000, SettledCash: 6000, HighWaterMark: 6000, SPYClose: 590},
		{Date: "2026-06-02", AccountEquity: 6800, SettledCash: 2000, HighWaterMark: 6800, SPYClose: 595},
		{Date: "2026-06-03", AccountEquity: 6300, SettledCash: 1500, HighWaterMark: 6800, SPYClose: 592},
	}
	for _, p := range points {
		if err := s.SaveEquityCurvePoint(p); err != nil {
			t.Fatalf("save %s: %v", p.Date, err)
		}
	}
	// Upsert: rewrite 2026-06-03 with higher equity.
	if err := s.SaveEquityCurvePoint(EquityCurvePoint{Date: "2026-06-03", AccountEquity: 6400, SettledCash: 1400, HighWaterMark: 6800, SPYClose: 593}); err != nil {
		t.Fatal(err)
	}

	curve, err := s.GetEquityCurve("2026-06-01", "2026-06-03")
	if err != nil {
		t.Fatal(err)
	}
	if len(curve) != 3 {
		t.Fatalf("expected 3 curve points, got %d", len(curve))
	}
	if curve[2].AccountEquity != 6400 {
		t.Fatalf("upsert did not replace 06-03 equity, got %f", curve[2].AccountEquity)
	}

	hwm, ok, err := s.GetHighWaterMark()
	if err != nil || !ok {
		t.Fatalf("expected a high-water mark, ok=%v err=%v", ok, err)
	}
	if hwm != 6800 {
		t.Fatalf("expected high-water 6800, got %f", hwm)
	}
}
