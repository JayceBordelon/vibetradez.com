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
