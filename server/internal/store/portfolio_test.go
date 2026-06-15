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

func TestOrderStatusReconciliation(t *testing.T) {
	s := setupTestDB(t)
	cleanPortfolio(s)

	working := PortfolioDecisionRow{
		Date: "2026-06-09", Source: "agent", Action: "buy_equity", AssetType: "EQUITY",
		Symbol: "GOOGL", Underlying: "GOOGL", Quantity: 5, LimitPrice: 371, Notional: 1855,
		Mode: "live", SchwabOrderID: "ord-1", Status: "WORKING", Rationale: "core",
	}
	submitted := PortfolioDecisionRow{
		Date: "2026-06-09", Source: "agent", Action: "buy_option", AssetType: "OPTION",
		Symbol: "NVDA  260918C00215000", Underlying: "NVDA", Quantity: 1, LimitPrice: 18.30, Notional: 1830,
		Mode: "live", SchwabOrderID: "ord-2", Status: "submitted", Rationale: "convexity",
	}
	alreadyFilled := PortfolioDecisionRow{
		Date: "2026-06-09", Source: "agent", Action: "sell_equity", AssetType: "EQUITY",
		Symbol: "AVGO", Underlying: "AVGO", Quantity: 4, LimitPrice: 186.46, Notional: 745.84,
		Mode: "live", SchwabOrderID: "ord-3", Status: "FILLED", FillPrice: ptrF(186.40), Rationale: "trim",
	}
	noOrder := PortfolioDecisionRow{
		Date: "2026-06-09", Source: "agent", Action: "hold", Rationale: "as-is",
	}
	for _, d := range []PortfolioDecisionRow{working, submitted, alreadyFilled, noOrder} {
		if _, err := s.SavePortfolioDecision(d); err != nil {
			t.Fatalf("save %s: %v", d.Action, err)
		}
	}

	// Only the non-terminal, order-carrying decisions are reconciliation
	// candidates: FILLED is terminal and the hold has no order id.
	ids, err := s.OpenOrderIDs()
	if err != nil {
		t.Fatalf("open order ids: %v", err)
	}
	want := map[string]bool{"ord-1": true, "ord-2": true}
	if len(ids) != 2 || !want[ids[0]] || !want[ids[1]] {
		t.Fatalf("open order ids = %v, want ord-1 and ord-2", ids)
	}

	// A fill updates status + fill price; a status-only change keeps the
	// existing fill price untouched.
	if err := s.UpdateDecisionOrderStatus("ord-1", "FILLED", 370.85); err != nil {
		t.Fatalf("update ord-1: %v", err)
	}
	if err := s.UpdateDecisionOrderStatus("ord-2", "CANCELED", 0); err != nil {
		t.Fatalf("update ord-2: %v", err)
	}
	rows, err := s.GetPortfolioDecisions("2026-06-09")
	if err != nil {
		t.Fatalf("get decisions: %v", err)
	}
	byOrder := map[string]PortfolioDecisionRow{}
	for _, r := range rows {
		byOrder[r.SchwabOrderID] = r
	}
	if g := byOrder["ord-1"]; g.Status != "FILLED" || g.FillPrice == nil || *g.FillPrice != 370.85 {
		t.Errorf("ord-1 after fill: status=%q fill=%v, want FILLED 370.85", g.Status, g.FillPrice)
	}
	if c := byOrder["ord-2"]; c.Status != "CANCELED" || c.FillPrice != nil {
		t.Errorf("ord-2 after cancel: status=%q fill=%v, want CANCELED with no fill", c.Status, c.FillPrice)
	}
	if ids, _ = s.OpenOrderIDs(); len(ids) != 0 {
		t.Errorf("open order ids after reconciliation = %v, want empty", ids)
	}
}

func TestGetPrevCloseValues(t *testing.T) {
	s := setupTestDB(t)
	cleanPortfolio(s)

	rows := []PortfolioPositionRow{
		{Date: "2026-06-05", Symbol: "GOOGL", Underlying: "GOOGL", AssetType: "EQUITY", Quantity: 5, MarketValue: 1800, CostBasis: 1750},
		{Date: "2026-06-08", Symbol: "GOOGL", Underlying: "GOOGL", AssetType: "EQUITY", Quantity: 5, MarketValue: 1851.7, CostBasis: 1750},
		{Date: "2026-06-08", Symbol: "NVDA  260918C00215000", Underlying: "NVDA", AssetType: "OPTION", ContractType: "CALL", Quantity: 1, MarketValue: 1804, CostBasis: 1830},
		{Date: "2026-06-09", Symbol: "GOOGL", Underlying: "GOOGL", AssetType: "EQUITY", Quantity: 5, MarketValue: 1817.7, CostBasis: 1750},
	}
	byDate := map[string][]PortfolioPositionRow{}
	for _, r := range rows {
		byDate[r.Date] = append(byDate[r.Date], r)
	}
	for date, batch := range byDate {
		if err := s.SavePositionsSnapshot(date, batch); err != nil {
			t.Fatalf("save snapshot %s: %v", date, err)
		}
	}

	// "Yesterday" relative to 2026-06-09 is the 06-08 snapshot: both
	// symbols held overnight, at their 06-08 closing marks. The 06-09 row
	// must not leak in, and the older 06-05 row must lose to 06-08.
	got, err := s.GetPrevCloseValues("2026-06-09")
	if err != nil {
		t.Fatalf("prev close values: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 symbols, got %v", got)
	}
	if got["GOOGL"] != 1851.7 {
		t.Errorf("GOOGL prev close value: want 1851.7, got %v", got["GOOGL"])
	}
	if got["NVDA  260918C00215000"] != 1804 {
		t.Errorf("option prev close value: want 1804, got %v", got["NVDA  260918C00215000"])
	}
}
