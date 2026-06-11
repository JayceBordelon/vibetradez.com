package server

import (
	"fmt"
	"testing"
)

// The granularity tiers: minute candles for short holds, daily beyond,
// daily when the window is unbounded or malformed.
func TestIntradayFreq_Tiers(t *testing.T) {
	cases := []struct {
		start, end string
		want       int
	}{
		{"2026-06-10", "2026-06-10", 15}, // same day
		{"2026-06-08", "2026-06-10", 15}, // 3 days
		{"2026-06-01", "2026-06-10", 30}, // 10 days
		{"2026-05-25", "2026-06-10", 0},  // 17 days: daily
		{"", "2026-06-10", 0},            // unbounded start: daily
		{"2026-06-12", "2026-06-10", 0},  // inverted window: daily
	}
	for _, c := range cases {
		if got := intradayFreq(c.start, c.end); got != c.want {
			t.Errorf("intradayFreq(%q, %q) = %d, want %d", c.start, c.end, got, c.want)
		}
	}
}

func TestThinPoints_BudgetAndLastPoint(t *testing.T) {
	pts := make([]pricePointView, 250)
	for i := range pts {
		pts[i] = pricePointView{Date: fmt.Sprintf("d%03d", i), Close: float64(i)}
	}
	out := thinPoints(pts)
	if len(out) > 91 {
		t.Fatalf("thinned to %d points, want <= 91", len(out))
	}
	if out[0].Date != "d000" || out[len(out)-1].Date != "d249" {
		t.Fatalf("endpoints not preserved: first %s last %s", out[0].Date, out[len(out)-1].Date)
	}

	small := pts[:40]
	if got := thinPoints(small); len(got) != 40 {
		t.Fatalf("under-budget series must pass through, got %d", len(got))
	}
}
