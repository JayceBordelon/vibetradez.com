/*
Package transcript captures the model's conversation during a tool-use
run (the 9:25 picker and the 9:30 at-open agent) so the reasoning,
tool calls, and tool results can be persisted and surfaced on the
dashboard.

The Recorder is appended to from inside each conversation loop and is
nil-safe: a nil *Recorder swallows every Add call, so the paths that
don't want capture (EOD analysis, tests) can pass nil without guarding.

Capture is narration-only by design: the live trading runs do NOT use
extended thinking, so the "reasoning" surfaced is the assistant text
emitted between tool calls plus the tool calls and results themselves.
The thinking EventType exists so genuine thinking blocks are captured
for free if extended thinking is ever enabled on those runs.
*/
package transcript

import (
	"encoding/json"
	"strings"
)

// EventType labels each entry in a transcript's ordered event stream.
type EventType string

const (
	// EventText is assistant narration emitted between tool calls.
	EventText EventType = "text"
	// EventThinking is an extended-thinking block. Adaptive extended
	// thinking is enabled, so live runs do produce these.
	EventThinking EventType = "thinking"
	// EventToolUse is a tool call the model issued (name + input args).
	EventToolUse EventType = "tool_use"
	// EventToolResult is the result string we fed back for a tool call.
	EventToolResult EventType = "tool_result"
)

/*
Event is one entry in the ordered transcript stream. Round is the
zero-based conversation round it occurred in so the UI can group a
round's narration, tool calls, and their results together. Fields are
omitempty so a text event doesn't carry empty tool fields and vice
versa.
*/
type Event struct {
	Round      int             `json:"round"`
	Type       EventType       `json:"type"`
	Text       string          `json:"text,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	ToolUseID  string          `json:"tool_use_id,omitempty"`
	ToolInput  json.RawMessage `json:"tool_input,omitempty"`
	ToolResult string          `json:"tool_result,omitempty"`
}

// Transcript is the full captured conversation: the model that produced
// it plus the ordered event stream.
type Transcript struct {
	Model  string  `json:"model"`
	Events []Event `json:"events"`
}

/*
Recorder accumulates events as a conversation loop runs. Construct one
with New; pass nil to disable capture. Not safe for concurrent use —
the conversation loops append from a single goroutine.
*/
type Recorder struct {
	model  string
	events []Event
}

// New returns an empty Recorder ready to be appended to.
func New() *Recorder { return &Recorder{} }

// SetModel records the model id once it's known (first round). Nil-safe.
func (r *Recorder) SetModel(model string) {
	if r == nil || r.model != "" || model == "" {
		return
	}
	r.model = model
}

// AddText appends an assistant-narration event. Empty text is dropped.
// Nil-safe.
func (r *Recorder) AddText(round int, text string) {
	if r == nil || strings.TrimSpace(text) == "" {
		return
	}
	r.events = append(r.events, Event{Round: round, Type: EventText, Text: text})
}

// AddThinking appends an extended-thinking event. Empty text is dropped.
// Nil-safe.
func (r *Recorder) AddThinking(round int, text string) {
	if r == nil || strings.TrimSpace(text) == "" {
		return
	}
	r.events = append(r.events, Event{Round: round, Type: EventThinking, Text: text})
}

// AddToolUse appends a tool-call event. Nil-safe.
func (r *Recorder) AddToolUse(round int, name, id string, input json.RawMessage) {
	if r == nil {
		return
	}
	// Copy the input so a later mutation of the SDK's buffer can't alter
	// what we captured.
	var in json.RawMessage
	if len(input) > 0 {
		in = append(json.RawMessage(nil), input...)
	}
	r.events = append(r.events, Event{Round: round, Type: EventToolUse, ToolName: name, ToolUseID: id, ToolInput: in})
}

/*
AddToolResult appends a tool-result event. The result is stored verbatim,
the exact string threaded back into the conversation as the tool_result
block. This is a single real account that publishes its own book, so
balances are shown openly, not redacted. Nil-safe.
*/
func (r *Recorder) AddToolResult(round int, name, id, result string) {
	if r == nil {
		return
	}
	r.events = append(r.events, Event{
		Round:      round,
		Type:       EventToolResult,
		ToolName:   name,
		ToolUseID:  id,
		ToolResult: result,
	})
}

// Transcript returns the accumulated transcript. Safe to call on nil
// (returns the zero value).
func (r *Recorder) Transcript() Transcript {
	if r == nil {
		return Transcript{}
	}
	return Transcript{Model: r.model, Events: r.events}
}
