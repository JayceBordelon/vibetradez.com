package main

import (
	"math"
	"testing"

	"vibetradez.com/internal/exec"
)

func TestRoundToTick(t *testing.T) {
	cases := []struct {
		price    float64
		isOption bool
		want     float64
	}{
		{0.874, true, 0.87},      // sub-$3 option: penny grid, floored
		{4.87, true, 4.85},       // $3+ option: nickel grid, floored
		{3.00, true, 3.00},       // exact nickel boundary stays put
		{0.01, true, 0.01},       // floor of the grid
		{182.499, false, 182.49}, // equity: penny grid, floored
	}
	for _, c := range cases {
		got := roundToTick(c.price, c.isOption)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("roundToTick(%v, %v) = %v, want %v", c.price, c.isOption, got, c.want)
		}
	}
}

func TestLiquidationLadderOption(t *testing.T) {
	// 2 contracts marked $310 total -> $1.55 per share of premium.
	p := exec.BrokerPosition{AssetType: "OPTION", Quantity: 2, MarketValue: 310}
	ladder := liquidationLadder(p, true)
	if len(ladder) == 0 || ladder[0] != 0.01 {
		t.Fatalf("ladder must start at the $0.01 'regardless of price' rung, got %v", ladder)
	}
	for i := 1; i < len(ladder); i++ {
		if ladder[i] <= ladder[i-1] {
			t.Errorf("ladder must strictly rise toward the mark, got %v", ladder)
		}
		if ladder[i] > 1.55 {
			t.Errorf("no rung should exceed the position's own mark, got %v", ladder)
		}
	}
}

func TestLiquidationLadderWorthlessOption(t *testing.T) {
	// A worthless mark must still produce the minimum sellable rung and
	// no duplicates.
	p := exec.BrokerPosition{AssetType: "OPTION", Quantity: 1, MarketValue: 0}
	ladder := liquidationLadder(p, true)
	if len(ladder) != 1 || ladder[0] != 0.01 {
		t.Errorf("worthless option ladder should collapse to [0.01], got %v", ladder)
	}
}
