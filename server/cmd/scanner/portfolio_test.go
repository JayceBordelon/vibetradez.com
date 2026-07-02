package main

import (
	"strings"
	"testing"

	"vibetradez.com/internal/trades"
)

/*
Every model-authored trade email must go out signed by Claudia, with the
model as small-print attribution when configured. The prompt forbids the
model from signing itself, so this stamp is the only signature — losing it
would ship anonymous position-change emails.
*/
func TestSignByClaudia(t *testing.T) {
	html := "<html><body><p>recap</p></body></html>"

	signed := signByClaudia(html, "claude-fable-5")
	if !strings.Contains(signed, "&mdash; Claudia") {
		t.Error("signed email missing the Claudia sign-off")
	}
	if !strings.Contains(signed, "written by claude-fable-5") {
		t.Error("signed email missing the model attribution")
	}
	if idx := strings.Index(signed, "&mdash; Claudia"); idx > strings.Index(signed, "</body>") {
		t.Error("sign-off must be inserted before </body>")
	}

	// No configured model: still signed by Claudia, just without attribution.
	unattributed := signByClaudia(html, "")
	if !strings.Contains(unattributed, "&mdash; Claudia") {
		t.Error("email with empty model must still be signed by Claudia")
	}
	if strings.Contains(unattributed, "written by") {
		t.Error("email with empty model should not carry a dangling attribution")
	}

	// Malformed model-authored HTML without </body>: the sign-off appends.
	if got := signByClaudia("<p>recap</p>", "claude-fable-5"); !strings.Contains(got, "&mdash; Claudia") {
		t.Error("sign-off must append when the model omitted </body>")
	}
}

func strikePtr(v float64) *float64 { return &v }

func TestTripLabel(t *testing.T) {
	cases := []struct {
		name string
		trip trades.RoundTrip
		want string
	}{
		{
			name: "call with strike",
			trip: trades.RoundTrip{Underlying: "NVDA", AssetType: "OPTION", ContractType: "CALL", Strike: strikePtr(190)},
			want: "NVDA $190 calls",
		},
		{
			name: "put with fractional strike",
			trip: trades.RoundTrip{Underlying: "AVGO", AssetType: "OPTION", ContractType: "PUT", Strike: strikePtr(312.5)},
			want: "AVGO $312.5 puts",
		},
		{
			name: "legacy equity",
			trip: trades.RoundTrip{Symbol: "AAPL", AssetType: "EQUITY"},
			want: "AAPL",
		},
		{
			name: "option missing strike",
			trip: trades.RoundTrip{Underlying: "TSLA", AssetType: "OPTION", ContractType: "CALL"},
			want: "TSLA calls",
		},
	}
	for _, c := range cases {
		if got := tripLabel(c.trip); got != c.want {
			t.Errorf("%s: tripLabel = %q, want %q", c.name, got, c.want)
		}
	}
}
