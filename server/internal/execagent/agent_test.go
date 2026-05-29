package execagent

import (
	"strings"
	"testing"

	"vibetradez.com/internal/trades"
)

/*
Agent reconciliation tests. We don't unit-test runConversation directly
(that would require mocking the entire Anthropic SDK transport, which
is high-effort low-signal); the conversation behavior is exercised
end-to-end against the local stack at boot time. What MUST be tested
in isolation is the reconcile() logic: given a set of buys persisted
via the tool layer and a set of (possibly-malformed) final-JSON
intentions from Claude, the per-candidate Decision slice the runner
returns must be coherent.
*/

func TestReconcile_BuyFromTool_TakesPrecedence(t *testing.T) {
	// The tool layer placed an order at rank 1. Claude's final JSON
	// claims rank 1 was skipped. The tool layer is ground truth —
	// the buy wins, the JSON skip claim is ignored.
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL"), mkCandidate(2, "TSLA", "PUT")}
	placed := map[int]Decision{
		1: {Rank: 1, Symbol: "AAPL", Action: ActionBuy, OrderID: "ord-1", Strike: 150, Expiration: "2099-01-01", LimitPrice: 2.30, ExecutionID: 101},
	}
	parsed := finalJSONShape{
		Decisions: []struct {
			Rank       int     `json:"rank"`
			Action     string  `json:"action"`
			OrderID    string  `json:"order_id,omitempty"`
			Strike     float64 `json:"strike,omitempty"`
			Expiration string  `json:"expiration,omitempty"`
			LimitPrice float64 `json:"limit_price,omitempty"`
			Quantity   int     `json:"quantity,omitempty"`
			SkipReason string  `json:"skip_reason,omitempty"`
		}{
			{Rank: 1, Action: "skip", SkipReason: "later second thoughts"},
			{Rank: 2, Action: "skip", SkipReason: "premium tripled at the open"},
		},
		FinalNote: "Choppy open.",
	}
	d := NewToolDispatcher(&stubChain{}, &stubExec{}, candidates)

	result := reconcile(candidates, placed, parsed, d)
	if len(result.Decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(result.Decisions))
	}
	if result.Decisions[0].Action != ActionBuy {
		t.Errorf("rank 1 should be buy (tool layer wins), got %s", result.Decisions[0].Action)
	}
	if result.Decisions[0].OrderID != "ord-1" {
		t.Errorf("rank 1 OrderID lost: %+v", result.Decisions[0])
	}
	if result.Decisions[1].Action != ActionSkip {
		t.Errorf("rank 2 should be skip, got %s", result.Decisions[1].Action)
	}
	if result.Decisions[1].SkipReason != "premium tripled at the open" {
		t.Errorf("rank 2 reason: %q", result.Decisions[1].SkipReason)
	}
	if result.FinalNote != "Choppy open." {
		t.Errorf("FinalNote: %q", result.FinalNote)
	}
}

func TestReconcile_MissingJSON_SynthesizesSkip(t *testing.T) {
	// Claude's final JSON omitted a candidate entirely. The reconciler
	// must still emit a Decision for it with a synthetic skip reason
	// so the basket reconciles + the dashboard has something to render.
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL"), mkCandidate(2, "TSLA", "PUT"), mkCandidate(3, "MSFT", "CALL")}
	placed := map[int]Decision{
		1: {Rank: 1, Symbol: "AAPL", Action: ActionBuy, OrderID: "ord-1"},
	}
	parsed := finalJSONShape{
		Decisions: []struct {
			Rank       int     `json:"rank"`
			Action     string  `json:"action"`
			OrderID    string  `json:"order_id,omitempty"`
			Strike     float64 `json:"strike,omitempty"`
			Expiration string  `json:"expiration,omitempty"`
			LimitPrice float64 `json:"limit_price,omitempty"`
			Quantity   int     `json:"quantity,omitempty"`
			SkipReason string  `json:"skip_reason,omitempty"`
		}{
			{Rank: 1, Action: "buy", OrderID: "ord-1"},
			// rank 2 + 3 missing entirely
		},
	}
	exec := &stubExec{}
	d := NewToolDispatcher(&stubChain{}, exec, candidates)

	result := reconcile(candidates, placed, parsed, d)
	if len(result.Decisions) != 3 {
		t.Fatalf("expected 3 decisions, got %d", len(result.Decisions))
	}
	for _, dec := range result.Decisions[1:] {
		if dec.Action != ActionSkip {
			t.Errorf("rank %d should be skip, got %s", dec.Rank, dec.Action)
		}
		if !strings.Contains(dec.SkipReason, "did not place an order") {
			t.Errorf("rank %d: expected synthetic skip reason, got %q", dec.Rank, dec.SkipReason)
		}
	}
	if len(exec.skipCalls) != 2 {
		t.Errorf("expected 2 synthetic skip persists, got %d", len(exec.skipCalls))
	}
}

func TestReconcile_PreservesOrderByCandidate(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL"), mkCandidate(2, "TSLA", "PUT"), mkCandidate(3, "MSFT", "CALL")}
	placed := map[int]Decision{
		1: {Rank: 1, Symbol: "AAPL", Action: ActionBuy, OrderID: "ord-A"},
		3: {Rank: 3, Symbol: "MSFT", Action: ActionBuy, OrderID: "ord-M"},
	}
	parsed := finalJSONShape{
		Decisions: []struct {
			Rank       int     `json:"rank"`
			Action     string  `json:"action"`
			OrderID    string  `json:"order_id,omitempty"`
			Strike     float64 `json:"strike,omitempty"`
			Expiration string  `json:"expiration,omitempty"`
			LimitPrice float64 `json:"limit_price,omitempty"`
			Quantity   int     `json:"quantity,omitempty"`
			SkipReason string  `json:"skip_reason,omitempty"`
		}{
			{Rank: 2, Action: "skip", SkipReason: "spread too wide"},
		},
	}
	d := NewToolDispatcher(&stubChain{}, &stubExec{}, candidates)

	result := reconcile(candidates, placed, parsed, d)
	if len(result.Decisions) != 3 {
		t.Fatalf("expected 3 decisions, got %d", len(result.Decisions))
	}
	if result.Decisions[0].Rank != 1 || result.Decisions[1].Rank != 2 || result.Decisions[2].Rank != 3 {
		t.Errorf("rank order broken: %v %v %v", result.Decisions[0].Rank, result.Decisions[1].Rank, result.Decisions[2].Rank)
	}
}

func TestExtractJSONObject_HandlesFencedAndPlain(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", `{"decisions":[]}`, `{"decisions":[]}`},
		{"fenced", "```json\n{\"decisions\":[]}\n```", `{"decisions":[]}`},
		{"prose-prefix", "Sure, here's the JSON: {\"decisions\":[]}", `{"decisions":[]}`},
		{"nested-braces", `{"a":{"b":1},"c":[1,2,3]}`, `{"a":{"b":1},"c":[1,2,3]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractJSONObject(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCandidatesFromTrades_FiltersOutOfRangeRanks(t *testing.T) {
	in := []trades.Trade{
		{Rank: 1, Symbol: "A"},
		{Rank: 2, Symbol: "B"},
		{Rank: 3, Symbol: "C"},
		{Rank: 4, Symbol: "D"}, // out of range
		{Rank: 0, Symbol: "E"}, // out of range
	}
	out := CandidatesFromTrades(in)
	if len(out) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(out))
	}
}
