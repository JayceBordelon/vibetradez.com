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

func TestCheckBuy_AllowsHealthyEquityBuy(t *testing.T) {
	c := DefaultCaps()
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 1_500}
	if err := c.CheckBuy(baseSnapshot(), m); err != nil {
		t.Fatalf("healthy buy should pass, got %s: %s", err.Code, err.Message)
	}
}

func TestCheckBuy_SettledCashRule(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	// Only $500 settled, the rest unsettled. A $600 buy must fail on
	// settled cash even though equity would allow it.
	s.SettledCash = 500
	s.UnsettledCash = 5_500
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 600}
	if err := c.CheckBuy(s, m); err == nil || err.Code != "settled_cash" {
		t.Fatalf("expected settled_cash violation, got %v", err)
	}
}

func TestCheckBuy_OptionsSleeveCap(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	// Sleeve cap is 50% of $6,000 = $3,000. Already $2,500 of options
	// across other names; a $600 option buy (-> $3,100) must trip.
	s.Positions = []Position{{Symbol: "AAPL_OPT", Underlying: "AAPL", AssetType: AssetOption, MarkValue: 2_500, Quantity: 5}}
	m := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "NVDA_OPT", Underlying: "NVDA", Notional: 600}
	if err := c.CheckBuy(s, m); err == nil || err.Code != "options_sleeve" {
		t.Fatalf("expected options_sleeve violation, got %v", err)
	}
	// An equity buy of the same size is gated by ITS sleeve, which has room.
	mEq := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "NVDA", Underlying: "NVDA", Notional: 600}
	if err := c.CheckBuy(s, mEq); err != nil {
		t.Fatalf("equity buy should not hit the options sleeve cap, got %s", err.Code)
	}
}

func TestCheckBuy_EquitySleeveCap(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	// Equity sleeve cap is 50% of $6,000 = $3,000. Already $2,800 of stock;
	// a $300 equity buy (-> $3,100) must trip, an option buy must not.
	s.Positions = []Position{{Symbol: "MDT", Underlying: "MDT", AssetType: AssetEquity, MarkValue: 2_800, Quantity: 50}}
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "AAPL", Underlying: "AAPL", Notional: 300}
	if err := c.CheckBuy(s, m); err == nil || err.Code != "equity_sleeve" {
		t.Fatalf("expected equity_sleeve violation, got %v", err)
	}
	mOpt := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "AAPL_OPT", Underlying: "AAPL", Notional: 300}
	if err := c.CheckBuy(s, mOpt); err != nil {
		t.Fatalf("option buy should not hit the equity sleeve cap, got %s", err.Code)
	}
}

func TestCheckBuy_SleevesAreTheWholePolicy(t *testing.T) {
	// Everything the old cap sheet refused is now allowed: full-conviction
	// single-name buys at any order size, with no liquidity floor, no
	// drawdown halt, and no session pacing. A single $3,000 order (50% of
	// equity, one name, deep drawdown vs high-water) passes.
	c := DefaultCaps()
	s := baseSnapshot()
	s.HighWaterMark = 12_000 // down 50% from the peak: no breaker anymore
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 3_000}
	if err := c.CheckBuy(s, m); err != nil {
		t.Fatalf("sleeve-sized single-name buy must pass under the 50/50 policy, got %s: %s", err.Code, err.Message)
	}
	// One dollar past the sleeve still trips.
	m.Notional = 3_001
	if err := c.CheckBuy(s, m); err == nil || err.Code != "equity_sleeve" {
		t.Fatalf("expected equity_sleeve just past the cap, got %v", err)
	}
}

func TestCheckSell(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	s.Positions = []Position{{Symbol: "MDT", Underlying: "MDT", AssetType: AssetEquity, Quantity: 30, MarkValue: 1_500}}

	// Valid partial sell.
	if err := c.CheckSell(s, Move{Action: ActionSellEquity, Symbol: "MDT", Quantity: 10}); err != nil {
		t.Fatalf("valid sell should pass, got %s", err.Code)
	}
	// Oversell.
	if err := c.CheckSell(s, Move{Action: ActionSellEquity, Symbol: "MDT", Quantity: 31}); err == nil || err.Code != "oversell" {
		t.Fatalf("expected oversell, got %v", err)
	}
	// No position.
	if err := c.CheckSell(s, Move{Action: ActionSellEquity, Symbol: "AAPL", Quantity: 1}); err == nil || err.Code != "no_position" {
		t.Fatalf("expected no_position, got %v", err)
	}
}
