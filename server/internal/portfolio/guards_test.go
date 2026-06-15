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
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 1_500}
	if err := CheckBuy(baseSnapshot(), m); err != nil {
		t.Fatalf("healthy buy should pass, got %s: %s", err.Code, err.Message)
	}
}

func TestCheckBuy_SettledCashRule(t *testing.T) {
	s := baseSnapshot()
	// Only $500 settled, the rest unsettled. A $600 buy must fail on settled
	// cash: a cash account cannot redeploy unsettled proceeds until T+1.
	s.SettledCash = 500
	s.UnsettledCash = 5_500
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 600}
	if err := CheckBuy(s, m); err == nil || err.Code != "settled_cash" {
		t.Fatalf("expected settled_cash violation, got %v", err)
	}
}

func TestCheckBuy_FullDiscretionOverAllocation(t *testing.T) {
	// The model has full discretion: no stock/option split, no per-name cap,
	// no drawdown breaker. The only buy-side gate is settled cash, so any
	// allocation that fits the settled cash on hand is allowed.
	s := baseSnapshot()
	s.HighWaterMark = 12_000 // down 50% from the peak: no breaker anymore

	// The whole account into a single equity name passes.
	allInEquity := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 6_000}
	if err := CheckBuy(s, allInEquity); err != nil {
		t.Fatalf("all-in single-name equity buy must pass, got %s: %s", err.Code, err.Message)
	}

	// The whole account into options (the old allocation caps blocked this)
	// passes now.
	allInOptions := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "NVDA_OPT", Underlying: "NVDA", Notional: 6_000}
	if err := CheckBuy(s, allInOptions); err != nil {
		t.Fatalf("all-in options buy must pass, got %s: %s", err.Code, err.Message)
	}

	// Adding to an already-concentrated single name is fine: no per-name cap.
	s.Positions = []Position{{Symbol: "MDT", Underlying: "MDT", AssetType: AssetEquity, MarkValue: 5_000, Quantity: 50}}
	addMore := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 1_000}
	if err := CheckBuy(s, addMore); err != nil {
		t.Fatalf("adding to a concentrated single name must pass, got %s: %s", err.Code, err.Message)
	}

	// The only line is settled cash: one dollar past it trips.
	overCash := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 6_001}
	if err := CheckBuy(s, overCash); err == nil || err.Code != "settled_cash" {
		t.Fatalf("expected settled_cash just past the cash on hand, got %v", err)
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
