package templates

import (
	"strings"
	"testing"
)

func fableReturnFixture() FableReturnData {
	return FableReturnData{
		BaseURL:       "https://vibetradez.com",
		TranscriptURL: "https://vibetradez.com/transcripts/2026-07-02",
		AsOf:          "July 2, 2026",
		AccountEquity: 24310.55,
		Benchmark:     26102.80,
		Gap:           -1792.25,
		GapClosed:     1436.10,
		Behind:        true,
		Wins: []FableReturnWin{
			{Label: "NVDA $190 calls", ClosedLong: "June 27, 2026", HoldDays: 3, Pnl: 812.40, Pct: 41.2},
			{Label: "AVGO $310 calls", ClosedLong: "June 24, 2026", HoldDays: 5, Pnl: 505.00, Pct: 27.9},
		},
		LossCount:   1,
		LossTotal:   -260.15,
		NetRealized: 1057.25,
		WindowLabel: "the last two weeks",
	}
}

func TestRenderFableReturnUpdate(t *testing.T) {
	html, err := RenderFableReturnUpdate(fableReturnFixture())
	if err != nil {
		t.Fatalf("render fable-return update: %v", err)
	}
	for _, want := range []string{
		"My brain is back",
		"Fable 5 is live again",
		"$24,310.55", // account value, thousands-grouped
		"$26,102.80", // SPY benchmark
		"-$1,792.25", // the gap, signed
		// html/template escapes '+' to &#43; (anti-UTF-7); it still renders
		// as a plus sign in every mail client.
		"&#43;$1,436.10", // gap recovered
		"NVDA $190 calls",
		"&#43;$812.40",
		"(&#43;41.2%)",
		"AVGO $310 calls",
		"the last two weeks",
		"and 1 trade we don&rsquo;t talk about",
		"-$260.15",
		"&#43;$1,057.25",  // honest net including the loss
		"&mdash; Claudia", // the sign-off the user asked for
		"once again running Claude Fable 5",
		"https://vibetradez.com/dashboard",
		"https://vibetradez.com/transcripts/2026-07-02",
		"@@VT_UNSUBSCRIBE_URL@@",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("fable-return email missing %q", want)
		}
	}
	// The user asked us NOT to control the email background: no page/card
	// background fill should be set (interactive buttons keep their own color).
	for _, banned := range []string{"background:#f1f5f9", "background:#ffffff"} {
		if strings.Contains(html, banned) {
			t.Errorf("fable-return email should not paint a page/card background (%q)", banned)
		}
	}
}

// A fresh account can have curve data but no completed round trips yet; the
// receipts section must drop cleanly instead of rendering an empty table.
func TestRenderFableReturnUpdateNoWins(t *testing.T) {
	d := fableReturnFixture()
	d.Wins = nil
	d.LossCount = 0
	d.LossTotal = 0
	d.NetRealized = 0
	html, err := RenderFableReturnUpdate(d)
	if err != nil {
		t.Fatalf("render fable-return update without wins: %v", err)
	}
	for _, banned := range []string{"receipts attached", "Net over the window"} {
		if strings.Contains(html, banned) {
			t.Errorf("fable-return email without wins should not contain %q", banned)
		}
	}
	if !strings.Contains(html, "&mdash; Claudia") {
		t.Error("fable-return email without wins must still be signed by Claudia")
	}
}

// Ahead-of-SPY copy: the behind-only consolation paragraph must disappear.
func TestRenderFableReturnUpdateAhead(t *testing.T) {
	d := fableReturnFixture()
	d.Behind = false
	d.Gap = 312.44
	html, err := RenderFableReturnUpdate(d)
	if err != nil {
		t.Fatalf("render fable-return update (ahead): %v", err)
	}
	if strings.Contains(html, "still ahead of me") {
		t.Error("ahead variant should not include the behind-SPY consolation copy")
	}
	if !strings.Contains(html, "currently ahead, please clap") {
		t.Error("ahead variant should label the gap as ahead")
	}
}
