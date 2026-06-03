package transcript

import (
	"encoding/json"
	"testing"
)

func TestRecorderOrderingAndShape(t *testing.T) {
	r := New()
	r.SetModel("claude-opus-4-8")
	r.SetModel("ignored-second-set")
	r.AddText(0, "thinking about candidates")
	r.AddText(0, "   ") // dropped (empty after trim)
	r.AddToolUse(0, "get_option_chain", "tu_1", json.RawMessage(`{"symbol":"AAPL"}`))
	r.AddToolResult(0, "get_option_chain", "tu_1", `{"calls":[]}`)
	r.AddText(1, "picking the strike")

	tr := r.Transcript()
	if tr.Model != "claude-opus-4-8" {
		t.Fatalf("model = %q, want claude-opus-4-8 (SetModel must ignore later sets)", tr.Model)
	}
	if len(tr.Events) != 4 {
		t.Fatalf("got %d events, want 4 (empty text dropped): %+v", len(tr.Events), tr.Events)
	}

	want := []struct {
		round int
		typ   EventType
	}{
		{0, EventText},
		{0, EventToolUse},
		{0, EventToolResult},
		{1, EventText},
	}
	for i, w := range want {
		if tr.Events[i].Round != w.round || tr.Events[i].Type != w.typ {
			t.Errorf("event %d = (round %d, %s), want (round %d, %s)", i, tr.Events[i].Round, tr.Events[i].Type, w.round, w.typ)
		}
	}
	if got := string(tr.Events[1].ToolInput); got != `{"symbol":"AAPL"}` {
		t.Errorf("tool input = %s, want {\"symbol\":\"AAPL\"}", got)
	}
}

func TestNilRecorderIsNoOp(t *testing.T) {
	var r *Recorder // nil
	// None of these should panic.
	r.SetModel("x")
	r.AddText(0, "x")
	r.AddThinking(0, "x")
	r.AddToolUse(0, "t", "id", json.RawMessage(`{}`))
	r.AddToolResult(0, "t", "id", "{}")
	if tr := r.Transcript(); tr.Model != "" || len(tr.Events) != 0 {
		t.Fatalf("nil recorder Transcript() should be zero value, got %+v", tr)
	}
}

func TestRedactToolResultAccountFunds(t *testing.T) {
	got := RedactToolResult("get_account_funds", `{"available_usd":12345.67,"currency":"USD"}`)
	var obj map[string]any
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Fatalf("redacted result not valid JSON: %v (%s)", err, got)
	}
	if obj["available_usd"] != "[redacted]" {
		t.Errorf("available_usd = %v, want [redacted]", obj["available_usd"])
	}
	if obj["currency"] != "USD" {
		t.Errorf("currency = %v, want USD preserved", obj["currency"])
	}
}

func TestRedactToolResultUnparseableFundsDropsNumber(t *testing.T) {
	got := RedactToolResult("get_account_funds", `available cash: $12,345.67`)
	if got != `{"available_usd":"[redacted]"}` {
		t.Errorf("unparseable funds result = %q, want a redacted marker (never leak a raw balance)", got)
	}
}

func TestRedactToolResultOtherToolsUntouched(t *testing.T) {
	in := `{"calls":[{"strike":150,"ask":1.23}]}`
	if got := RedactToolResult("get_option_chain", in); got != in {
		t.Errorf("non-funds tool result was modified: got %q, want %q", got, in)
	}
}
