package templates

import (
	"strings"
	"testing"
)

func TestRenderLiveTradingUpdate(t *testing.T) {
	html, err := RenderLiveTradingUpdate(LiveTradingUpdateData{
		BaseURL:       "https://vibetradez.com",
		TranscriptURL: "https://vibetradez.com/transcripts/2026-06-09",
	})
	if err != nil {
		t.Fatalf("render live-trading update: %v", err)
	}
	for _, want := range []string{
		"Live trading, live everything",
		"trading is now live indefinitely",
		"Everything updates in real time",
		"truer profit and loss picture",
		"executions, on the dashboard",
		"Every trade has its own page",
		"The leash came off",
		"Transcripts you can actually read",
		"Polish everywhere",
		"The flow from here",
		"https://vibetradez.com/dashboard",
		"https://vibetradez.com/transcripts/2026-06-09",
		"@@VT_UNSUBSCRIBE_URL@@",
		"can lose most or all of its value",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("live-trading update email missing %q", want)
		}
	}
}
