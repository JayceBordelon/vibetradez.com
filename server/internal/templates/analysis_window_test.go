package templates

import (
	"strings"
	"testing"
)

func TestRenderAnalysisWindow(t *testing.T) {
	html, err := RenderAnalysisWindow(AnalysisWindowData{
		BaseURL:            "https://vibetradez.com",
		TranscriptURL:      "https://vibetradez.com/transcripts/2026-06-08",
		TranscriptDateLong: "June 8, 2026",
	})
	if err != nil {
		t.Fatalf("render analysis-window update: %v", err)
	}
	for _, want := range []string{
		"A longer analysis window",
		"one hour",
		"nine minute",
		"fails safe",
		"More detail in the transcripts",
		"June 8, 2026",
		"https://vibetradez.com/transcripts/2026-06-08",
		"https://vibetradez.com/dashboard",
		"@@VT_UNSUBSCRIBE_URL@@",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("analysis-window email missing %q", want)
		}
	}
	// The template must not ship Go-template syntax: every placeholder fills.
	if strings.Contains(html, "{{") || strings.Contains(html, "}}") {
		t.Error("analysis-window email has unfilled template placeholders")
	}
}
