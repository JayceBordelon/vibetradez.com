/*
Package transcript captures the portfolio agent's daily session so the
reasoning, tool calls, tool results, token usage, and session duration can
be persisted and surfaced on the dashboard.

The Recorder is appended to from inside the conversation loop and is
nil-safe: a nil *Recorder swallows every Add call, so the paths that
don't want capture (EOD analysis, tests) can pass nil without guarding.

What's captured: the model's summarized extended-thinking blocks, the
assistant narration emitted between tool calls, the tool calls and their
results, and the accumulated token Usage plus the wall-clock duration.
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
	// EventSessionMarker labels the seam between two intraday sessions merged
	// into one day's transcript (open / midday / pre-close). Its Text is the
	// human session label; the UI renders it as a session divider and builds
	// the day's session jump bar from these markers.
	EventSessionMarker EventType = "session_marker"
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

	// Set only on EventSessionMarker events: the token usage and wall-clock of
	// the session this marker opens, so the UI can break the merged day's cost
	// and timing down by session (open / midday / pre-close).
	Usage      *Usage `json:"usage,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

/*
Usage is the accumulated token spend for a whole session, summed across
every API round. Kept as plain int64 fields (no SDK import) so the agent
maps the SDK's per-call usage into it. InputTokens here is the sum of the
fresh (uncached) input tokens billed each round; CacheReadTokens is what
was served from the prompt cache instead.
*/
type Usage struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	WebSearchRequests   int64 `json:"web_search_requests"`
	WebFetchRequests    int64 `json:"web_fetch_requests"`
	Rounds              int   `json:"rounds"`
}

// Transcript is the full captured conversation: the model that produced
// it, the ordered event stream, the session token usage, and how long the
// session took in milliseconds.
type Transcript struct {
	Model      string  `json:"model"`
	Events     []Event `json:"events"`
	Usage      Usage   `json:"usage"`
	DurationMS int64   `json:"duration_ms"`
}

/*
Recorder accumulates events as a conversation loop runs. Construct one
with New; pass nil to disable capture. Not safe for concurrent use —
the conversation loops append from a single goroutine.
*/
type Recorder struct {
	model      string
	events     []Event
	usage      Usage
	durationMS int64
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

/*
AddUsage adds one API round's token spend to the session total and bumps
the round count. Nil-safe. The caller maps the SDK's per-call usage into
the plain Usage struct.
*/
func (r *Recorder) AddUsage(u Usage) {
	if r == nil {
		return
	}
	r.usage.InputTokens += u.InputTokens
	r.usage.OutputTokens += u.OutputTokens
	r.usage.CacheReadTokens += u.CacheReadTokens
	r.usage.CacheCreationTokens += u.CacheCreationTokens
	r.usage.WebSearchRequests += u.WebSearchRequests
	r.usage.WebFetchRequests += u.WebFetchRequests
	r.usage.Rounds++
}

// SetDuration records the session wall-clock in milliseconds. Nil-safe.
func (r *Recorder) SetDuration(ms int64) {
	if r == nil {
		return
	}
	r.durationMS = ms
}

// Transcript returns the accumulated transcript. Safe to call on nil
// (returns the zero value).
func (r *Recorder) Transcript() Transcript {
	if r == nil {
		return Transcript{}
	}
	return Transcript{Model: r.model, Events: r.events, Usage: r.usage, DurationMS: r.durationMS}
}

/*
Merge folds a later intraday session's transcript into the day's existing
one so the three daily sessions (open / midday / close) share a single
(date, kind) row and render as one continuous reasoning log. The later
events are appended after the prior ones with their Round numbers shifted
past the prior session's highest round, so the UI's per-round grouping
stays monotonic across the seam. A non-empty separatorLabel is inserted as
a session-marker event (EventSessionMarker) marking where the new session
begins, so the UI can render a session divider and jump bar. Token usage and
wall-clock duration are summed; the model id prefers the later session's
(they are normally identical). Merging into a zero-value prior (no row yet)
just returns next with the separator prepended, so the first session of the
day is handled by the same path.
*/
func Merge(prior, next Transcript, separatorLabel string) Transcript {
	offset := 0
	for _, e := range prior.Events {
		if e.Round+1 > offset {
			offset = e.Round + 1
		}
	}
	merged := make([]Event, 0, len(prior.Events)+len(next.Events)+1)
	merged = append(merged, prior.Events...)
	if separatorLabel != "" {
		// Carry the incoming session's own usage + wall-clock on its marker so
		// the UI can attribute cost and timing to each session of the day.
		u := next.Usage
		merged = append(merged, Event{Round: offset, Type: EventSessionMarker, Text: separatorLabel, Usage: &u, DurationMS: next.DurationMS})
	}
	for _, e := range next.Events {
		e.Round += offset
		merged = append(merged, e)
	}
	model := next.Model
	if model == "" {
		model = prior.Model
	}
	return Transcript{
		Model:  model,
		Events: merged,
		Usage: Usage{
			InputTokens:         prior.Usage.InputTokens + next.Usage.InputTokens,
			OutputTokens:        prior.Usage.OutputTokens + next.Usage.OutputTokens,
			CacheReadTokens:     prior.Usage.CacheReadTokens + next.Usage.CacheReadTokens,
			CacheCreationTokens: prior.Usage.CacheCreationTokens + next.Usage.CacheCreationTokens,
			WebSearchRequests:   prior.Usage.WebSearchRequests + next.Usage.WebSearchRequests,
			WebFetchRequests:    prior.Usage.WebFetchRequests + next.Usage.WebFetchRequests,
			Rounds:              prior.Usage.Rounds + next.Usage.Rounds,
		},
		DurationMS: prior.DurationMS + next.DurationMS,
	}
}
