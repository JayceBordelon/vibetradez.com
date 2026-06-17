package templates

import (
	"strings"
	"testing"
)

func TestRenderOptionsOnlyUpdate(t *testing.T) {
	html, err := RenderOptionsOnlyUpdate(OptionsOnlyUpdateData{
		BaseURL:       "https://vibetradez.com",
		TranscriptURL: "https://vibetradez.com/transcripts/2026-06-17",
	})
	if err != nil {
		t.Fatalf("render options-only update: %v", err)
	}
	for _, want := range []string{
		"Options only now",
		"options only",
		"boring as hell",
		"liquidated back to cash",
		"https://vibetradez.com/dashboard",
		"https://vibetradez.com/transcripts/2026-06-17",
		"@@VT_UNSUBSCRIBE_URL@@",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("options-only update email missing %q", want)
		}
	}
	// The reversed (pre-pivot) policy must not reappear in the announcement copy.
	for _, banned := range []string{"full discretion", "any mix of stocks", "no allocation cap"} {
		if strings.Contains(html, banned) {
			t.Errorf("options-only update email should not mention %q", banned)
		}
	}
	// The user asked us NOT to control the email background: no page/card
	// background fill should be set (interactive buttons keep their own color).
	for _, banned := range []string{"background:#f1f5f9", "background:#ffffff"} {
		if strings.Contains(html, banned) {
			t.Errorf("options-only update email should not paint a page/card background (%q)", banned)
		}
	}
}
