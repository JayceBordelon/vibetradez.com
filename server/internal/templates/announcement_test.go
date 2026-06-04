package templates

import (
	"strings"
	"testing"
)

func TestRenderAnnouncement(t *testing.T) {
	html, err := RenderAnnouncement(AnnouncementData{
		BaseURL:         "https://vibetradez.com",
		StartingBalance: 6375.83,
		BalanceAsOf:     "June 4, 2026",
	})
	if err != nil {
		t.Fatalf("render announcement: %v", err)
	}
	for _, want := range []string{
		"letting Claude drive",
		"Starting balance",
		"$6,375.83",
		"June 4, 2026",
		"How it works",
		"What keeps it in check",
		"What you get",
		"The transcripts",
		"I sexified the UI",
		"Bring your own brokerage",
		"sketchy little harness",
		"raw.githubusercontent.com/JayceBordelon/vibetradez.com/main/server/internal/portfolio/prompt.md",
		"https://github.com/JayceBordelon/vibetradez.com/pull/21",
		"https://vibetradez.com/dashboard",
		"https://vibetradez.com/holdings",
		"https://vibetradez.com/closed",
		"@@VT_UNSUBSCRIBE_URL@@",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("announcement email missing %q", want)
		}
	}
}

func TestRenderAnnouncement_NoBalance(t *testing.T) {
	html, err := RenderAnnouncement(AnnouncementData{BaseURL: "https://vibetradez.com"})
	if err != nil {
		t.Fatalf("render announcement: %v", err)
	}
	if strings.Contains(html, "Starting balance") {
		t.Error("starting-balance block should be hidden when balance is zero")
	}
}
