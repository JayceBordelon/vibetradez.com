package templates

import (
	"strings"
	"testing"
)

func TestRenderFullDiscretionUpdate(t *testing.T) {
	html, err := RenderFullDiscretionUpdate(FullDiscretionUpdateData{
		BaseURL:       "https://vibetradez.com",
		TranscriptURL: "https://vibetradez.com/transcripts/2026-06-14",
	})
	if err != nil {
		t.Fatalf("render full-discretion update: %v", err)
	}
	for _, want := range []string{
		"Full discretion",
		"full discretion over how the account is allocated",
		"No allocation split",
		"spends settled cash only",
		"can lose most or all of its value",
		"https://vibetradez.com/dashboard",
		"https://vibetradez.com/transcripts/2026-06-14",
		"@@VT_UNSUBSCRIBE_URL@@",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("full-discretion update email missing %q", want)
		}
	}
	// The removed policy must not be reintroduced by the announcement copy.
	for _, banned := range []string{"half in stock", "half the account", "sleeve", "50%"} {
		if strings.Contains(html, banned) {
			t.Errorf("full-discretion update email should not mention %q", banned)
		}
	}
}
