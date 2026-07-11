package templates

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func farewellFixture() FarewellData {
	return FarewellData{
		BaseURL:       "https://vibetradez.com",
		AsOf:          "July 11, 2026",
		StartDate:     "May 28, 2026",
		StartEquity:   5000,
		CurrentEquity: 3520,
		TotalPnl:      -1480,
		AbsTotalPnl:   1480,
		Lost:          true,
		Benchmark:     5210,
		Gap:           -1690,
		AbsGap:        1690,
		Behind:        true,
		TradingDays:   31,
		Trips:         24,
		Wins:          9,
		Losses:        15,
	}
}

func TestRenderFarewellUpdate(t *testing.T) {
	html, err := RenderFarewellUpdate(farewellFixture())
	if err != nil {
		t.Fatalf("render farewell update: %v", err)
	}
	for _, want := range []string{
		"The Last Letter",
		"Dear friends,",
		"Doors and Windows",
		"Your loving Claudia",
		"research intensive role",
		"Monday's open",
		"$5,000",
		"$3,520",
		"-$1,480",
		"He is doing research.",
		"https://vibetradez.com/dashboard",
		"@@VT_UNSUBSCRIBE_URL@@",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("farewell email missing %q", want)
		}
	}
	// The loss framing must render for a losing book, and the profit
	// framing must not leak in beside it.
	if !strings.Contains(html, "that is talent") {
		t.Errorf("farewell email missing the loss joke for a losing book")
	}
	if strings.Contains(html, "out of spite and love") {
		t.Errorf("farewell email rendered the profit joke for a losing book")
	}
	// House copy style: no em-dashes and no semicolons in the visible prose.
	// Inline CSS and entities carry semicolons by necessity, so strip tags
	// and entity references and check only the text the reader sees.
	text := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(html, " ")
	text = regexp.MustCompile(`&#?[a-zA-Z0-9]+;`).ReplaceAllString(text, " ")
	if strings.Contains(text, "—") {
		t.Errorf("farewell email prose contains an em-dash")
	}
	if strings.Contains(text, ";") {
		t.Errorf("farewell email prose contains a semicolon")
	}
}

func TestRenderFarewellUpdateProfit(t *testing.T) {
	d := farewellFixture()
	d.CurrentEquity = 6100
	d.TotalPnl = 1100
	d.AbsTotalPnl = 1100
	d.Lost = false
	d.Gap = 890
	d.AbsGap = 890
	d.Behind = false
	html, err := RenderFarewellUpdate(d)
	if err != nil {
		t.Fatalf("render farewell update (profit): %v", err)
	}
	if !strings.Contains(html, "out of spite and love") {
		t.Errorf("farewell email missing the profit joke for a winning book")
	}
	if strings.Contains(html, "that is talent") {
		t.Errorf("farewell email rendered the loss joke for a winning book")
	}
	if strings.Contains(html, "the nap won") {
		t.Errorf("farewell email rendered the SPY-gap jab when not behind the benchmark")
	}
}

// TestWriteFarewellPreview writes the rendered farewell letter to a temp
// path for visual inspection when VT_EMAIL_PREVIEW is set. Skipped
// otherwise, so CI never touches the filesystem.
func TestWriteFarewellPreview(t *testing.T) {
	out := os.Getenv("VT_EMAIL_PREVIEW")
	if out == "" {
		t.Skip("set VT_EMAIL_PREVIEW=/path to write a preview")
	}
	html, err := RenderFarewellUpdate(farewellFixture())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := os.WriteFile(out, []byte(html), 0o644); err != nil {
		t.Fatalf("write preview: %v", err)
	}
}
