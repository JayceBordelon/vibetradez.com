package exec

import (
	"math"
	"testing"
)

func TestVolumeWeightedFillPrice_SingleFill(t *testing.T) {
	got := volumeWeightedFillPrice([]orderActivity{
		{
			ActivityType:  "EXECUTION",
			ExecutionType: "FILL",
			ExecutionLegs: []orderExecutionLeg{
				{LegID: 1, Price: 6.09, Quantity: 1},
			},
		},
	})
	if math.Abs(got-6.09) > 1e-9 {
		t.Errorf("single-fill: want 6.09, got %.6f", got)
	}
}

func TestVolumeWeightedFillPrice_MultiplePartialFills(t *testing.T) {
	// Two partial fills: 2 contracts @ $6.00 + 1 contract @ $6.30
	// VWAP = (2*6.00 + 1*6.30) / 3 = 18.30/3 = 6.10
	got := volumeWeightedFillPrice([]orderActivity{
		{
			ActivityType:  "EXECUTION",
			ExecutionType: "FILL",
			ExecutionLegs: []orderExecutionLeg{
				{LegID: 1, Price: 6.00, Quantity: 2},
			},
		},
		{
			ActivityType:  "EXECUTION",
			ExecutionType: "FILL",
			ExecutionLegs: []orderExecutionLeg{
				{LegID: 1, Price: 6.30, Quantity: 1},
			},
		},
	})
	if math.Abs(got-6.10) > 1e-9 {
		t.Errorf("VWAP: want 6.10, got %.6f", got)
	}
}

func TestVolumeWeightedFillPrice_IgnoresNonFillActivities(t *testing.T) {
	// A "REPLACED" or "CANCEL" activity should not contribute, even
	// if it carries an executionLeg with bogus data.
	got := volumeWeightedFillPrice([]orderActivity{
		{
			ActivityType:  "REPLACED",
			ExecutionType: "REPLACE",
			ExecutionLegs: []orderExecutionLeg{
				{LegID: 1, Price: 999, Quantity: 1},
			},
		},
		{
			ActivityType:  "EXECUTION",
			ExecutionType: "FILL",
			ExecutionLegs: []orderExecutionLeg{
				{LegID: 1, Price: 5.50, Quantity: 1},
			},
		},
	})
	if math.Abs(got-5.50) > 1e-9 {
		t.Errorf("expected only FILL to contribute, got %.6f", got)
	}
}

func TestVolumeWeightedFillPrice_EmptyReturnsZero(t *testing.T) {
	if got := volumeWeightedFillPrice(nil); got != 0 {
		t.Errorf("nil input: want 0, got %.6f", got)
	}
	if got := volumeWeightedFillPrice([]orderActivity{}); got != 0 {
		t.Errorf("empty input: want 0, got %.6f", got)
	}
}

func TestVolumeWeightedFillPrice_ZeroQuantitySkipped(t *testing.T) {
	got := volumeWeightedFillPrice([]orderActivity{
		{
			ActivityType:  "EXECUTION",
			ExecutionType: "FILL",
			ExecutionLegs: []orderExecutionLeg{
				{LegID: 1, Price: 5.00, Quantity: 0}, // skipped
				{LegID: 2, Price: 5.50, Quantity: 1},
			},
		},
	})
	if math.Abs(got-5.50) > 1e-9 {
		t.Errorf("expected zero-qty leg ignored, got %.6f", got)
	}
}

func TestEnrichOptionContract_FillsSpecFromOCCSymbol(t *testing.T) {
	bp := BrokerPosition{
		Symbol:       "NVDA  260918C00215000",
		AssetType:    "OPTION",
		Underlying:   "NVDA",
		ContractType: "CALL",
	}
	enrichOptionContract(&bp)
	if bp.Strike != 215 {
		t.Errorf("strike: want 215, got %v", bp.Strike)
	}
	if bp.Expiration != "2026-09-18" {
		t.Errorf("expiration: want 2026-09-18, got %q", bp.Expiration)
	}
	if bp.ContractType != "CALL" || bp.Underlying != "NVDA" {
		t.Errorf("passthrough fields changed: contractType=%q underlying=%q", bp.ContractType, bp.Underlying)
	}
}

func TestEnrichOptionContract_RecoversMissingPutCallAndUnderlying(t *testing.T) {
	// Schwab can omit putCall/underlyingSymbol; GetPositions falls back to
	// underlying = symbol, which for an option is the 21-char OCC string.
	// The decode must recover both so cap exposure keys against the ticker.
	bp := BrokerPosition{
		Symbol:     "AMD   260728P00150500",
		AssetType:  "OPTION",
		Underlying: "AMD   260728P00150500",
	}
	enrichOptionContract(&bp)
	if bp.Strike != 150.5 {
		t.Errorf("strike: want 150.5, got %v", bp.Strike)
	}
	if bp.Expiration != "2026-07-28" {
		t.Errorf("expiration: want 2026-07-28, got %q", bp.Expiration)
	}
	if bp.ContractType != "PUT" {
		t.Errorf("contractType: want PUT, got %q", bp.ContractType)
	}
	if bp.Underlying != "AMD" {
		t.Errorf("underlying: want AMD, got %q", bp.Underlying)
	}
}

func TestEnrichOptionContract_LeavesEquityAndMalformedAlone(t *testing.T) {
	eq := BrokerPosition{Symbol: "GOOGL", AssetType: "EQUITY", Underlying: "GOOGL"}
	enrichOptionContract(&eq)
	if eq.Strike != 0 || eq.Expiration != "" {
		t.Errorf("equity must not be enriched: %+v", eq)
	}
	bad := BrokerPosition{Symbol: "NOT-AN-OCC", AssetType: "OPTION", Underlying: "X"}
	enrichOptionContract(&bad)
	if bad.Strike != 0 || bad.Expiration != "" || bad.Underlying != "X" {
		t.Errorf("malformed OCC must degrade to passthrough: %+v", bad)
	}
}
