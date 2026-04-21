package trades

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain array",
			in:   `[{"symbol":"AAPL"}]`,
			want: `[{"symbol":"AAPL"}]`,
		},
		{
			name: "plain object",
			in:   `{"verdict":"buy"}`,
			want: `{"verdict":"buy"}`,
		},
		{
			name: "fenced json",
			in:   "```json\n[{\"symbol\":\"AAPL\"}]\n```",
			want: `[{"symbol":"AAPL"}]`,
		},
		{
			name: "fenced with prose around",
			in:   "Here are my picks:\n```json\n[{\"symbol\":\"AAPL\"}]\n```\nGood luck!",
			want: `[{"symbol":"AAPL"}]`,
		},
		{
			name: "prose preamble, bare json",
			in:   `Note: based on my analysis [{"symbol":"AAPL","rank":1}] are the picks.`,
			want: `[{"symbol":"AAPL","rank":1}]`,
		},
		{
			name: "prose postamble, bare json",
			in:   `[{"symbol":"AAPL"}] These are my top picks for the day.`,
			want: `[{"symbol":"AAPL"}]`,
		},
		{
			name: "string literal contains close bracket",
			in:   `[{"thesis":"strong [breakout] signal","symbol":"MSFT"}]`,
			want: `[{"thesis":"strong [breakout] signal","symbol":"MSFT"}]`,
		},
		{
			name: "escaped quote in string",
			in:   `[{"thesis":"earnings \"beat\" expectations"}]`,
			want: `[{"thesis":"earnings \"beat\" expectations"}]`,
		},
		{
			name: "nested objects",
			in:   `Picks: [{"symbol":"AAPL","meta":{"score":9,"sub":{"x":1}}}]`,
			want: `[{"symbol":"AAPL","meta":{"score":9,"sub":{"x":1}}}]`,
		},
		{
			name: "whitespace",
			in:   "\n\n  [{\"symbol\":\"AAPL\"}]  \n",
			want: `[{"symbol":"AAPL"}]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractJSON(tc.in)
			if got != tc.want {
				t.Fatalf("extractJSON mismatch\nwant: %s\ngot:  %s", tc.want, got)
			}
			var any interface{}
			if err := json.Unmarshal([]byte(got), &any); err != nil {
				t.Fatalf("extracted output is not valid JSON: %v\nvalue: %s", err, got)
			}
		})
	}
}

func TestParseJSONResponse_ProseWrappedArray(t *testing.T) {
	raw := `Note: here are today's top picks based on my analysis:
[{"symbol":"AAPL","rank":1},{"symbol":"MSFT","rank":2}]
Hope these help!`

	var out []struct {
		Symbol string `json:"symbol"`
		Rank   int    `json:"rank"`
	}
	if err := parseJSONResponse(raw, &out, "test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 || out[0].Symbol != "AAPL" || out[1].Rank != 2 {
		t.Fatalf("unexpected parse result: %+v", out)
	}
}

func TestParseJSONResponse_PureProseErrors(t *testing.T) {
	raw := `No viable trades available today due to market conditions.`

	var out []map[string]any
	err := parseJSONResponse(raw, &out, "test")
	if err == nil {
		t.Fatal("expected parse error on pure-prose input, got nil")
	}
}

func TestTruncateForLog(t *testing.T) {
	short := "hello"
	if got := truncateForLog(short, 10); got != short {
		t.Fatalf("short string should pass through, got %q", got)
	}

	long := strings.Repeat("a", 100)
	got := truncateForLog(long, 10)
	if !strings.HasPrefix(got, strings.Repeat("a", 10)) {
		t.Fatalf("truncated output missing prefix: %q", got)
	}
	if !strings.Contains(got, "truncated") {
		t.Fatalf("truncated output missing marker: %q", got)
	}
}
