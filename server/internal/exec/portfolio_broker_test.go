package exec

import (
	"testing"
)

func TestBuildEquityOrder(t *testing.T) {
	o, err := BuildEquityOrder("mdt", "buy", 10, 79.50)
	if err != nil {
		t.Fatalf("valid buy should build, got %v", err)
	}
	if o.OrderType != "LIMIT" || o.Price != 79.50 {
		t.Fatalf("unexpected order envelope: %+v", o)
	}
	if len(o.OrderLegCollection) != 1 {
		t.Fatalf("expected single leg, got %d", len(o.OrderLegCollection))
	}
	leg := o.OrderLegCollection[0]
	if leg.Instruction != "BUY" || leg.Quantity != 10 {
		t.Fatalf("unexpected leg: %+v", leg)
	}
	if leg.Instrument.Symbol != "MDT" || leg.Instrument.AssetType != "EQUITY" {
		t.Fatalf("unexpected instrument: %+v", leg.Instrument)
	}

	if _, err := BuildEquityOrder("MDT", "SELL", 5, 80); err != nil {
		t.Fatalf("valid sell should build, got %v", err)
	}
	if _, err := BuildEquityOrder("MDT", "SHORT", 5, 80); err == nil {
		t.Fatal("invalid side must error")
	}
	if _, err := BuildEquityOrder("MDT", "BUY", 0, 80); err == nil {
		t.Fatal("zero quantity must error")
	}
	if _, err := BuildEquityOrder("MDT", "BUY", 5, 0); err == nil {
		t.Fatal("non-positive limit must error")
	}
	if _, err := BuildEquityOrder("", "BUY", 5, 80); err == nil {
		t.Fatal("empty symbol must error")
	}
}

func TestBuildOptionLimitOrder(t *testing.T) {
	o, err := BuildOptionLimitOrder("MDT   260417C00079000", "buy_to_open", 2, 1.20)
	if err != nil {
		t.Fatalf("valid open should build, got %v", err)
	}
	if o.OrderType != "LIMIT" || o.OrderLegCollection[0].Instruction != "BUY_TO_OPEN" {
		t.Fatalf("unexpected order: %+v", o)
	}
	if o.OrderLegCollection[0].Instrument.AssetType != "OPTION" {
		t.Fatalf("expected OPTION asset type, got %q", o.OrderLegCollection[0].Instrument.AssetType)
	}
	if _, err := BuildOptionLimitOrder("MDT   260417C00079000", "sell_to_close", 2, 1.20); err != nil {
		t.Fatalf("valid close should build, got %v", err)
	}
	if _, err := BuildOptionLimitOrder("MDT   260417C00079000", "BUY", 2, 1.20); err == nil {
		t.Fatal("invalid instruction must error")
	}
	if _, err := BuildOptionLimitOrder("", "BUY_TO_OPEN", 2, 1.20); err == nil {
		t.Fatal("empty OCC must error")
	}
}
