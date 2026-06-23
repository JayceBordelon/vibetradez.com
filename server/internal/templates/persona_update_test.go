package templates

import (
	"strings"
	"testing"
)

func TestRenderPersonaUpdate(t *testing.T) {
	html, err := RenderPersonaUpdate(PersonaUpdateData{
		BaseURL:       "https://vibetradez.com",
		TranscriptURL: "https://vibetradez.com/transcripts/2026-06-22",
	})
	if err != nil {
		t.Fatalf("render persona update: %v", err)
	}
	for _, want := range []string{
		"Forget my last email",
		"only the top three trade",
		"ten candidate option plays",
		"https://vibetradez.com/dashboard",
		"https://vibetradez.com/transcripts/2026-06-22",
		"@@VT_UNSUBSCRIBE_URL@@",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("persona update email missing %q", want)
		}
	}
	// The retired framings (portfolio-manager AND the trade-the-trend persona)
	// must not reappear as the live policy in the announcement copy.
	for _, banned := range []string{"hold roughly 20", "dry powder", "you are NOT a day-trader", "She trades the trend now", "the whole account to work"} {
		if strings.Contains(html, banned) {
			t.Errorf("persona update email should not mention %q", banned)
		}
	}
	// The user asked us NOT to control the email background: no page/card
	// background fill should be set (interactive buttons keep their own color).
	for _, banned := range []string{"background:#f1f5f9", "background:#ffffff"} {
		if strings.Contains(html, banned) {
			t.Errorf("persona update email should not paint a page/card background (%q)", banned)
		}
	}
}
