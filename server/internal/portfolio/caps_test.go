package portfolio

import "testing"

// goodLiquidity returns a LiquidityCtx that clears every floor in
// DefaultCaps, so individual tests can perturb a single field.
func goodLiquidity() LiquidityCtx {
	return LiquidityCtx{
		StockPrice:         50,
		UnderlyingMktCap:   50_000_000_000,
		OptionOpenInterest: 5_000,
		OptionVolume:       1_000,
		SpreadPct:          0.02,
	}
}

// baseSnapshot is a healthy $6,000 all-cash account with no positions. The
// deployment budget is pinned to a fixed $3,000 (not derived from
// DefaultDailyDeploymentPct) so the cap-mechanism tests below stay stable
// when the policy default changes.
func baseSnapshot() Snapshot {
	return Snapshot{
		Equity:              6_000,
		SettledCash:         6_000,
		UnsettledCash:       0,
		HighWaterMark:       6_000,
		DeploymentBudget:    3_000,
		DeployedThisSession: 0,
		Positions:           nil,
	}
}

func TestCheckBuy_AllowsHealthyEquityBuy(t *testing.T) {
	c := DefaultCaps()
	m := Move{
		Action:     ActionBuyEquity,
		AssetType:  AssetEquity,
		Symbol:     "MDT",
		Underlying: "MDT",
		Notional:   1_000,
		Liquidity:  goodLiquidity(),
	}
	if err := c.CheckBuy(baseSnapshot(), m); err != nil {
		t.Fatalf("expected healthy buy to pass, got %s (%s)", err.Code, err.Message)
	}
}

func TestCheckBuy_PerOrderCap(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	// Per-order cap is 30% of $6,000 = $1,800. $1,801 must fail.
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 1_801, Liquidity: goodLiquidity()}
	err := c.CheckBuy(s, m)
	if err == nil || err.Code != "per_order" {
		t.Fatalf("expected per_order violation, got %v", err)
	}
	// Exactly $1,800 must pass (boundary).
	m.Notional = 1_800
	if err := c.CheckBuy(s, m); err != nil {
		t.Fatalf("expected boundary $1,800 to pass, got %s", err.Code)
	}
}

func TestCheckBuy_ConcentrationCap(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	// Already holding $1,500 of MDT stock. Per-name cap is 40% of $6,000 =
	// $2,400, so only $900 more of MDT exposure is allowed.
	s.Positions = []Position{{Symbol: "MDT", Underlying: "MDT", AssetType: AssetEquity, MarkValue: 1_500, Quantity: 30}}
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 1_000, Liquidity: goodLiquidity()}
	err := c.CheckBuy(s, m)
	if err == nil || err.Code != "concentration" {
		t.Fatalf("expected concentration violation, got %v", err)
	}
	// $900 more lands exactly on the cap and must pass.
	m.Notional = 900
	if err := c.CheckBuy(s, m); err != nil {
		t.Fatalf("expected $900 top-up to pass at the cap, got %s", err.Code)
	}
}

func TestCheckBuy_ConcentrationCountsOptionsAndEquityTogether(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	// $1,400 MDT stock + $900 MDT calls = $2,300 exposure on MDT. Cap is
	// $2,400, so a $200 buy (-> $2,500) must trip concentration.
	s.Positions = []Position{
		{Symbol: "MDT", Underlying: "MDT", AssetType: AssetEquity, MarkValue: 1_400, Quantity: 28},
		{Symbol: "MDT_OPT", Underlying: "MDT", AssetType: AssetOption, MarkValue: 900, Quantity: 3},
	}
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 200, Liquidity: goodLiquidity()}
	if err := c.CheckBuy(s, m); err == nil || err.Code != "concentration" {
		t.Fatalf("expected concentration to count equity+options together, got %v", err)
	}
}

func TestCheckBuy_OptionsSleeveCap(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	// Bump deployment budget so the sleeve cap is what bites, not the daily
	// budget. Sleeve cap is 50% of $6,000 = $3,000. Already $2,500 of
	// options across other names; a $600 option buy (-> $3,100) must trip.
	s.DeploymentBudget = 6_000
	s.Positions = []Position{{Symbol: "AAPL_OPT", Underlying: "AAPL", AssetType: AssetOption, MarkValue: 2_500, Quantity: 5}}
	m := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "NVDA_OPT", Underlying: "NVDA", Notional: 600, Liquidity: goodLiquidity()}
	if err := c.CheckBuy(s, m); err == nil || err.Code != "options_sleeve" {
		t.Fatalf("expected options_sleeve violation, got %v", err)
	}
	// An equity buy of the same size is not gated by the sleeve cap.
	mEq := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "NVDA", Underlying: "NVDA", Notional: 600, Liquidity: goodLiquidity()}
	if err := c.CheckBuy(s, mEq); err != nil {
		t.Fatalf("equity buy should not hit the options sleeve cap, got %s", err.Code)
	}
}

func TestCheckBuy_SettledCashRule(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	// Only $500 settled, the rest unsettled. A $600 buy must fail on
	// settled cash even though equity and budgets would allow it.
	s.SettledCash = 500
	s.UnsettledCash = 5_500
	s.DeploymentBudget = 6_000
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 600, Liquidity: goodLiquidity()}
	if err := c.CheckBuy(s, m); err == nil || err.Code != "settled_cash" {
		t.Fatalf("expected settled_cash violation, got %v", err)
	}
}

func TestCheckBuy_DailyDeploymentBudget(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot() // budget $3,000
	s.DeployedThisSession = 2_500
	// $600 more would push to $3,100, over the $3,000 budget.
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 600, Liquidity: goodLiquidity()}
	if err := c.CheckBuy(s, m); err == nil || err.Code != "daily_deployment" {
		t.Fatalf("expected daily_deployment violation, got %v", err)
	}
	// Exactly $500 lands on the budget and must pass.
	m.Notional = 500
	if err := c.CheckBuy(s, m); err != nil {
		t.Fatalf("expected $500 to pass at the budget edge, got %s", err.Code)
	}
}

func TestCheckBuy_DrawdownBreaker(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	// High-water $10,000, equity $6,000 = down 40%, past the -35% halt.
	s.HighWaterMark = 10_000
	s.Equity = 6_000
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 500, Liquidity: goodLiquidity()}
	if err := c.CheckBuy(s, m); err == nil || err.Code != "drawdown_halt" {
		t.Fatalf("expected drawdown_halt, got %v", err)
	}
	// Down only 30% (equity $7,000): buys allowed again.
	s.Equity = 7_000
	s.SettledCash = 7_000
	s.DeploymentBudget = 3_500
	if err := c.CheckBuy(s, m); err != nil {
		t.Fatalf("expected buys allowed at -30%%, got %s", err.Code)
	}
}

func TestDrawdownHalted_NoHistoryNeverTrips(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	s.HighWaterMark = 0
	if c.DrawdownHalted(s) {
		t.Fatal("a fresh account with no high-water mark must not be halted")
	}
}

func TestCheckBuy_LiquidityFloors(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	base := Move{Action: ActionBuyOption, AssetType: AssetOption, Symbol: "X_OPT", Underlying: "X", Notional: 500}

	cases := []struct {
		name string
		mut  func(l *LiquidityCtx)
		code string
	}{
		{"cheap stock", func(l *LiquidityCtx) { l.StockPrice = 4.50 }, "liquidity_price"},
		{"small cap", func(l *LiquidityCtx) { l.UnderlyingMktCap = 1_000_000_000 }, "liquidity_mktcap"},
		{"wide spread", func(l *LiquidityCtx) { l.SpreadPct = 0.15 }, "liquidity_spread"},
		{"thin OI", func(l *LiquidityCtx) { l.OptionOpenInterest = 100 }, "liquidity_oi"},
		{"thin volume", func(l *LiquidityCtx) { l.OptionVolume = 10 }, "liquidity_volume"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := goodLiquidity()
			tc.mut(&l)
			m := base
			m.Liquidity = l
			err := c.CheckBuy(s, m)
			if err == nil || err.Code != tc.code {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
		})
	}
}

func TestCheckBuy_OptionLiquidityGatesSkipForEquity(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	// Thin OI/volume should NOT block an equity buy (those gates are
	// options-only); the underlying price/cap/spread still apply.
	l := goodLiquidity()
	l.OptionOpenInterest = 0
	l.OptionVolume = 0
	m := Move{Action: ActionBuyEquity, AssetType: AssetEquity, Symbol: "MDT", Underlying: "MDT", Notional: 500, Liquidity: l}
	if err := c.CheckBuy(s, m); err != nil {
		t.Fatalf("equity buy must ignore option OI/volume floors, got %s", err.Code)
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

func TestCheckSell_AllowedWhileDrawdownHalted(t *testing.T) {
	c := DefaultCaps()
	s := baseSnapshot()
	s.HighWaterMark = 10_000
	s.Equity = 6_000 // halted for buys
	s.Positions = []Position{{Symbol: "MDT", Underlying: "MDT", AssetType: AssetEquity, Quantity: 30, MarkValue: 1_500}}
	if !c.DrawdownHalted(s) {
		t.Fatal("precondition: expected drawdown halted")
	}
	if err := c.CheckSell(s, Move{Action: ActionSellEquity, Symbol: "MDT", Quantity: 30}); err != nil {
		t.Fatalf("de-risking sells must be allowed while halted, got %s", err.Code)
	}
}

func TestHeldOptionsBelowDTE(t *testing.T) {
	c := DefaultCaps() // MinHeldOptionDTE = 7
	s := baseSnapshot()
	s.Positions = []Position{
		{Symbol: "A_OPT", Underlying: "A", AssetType: AssetOption, DTE: 3},
		{Symbol: "B_OPT", Underlying: "B", AssetType: AssetOption, DTE: 14},
		{Symbol: "C", Underlying: "C", AssetType: AssetEquity, DTE: 0}, // equity ignored
		{Symbol: "D_OPT", Underlying: "D", AssetType: AssetOption, DTE: 6},
	}
	got := c.HeldOptionsBelowDTE(s)
	if len(got) != 2 {
		t.Fatalf("expected 2 options under 7 DTE, got %d", len(got))
	}
	if got[0].Symbol != "A_OPT" || got[1].Symbol != "D_OPT" {
		t.Fatalf("unexpected set: %+v", got)
	}
}
