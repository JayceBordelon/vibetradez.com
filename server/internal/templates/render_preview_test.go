package templates

import (
	"os"
	"testing"
)

// TestWriteLiveTradingPreview writes the rendered one-time update to a
// temp path for visual inspection when VT_EMAIL_PREVIEW is set. Skipped
// otherwise, so CI never touches the filesystem.
func TestWriteLiveTradingPreview(t *testing.T) {
	out := os.Getenv("VT_EMAIL_PREVIEW")
	if out == "" {
		t.Skip("set VT_EMAIL_PREVIEW=/path to write a preview")
	}
	html, err := RenderLiveTradingUpdate(LiveTradingUpdateData{
		BaseURL:       "https://vibetradez.com",
		TranscriptURL: "https://vibetradez.com/transcripts/2026-06-09",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := os.WriteFile(out, []byte(html), 0o644); err != nil {
		t.Fatalf("write preview: %v", err)
	}
}
