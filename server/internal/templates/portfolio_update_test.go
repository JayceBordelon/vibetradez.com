package templates

import (
	"strings"
	"testing"
)

func TestRenderPortfolioUpdate(t *testing.T) {
	data := PortfolioUpdateData{
		Date: "2026-06-03", DateLong: "June 3, 2026", Equity: 6234.50, DayChangePct: 1.8, HasDayChange: true,
		SettledCash: 1820, InvestedPct: 71, UnrealizedPnL: 142.5,
		Summary:          "Trimmed the loser, added to the core, and held the rest.",
		ActionItems:      "Confirm the MDT trim filled; watch AAPL into earnings.",
		ActionItemsLabel: "Tomorrow's action items",
		Stance:           "Leaning long, trimmed the loser.",
		Moves: []PortfolioMove{
			{Action: "Buy", Label: "MDT", Notional: 1200, Rationale: "Adding to the core.", IsBuy: true},
			{Action: "Sell", Label: "AAPL 200C 2026-06-19", Notional: 0, Rationale: "Thesis played out.", IsSell: true},
			{Action: "Hold", Rationale: "Nothing else cleared the bar.", IsHold: true},
		},
		Holdings:      []PortfolioHolding{{Label: "MDT", MarketValue: 2400, UnrealizedPnL: 120}},
		TranscriptURL: "https://vibetradez.com/transcripts/2026-06-03",
		DashboardURL:  "https://vibetradez.com/dashboard",
	}
	html, err := RenderPortfolioUpdate(data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	for _, want := range []string{
		"Daily recap &middot; June 3, 2026", "Today's synopsis", "Trimmed the loser", "MDT", "Adding to the core",
		"action items", "Confirm the MDT trim filled",
		"$6,234.50", // currency now carries thousands separators
		"https://vibetradez.com/transcripts/2026-06-03", "Read the full session",
		"@@VT_UNSUBSCRIBE_URL@@",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered email missing %q", want)
		}
	}
	// No em-dash glyph in the rendered copy (project copy rule).
	if strings.Contains(html, "—") || strings.Contains(html, "&mdash;") {
		t.Errorf("rendered email contains an em-dash")
	}
}
