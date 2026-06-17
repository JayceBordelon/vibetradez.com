package portfolio

import "testing"

// baseSnapshot is a healthy $6,000 all-cash account with no positions.
func baseSnapshot() Snapshot {
	return Snapshot{
		Equity:        6_000,
		SettledCash:   6_000,
		UnsettledCash: 0,
		HighWaterMark: 6_000,
		Positions:     nil,
	}
}

func TestCheckBuy_AllowsHealthyOptionBuy(t *testing.T) {
	m := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "NVDA  260116C00150000", Underlying: "NVDA", Notional: 1_500}
	if err := CheckBuy(baseSnapshot(), m); err != nil {
		t.Fatalf("healthy option buy should pass, got %s: %s", err.Code, err.Message)
	}
}

func TestCheckBuy_SettledCashRule(t *testing.T) {
	s := baseSnapshot()
	// Only $500 settled, the rest unsettled. A $600 buy must fail on settled
	// cash: a cash account cannot redeploy unsettled proceeds until T+1.
	s.SettledCash = 500
	s.UnsettledCash = 5_500
	m := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "NVDA  260116C00150000", Underlying: "NVDA", Notional: 600}
	if err := CheckBuy(s, m); err == nil || err.Code != "settled_cash" {
		t.Fatalf("expected settled_cash violation, got %v", err)
	}
}

// CheckOptionSize caps a single option position at MaxSingleOptionFrac of
// equity. baseSnapshot is $6,000 equity, so the per-position limit is $600.
func TestCheckOptionSize(t *testing.T) {
	s := baseSnapshot()
	over := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "NVDA  OPT", Underlying: "NVDA", Notional: 700}
	if err := CheckOptionSize(s, over); err == nil || err.Code != "position_too_large" {
		t.Fatalf("a $700 position should breach the $600 cap, got %v", err)
	}
	ok := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "NVDA  OPT", Underlying: "NVDA", Notional: 500}
	if err := CheckOptionSize(s, ok); err != nil {
		t.Fatalf("a $500 position should pass the $600 cap, got %s", err.Code)
	}
	// The resulting exposure counts: $400 already held + $300 new = $700 > cap.
	s.Positions = []Position{{Symbol: "NVDA  OPT", Underlying: "NVDA", AssetType: AssetOption, MarkValue: 400, Quantity: 1}}
	add := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "NVDA  OPT", Underlying: "NVDA", Notional: 300}
	if err := CheckOptionSize(s, add); err == nil || err.Code != "position_too_large" {
		t.Fatalf("expected position_too_large on the resulting exposure, got %v", err)
	}
	// Non-buy moves are skipped.
	if err := CheckOptionSize(s, Move{Action: ActionSellOption, AssetType: AssetOption, Symbol: "X", Notional: 9_999}); err != nil {
		t.Fatalf("a non-buy must be skipped, got %v", err)
	}
}

// CheckUnderlyingExposure caps total exposure per underlying at
// MaxPerUnderlyingFrac of equity. baseSnapshot is $6,000, so the limit is
// $1,500 across all positions keyed to one underlying.
func TestCheckUnderlyingExposure(t *testing.T) {
	s := baseSnapshot()
	s.Positions = []Position{
		{Symbol: "NVDA  C1", Underlying: "NVDA", AssetType: AssetOption, MarkValue: 800, Quantity: 1},
		{Symbol: "NVDA  C2", Underlying: "NVDA", AssetType: AssetOption, MarkValue: 400, Quantity: 1},
	}
	// $1,200 already on NVDA + $400 new = $1,600 > the $1,500 cap.
	over := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "NVDA  C3", Underlying: "NVDA", Notional: 400}
	if err := CheckUnderlyingExposure(s, over); err == nil || err.Code != "underlying_concentration" {
		t.Fatalf("expected underlying_concentration, got %v", err)
	}
	// $1,200 + $200 = $1,400 is under the cap.
	ok := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "NVDA  C4", Underlying: "NVDA", Notional: 200}
	if err := CheckUnderlyingExposure(s, ok); err != nil {
		t.Fatalf("$1,400 total on NVDA should pass the $1,500 cap, got %s", err.Code)
	}
	// A different underlying is unaffected by NVDA's exposure.
	other := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "AAPL  C1", Underlying: "AAPL", Notional: 1_400}
	if err := CheckUnderlyingExposure(s, other); err != nil {
		t.Fatalf("a fresh underlying should pass, got %s", err.Code)
	}
}

func TestCheckSell(t *testing.T) {
	s := baseSnapshot()
	s.Positions = []Position{{Symbol: "MDT", Underlying: "MDT", AssetType: AssetEquity, Quantity: 30, MarkValue: 1_500}}

	// Valid partial sell.
	if err := CheckSell(s, Move{Action: ActionSellEquity, Symbol: "MDT", Quantity: 10}); err != nil {
		t.Fatalf("valid sell should pass, got %s", err.Code)
	}
	// Oversell.
	if err := CheckSell(s, Move{Action: ActionSellEquity, Symbol: "MDT", Quantity: 31}); err == nil || err.Code != "oversell" {
		t.Fatalf("expected oversell, got %v", err)
	}
	// No position.
	if err := CheckSell(s, Move{Action: ActionSellEquity, Symbol: "AAPL", Quantity: 1}); err == nil || err.Code != "no_position" {
		t.Fatalf("expected no_position, got %v", err)
	}
}
