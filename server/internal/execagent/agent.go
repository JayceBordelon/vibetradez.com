package execagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	"vibetradez.com/internal/trades"
)

/*
Conversation-loop knobs.

  - maxToolRounds is generous on purpose: the agent walks 3 chains +
    optionally web-searches per candidate, can chain a dozen tool
    calls before reaching a final JSON.
  - maxOutputTokens caps the structured JSON output; well within the
    16k headroom the picker uses.
  - httpRequestTimeout caps a single Messages.New round. The agent
    runs inside a 5-minute cron budget overall (see runExecuteAtOpen);
    no single round should burn more than that.
*/
const (
	maxToolRounds            = 20
	maxOutputTokens    int64 = 4096
	httpRequestTimeout       = 5 * time.Minute

	maxAgentAttempts = 5
	backoffBase      = 2 * time.Second
	backoffCeil      = 30 * time.Second
)

/*
Agent runs the 9:30:00 ET at-open execution loop against Anthropic
Claude. One Agent per process is fine; Run is safe to call sequentially
across days but NOT concurrently (the dispatcher's mutex serializes
within a run, not across).

Construction is deliberately simple — wire the read-side and
write-side dependencies once at startup. Tests can substitute either
side with a stub.
*/
type Agent struct {
	client anthropic.Client
	model  string
	chain  ChainReader
	exec   ExecutionService
}

/*
NewAgent wires the Anthropic client + read/write dependencies. The
apiKey + model are passed in rather than read from env so the caller
(cmd/scanner/main.go) gets one place to inject the same model the
picker uses.
*/
func NewAgent(apiKey, model string, chain ChainReader, exec ExecutionService) *Agent {
	return &Agent{
		client: anthropic.NewClient(
			option.WithAPIKey(apiKey),
			option.WithRequestTimeout(httpRequestTimeout),
		),
		model: model,
		chain: chain,
		exec:  exec,
	}
}

/*
Run is the agent entry point. Given today's 3 morning candidates,
invokes Claude with the agent tool surface, lets it place orders /
explain skips, then reconciles the final JSON output against what was
actually placed via the tool layer and persists skip rows for any
candidate without a buy.

Returns one Decision per Candidate (in input order). On Anthropic API
failure or unparseable final JSON, returns the partial decisions
collected from successful tool calls + an error; caller logs and
sends the error-path email.
*/
func (a *Agent) Run(ctx context.Context, candidates []Candidate) (*ExecagentResult, error) {
	if len(candidates) == 0 {
		return &ExecagentResult{}, nil
	}

	dispatcher := NewToolDispatcher(a.chain, a.exec, candidates)
	prompt, err := buildPrompt(candidates)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	finalJSON, convErr := a.runConversation(ctx, prompt, dispatcher)
	// Even on conversation error, persist skips for every candidate
	// that didn't get a buy — the user gets a fully-reconciled basket
	// regardless of whether the model crashed mid-loop.
	placed := dispatcher.PlacedDecisions()

	var parsed finalJSONShape
	parseErr := parseFinalJSON(finalJSON, &parsed)

	result := reconcile(candidates, placed, parsed, dispatcher)
	if convErr != nil {
		return result, fmt.Errorf("agent conversation: %w", convErr)
	}
	if parseErr != nil {
		return result, fmt.Errorf("parse final JSON: %w", parseErr)
	}
	return result, nil
}

/*
runConversation drives the tool-use loop. Mirrors the picker's
runConversation shape but uses execagent's own tool dispatcher so
state (orders placed, allowlist) is scoped to this run.

Returns the final assistant text (expected to be the JSON object
described in prompt.go) or an error. The text is empty if the model
hit the round cap without producing a final response.
*/
func (a *Agent) runConversation(ctx context.Context, prompt string, dispatcher *ToolDispatcher) (string, error) {
	tools := ToolDefinitions()
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
	}
	var containerID string

	for round := 0; round < maxToolRounds; round++ {
		_ = round
		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(a.model),
			MaxTokens: maxOutputTokens,
			Messages:  messages,
			Tools:     tools,
		}
		if containerID != "" {
			params.Container = param.NewOpt(containerID)
		}
		msg, err := a.sendWithRetry(ctx, params)
		if err != nil {
			return "", fmt.Errorf("anthropic messages.new: %w", err)
		}
		if msg.Container.JSON.ID.Valid() {
			containerID = msg.Container.ID
		}

		var toolResults []anthropic.ContentBlockParamUnion
		var finalText strings.Builder
		for _, block := range msg.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				finalText.WriteString(b.Text)
			case anthropic.ToolUseBlock:
				out := dispatcher.Dispatch(ctx, b.Name, b.Input)
				log.Printf("execagent: tool=%s -> %d bytes", b.Name, len(out))
				toolResults = append(toolResults, anthropic.NewToolResultBlock(b.ID, out, false))
			}
		}

		if len(toolResults) > 0 {
			messages = append(messages, assistantEchoFromRaw(msg))
			messages = append(messages, anthropic.NewUserMessage(toolResults...))
			continue
		}

		text := strings.TrimSpace(finalText.String())
		if text == "" {
			return "", errors.New("empty response from claude")
		}
		return text, nil
	}
	return "", fmt.Errorf("exceeded max tool rounds (%d)", maxToolRounds)
}

/*
sendWithRetry mirrors picker.sendWithRetry. Duplicated rather than
shared so this package can be tested independently and so a future
picker tweak can't quietly change the agent's retry semantics.
*/
func (a *Agent) sendWithRetry(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
	var lastErr error
	for i := 1; i <= maxAgentAttempts; i++ {
		msg, err := a.client.Messages.New(ctx, params)
		if err == nil {
			return msg, nil
		}
		lastErr = err
		if !isRetryableAPIError(err) || i == maxAgentAttempts {
			return nil, err
		}
		delay := backoffDelay(i)
		log.Printf("execagent: Anthropic transient failure (attempt %d/%d, retry in %s): %v", i, maxAgentAttempts, delay, err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}

func isRetryableAPIError(err error) bool {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.StatusCode {
	case 429, 502, 503, 504, 529:
		return true
	}
	return false
}

func backoffDelay(attempt int) time.Duration {
	d := backoffBase << (attempt - 1)
	if d > backoffCeil {
		d = backoffCeil
	}
	jitter := time.Duration(rand.Int63n(int64(d) / 2))
	return d - d/4 + jitter
}

/*
assistantEchoFromRaw rebuilds an assistant MessageParam by round-
tripping each content block's raw server JSON. Same workaround the
picker uses (anthropic-sdk-go v1.35.1 drops the `type` field on
code_execution_tool_result errors when going through ToParam, which
the next round rejects with a 400).
*/
func assistantEchoFromRaw(msg *anthropic.Message) anthropic.MessageParam {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(msg.Content))
	for _, b := range msg.Content {
		raw := b.RawJSON()
		if raw == "" {
			continue
		}
		blocks = append(blocks, param.Override[anthropic.ContentBlockParamUnion](json.RawMessage(raw)))
	}
	return anthropic.NewAssistantMessage(blocks...)
}

/*
finalJSONShape mirrors the schema the prompt describes. The reconciler
treats this as soft input: the source of truth for buys is what
actually got placed via the tool layer; this JSON is consulted only
for skip reasons + the operator's final_note.
*/
type finalJSONShape struct {
	Decisions []struct {
		Rank       int     `json:"rank"`
		Action     string  `json:"action"`
		OrderID    string  `json:"order_id,omitempty"`
		Strike     float64 `json:"strike,omitempty"`
		Expiration string  `json:"expiration,omitempty"`
		LimitPrice float64 `json:"limit_price,omitempty"`
		Quantity   int     `json:"quantity,omitempty"`
		SkipReason string  `json:"skip_reason,omitempty"`
	} `json:"decisions"`
	FinalNote string `json:"final_note"`
}

/*
parseFinalJSON extracts a JSON object from the model's text response.
Handles plain JSON, fenced JSON (```json ... ```), and stray prose
before/after the object. Mirrors the picker's extractJSON heuristic.
*/
func parseFinalJSON(raw string, dst *finalJSONShape) error {
	cleaned := extractJSONObject(raw)
	if cleaned == "" {
		return errors.New("no JSON object found in response")
	}
	return json.Unmarshal([]byte(cleaned), dst)
}

func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "```"); i >= 0 {
		rest := s[i+3:]
		if nl := strings.Index(rest, "\n"); nl >= 0 {
			rest = rest[nl+1:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			s = strings.TrimSpace(rest[:end])
		}
	}
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if inStr {
			switch c {
			case '\\':
				escape = true
			case '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}

/*
reconcile produces the final per-candidate Decision slice. Logic:

 1. For each candidate, prefer the buy persisted via the tool layer
    (PlacedDecisions). That's the ground truth — Schwab already saw
    the order, the execution row already exists.
 2. If no buy was placed, look for a skip in the model's final JSON.
    Write a 'skipped' execution row with the reason; copy it into
    the Decision.
 3. If neither a buy nor a model-provided skip exists, synthesize a
    skip with reason "agent did not act on this candidate" so the
    basket reconciles cleanly. (Defensive: the prompt requires
    every candidate appear in decisions[], but a model regression
    shouldn't strand the day.)

Decision order matches the input candidate order so callers (the
summary email, the dashboard) can rely on rank ordering without
re-sorting.
*/
func reconcile(
	candidates []Candidate,
	placed map[int]Decision,
	parsed finalJSONShape,
	dispatcher *ToolDispatcher,
) *ExecagentResult {
	skipByRank := make(map[int]string, len(parsed.Decisions))
	for _, d := range parsed.Decisions {
		if strings.EqualFold(d.Action, string(ActionSkip)) {
			skipByRank[d.Rank] = d.SkipReason
		}
	}

	out := &ExecagentResult{
		Decisions: make([]Decision, 0, len(candidates)),
		FinalNote: strings.TrimSpace(parsed.FinalNote),
	}
	for i := range candidates {
		t := &candidates[i].Trade
		if dec, ok := placed[t.Rank]; ok {
			out.Decisions = append(out.Decisions, dec)
			continue
		}
		reason := skipByRank[t.Rank]
		if strings.TrimSpace(reason) == "" {
			reason = "Agent did not place an order for this candidate and did not provide an explicit skip reason."
		}
		execID, err := dispatcher.RecordSkip(t.Rank, reason)
		if err != nil {
			log.Printf("execagent: reconcile: RecordSkip rank=%d: %v", t.Rank, err)
		}
		out.Decisions = append(out.Decisions, Decision{
			Rank:       t.Rank,
			Symbol:     t.Symbol,
			Action:     ActionSkip,
			SkipReason: reason,
			SkipExecID: execID,
		})
	}
	return out
}

/*
buildPrompt formats the per-candidate context into the system prompt.
Candidates are rendered as a compact JSON array so the model gets
typed structure rather than free-form bullets.
*/
func buildPrompt(candidates []Candidate) (string, error) {
	type ctxPick struct {
		Rank         int     `json:"rank"`
		Symbol       string  `json:"symbol"`
		ContractType string  `json:"contract_type"`
		Score        int     `json:"score"`
		CurrentPrice float64 `json:"picker_current_price"`
		TargetOTMPct float64 `json:"target_otm_pct"`
		MinDTE       int     `json:"min_dte"`
		Thesis       string  `json:"thesis"`
		Rationale    string  `json:"rationale"`
		Catalyst     string  `json:"catalyst,omitempty"`
		RiskLevel    string  `json:"risk_level"`
	}
	rows := make([]ctxPick, 0, len(candidates))
	for _, c := range candidates {
		t := c.Trade
		rows = append(rows, ctxPick{
			Rank:         t.Rank,
			Symbol:       t.Symbol,
			ContractType: t.ContractType,
			Score:        t.Score,
			CurrentPrice: t.CurrentPrice,
			TargetOTMPct: t.TargetOTMPct,
			MinDTE:       t.MinDTE,
			Thesis:       t.Thesis,
			Rationale:    t.Rationale,
			Catalyst:     t.Catalyst,
			RiskLevel:    t.RiskLevel,
		})
	}
	pickJSON, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return "", err
	}

	now := time.Now().In(easternTime())
	today := now.Format("2006-01-02")
	weekday := now.Weekday().String()
	return fmt.Sprintf(SystemPrompt, today, weekday, string(pickJSON)), nil
}

func easternTime() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.UTC
	}
	return loc
}

/*
CandidatesFromTrades is a small helper for callers that have a slice
of picker.Trade and want to feed it to Run without iterating
themselves. Filters out trades with rank outside [1, MaxOrdersPerRun].
*/
func CandidatesFromTrades(in []trades.Trade) []Candidate {
	out := make([]Candidate, 0, len(in))
	for i := range in {
		t := in[i]
		if t.Rank < 1 || t.Rank > MaxOrdersPerRun {
			continue
		}
		out = append(out, Candidate{Trade: t})
	}
	return out
}
