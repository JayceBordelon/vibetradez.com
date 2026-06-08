package portfolio

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

	"vibetradez.com/internal/transcript"
)

/*
Conversation-loop knobs. maxToolRounds is generous: the agent reviews
existing holdings, researches new names, and may chain many quote / chain /
web_search calls before committing moves. Mirrors the execagent shape this
package supersedes.
*/
const (
	maxToolRounds = 30
	/*
		Adaptive extended thinking is enabled and its depth is driven by the
		OutputConfig effort level, not a fixed token budget (opus-4.8+ reject
		the legacy thinking.type=enabled + budget_tokens shape). max_tokens
		still caps the whole response, so it must comfortably cover both the
		thinking and the visible tool calls + final stance JSON.
	*/
	maxOutputTokens    int64 = 32000
	httpRequestTimeout       = 8 * time.Minute
	// Cap the server-tool (web_search / web_fetch) result JSON stored in the
	// transcript so a fetched page can't bloat the record.
	serverResultCap = 8000

	maxAgentAttempts = 5
	backoffBase      = 2 * time.Second
	backoffCeil      = 30 * time.Second
)

/*
Agent runs the daily portfolio-management loop against Anthropic Claude.
One Agent per process is fine; Run is safe to call sequentially across
sessions but NOT concurrently (the dispatcher's mutex serializes within a
run, not across).
*/
type Agent struct {
	client anthropic.Client
	model  string
	reader PortfolioReader
	exec   PortfolioExecutor
	caps   Caps
}

/*
NewAgent wires the Anthropic client + read/write dependencies + the cap
policy. apiKey + model are injected by the caller (cmd/scanner/main.go) so
the portfolio agent uses the same model the rest of the system does. Pass
DefaultCaps() unless an override is configured.
*/
func NewAgent(apiKey, model string, reader PortfolioReader, exec PortfolioExecutor, caps Caps) *Agent {
	return &Agent{
		client: anthropic.NewClient(
			option.WithAPIKey(apiKey),
			option.WithRequestTimeout(httpRequestTimeout),
		),
		model:  model,
		reader: reader,
		exec:   exec,
		caps:   caps,
	}
}

/*
Run is the agent entry point. It reads the live account snapshot, invokes
Claude with the portfolio tool surface, lets it commit moves (or hold)
through the tool layer, then returns every committed decision plus the
final stance note.

The committed moves are the source of truth (they are already at the
broker and persisted); the final JSON is consulted only for the stance
note (mirrors execagent's buy-is-truth / JSON-is-narrative split). On
Anthropic API failure or unparseable final JSON, Run returns the partial
decisions collected from successful tool calls plus an error so the caller
can log and send the error-path email.
*/
func (a *Agent) Run(ctx context.Context) (*Result, error) {
	snap, err := a.reader.Snapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("read portfolio snapshot: %w", err)
	}

	dispatcher := NewToolDispatcher(a.reader, a.exec, a.caps, snap)
	prompt := buildPrompt(a.caps)

	rec := transcript.New()
	start := time.Now()
	stance, convErr := a.runConversation(ctx, prompt, dispatcher, rec)
	rec.SetDuration(time.Since(start).Milliseconds())

	tr := rec.Transcript()
	result := &Result{
		Decisions:   dispatcher.Decisions(),
		Stance:      stance,
		Summary:     dispatcher.Summary(),
		ActionItems: dispatcher.ActionItems(),
		Transcript:  &tr,
	}
	if convErr != nil {
		return result, fmt.Errorf("agent conversation: %w", convErr)
	}
	return result, nil
}

/*
runConversation drives the tool-use loop and returns the model's final
stance string (parsed from its terminal JSON object). Mirrors
execagent.runConversation: container threading, raw assistant echo, and the
retry wrapper are duplicated rather than shared so this package can be
tested and tuned independently.
*/
func (a *Agent) runConversation(ctx context.Context, prompt string, dispatcher *ToolDispatcher, rec *transcript.Recorder) (string, error) {
	tools := ToolDefinitions()
	rec.SetModel(a.model)

	// Prompt caching. Mark the static prefix so each turn re-reads it from
	// cache (~0.1x input price) instead of paying full input price every
	// round: the tool schemas (identical across sessions) and the system
	// prompt (identical within a session). A rolling breakpoint on the newest
	// tool results (set in the loop) caches the growing conversation too.
	// Ephemeral, 5-minute TTL, which comfortably covers one multi-round
	// session where turns are seconds apart.
	if n := len(tools); n > 0 && tools[n-1].OfTool != nil {
		tools[n-1].OfTool.CacheControl = anthropic.NewCacheControlEphemeralParam()
	}
	promptBlock := anthropic.NewTextBlock(prompt)
	if promptBlock.OfText != nil {
		promptBlock.OfText.CacheControl = anthropic.NewCacheControlEphemeralParam()
	}
	messages := []anthropic.MessageParam{anthropic.NewUserMessage(promptBlock)}
	var containerID string
	var prevCacheBlock *anthropic.ToolResultBlockParam // rolling conversation cache breakpoint

	for round := 0; round < maxToolRounds; round++ {
		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(a.model),
			MaxTokens: maxOutputTokens,
			Messages:  messages,
			Tools:     tools,
			// Adaptive extended thinking: the model reasons before each move and
			// picks its own thinking depth, dialed by the OutputConfig effort
			// level. The thinking blocks come back in msg.Content and are
			// recorded into the transcript and threaded back (with signatures)
			// via the raw assistant echo so multi-round tool use stays valid.
			//
			// Display MUST be set explicitly: on opus-4.8/4.7 the server-side
			// default is "omitted", which returns thinking blocks with an empty
			// Thinking field (signature only), so the transcript would capture no
			// reasoning at all. "summarized" returns the readable summary we show.
			Thinking: anthropic.ThinkingConfigParamUnion{OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{
				Display: anthropic.ThinkingConfigAdaptiveDisplaySummarized,
			}},
			OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortHigh},
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

		// Accumulate this round's token spend before any early return, so the
		// session total reflects every API call (including a truncated one).
		rec.AddUsage(transcript.Usage{
			InputTokens:         msg.Usage.InputTokens,
			OutputTokens:        msg.Usage.OutputTokens,
			CacheReadTokens:     msg.Usage.CacheReadInputTokens,
			CacheCreationTokens: msg.Usage.CacheCreationInputTokens,
			WebSearchRequests:   msg.Usage.ServerToolUse.WebSearchRequests,
			WebFetchRequests:    msg.Usage.ServerToolUse.WebFetchRequests,
		})

		// Guard the max_tokens stop: at effort=high the model can think
		// extensively, and summarized thinking adds visible output, so a turn
		// can hit the maxOutputTokens ceiling. A truncated turn may carry an
		// unterminated tool_use block; echoing it back to continue the loop
		// would 400 (or commit a half-formed move). Fail cleanly instead so the
		// session's fail-safe records a no-action day rather than corrupting the
		// conversation.
		if msg.StopReason == anthropic.StopReasonMaxTokens {
			return "", fmt.Errorf("response truncated at max_tokens (%d) in round %d; raise maxOutputTokens or lower effort", maxOutputTokens, round)
		}

		var toolResults []anthropic.ContentBlockParamUnion
		var finalText strings.Builder
		for _, block := range msg.Content {
			switch b := block.AsAny().(type) {
			case anthropic.ThinkingBlock:
				rec.AddThinking(round, b.Thinking)
			case anthropic.RedactedThinkingBlock:
				rec.AddThinking(round, "[redacted thinking]")
			case anthropic.TextBlock:
				finalText.WriteString(b.Text)
				rec.AddText(round, b.Text)
			case anthropic.ToolUseBlock:
				rec.AddToolUse(round, b.Name, b.ID, b.Input)
				out := dispatcher.Dispatch(ctx, b.Name, b.Input)
				log.Printf("portfolio: tool=%s -> %d bytes", b.Name, len(out))
				rec.AddToolResult(round, b.Name, b.ID, out)
				toolResults = append(toolResults, anthropic.NewToolResultBlock(b.ID, out, false))
			case anthropic.ServerToolUseBlock:
				// web_search / web_fetch / code_execution run server-side at
				// Anthropic, so there is no local result to feed back: we only
				// record the call and (below) its result so the model's web
				// research and computations are captured in the transcript
				// instead of vanishing.
				input, _ := json.Marshal(b.Input)
				rec.AddToolUse(round, string(b.Name), b.ID, input)
			case anthropic.WebSearchToolResultBlock:
				rec.AddToolResult(round, "web_search", b.ToolUseID, truncJSON(b.Content, serverResultCap))
			case anthropic.WebFetchToolResultBlock:
				rec.AddToolResult(round, "web_fetch", b.ToolUseID, truncJSON(b.Content, serverResultCap))
			case anthropic.CodeExecutionToolResultBlock:
				// Python code_execution result (stdout/stderr/return_code, or an
				// error). Without this case the call recorded above would show in
				// the transcript with no output at all.
				rec.AddToolResult(round, "code_execution", b.ToolUseID, truncJSON(b.Content, serverResultCap))
			case anthropic.BashCodeExecutionToolResultBlock:
				rec.AddToolResult(round, "bash_code_execution", b.ToolUseID, truncJSON(b.Content, serverResultCap))
			case anthropic.TextEditorCodeExecutionToolResultBlock:
				// File create/view/edit run by the auto-enabled code-execution
				// sandbox (its third sub-tool, alongside python + bash).
				rec.AddToolResult(round, "text_editor_code_execution", b.ToolUseID, truncJSON(b.Content, serverResultCap))
			}
		}

		if len(toolResults) > 0 {
			// Roll the conversation cache breakpoint onto the newest tool
			// results and clear the prior one, so we never exceed the 4
			// cache_control blocks the API allows (tools + prompt + this one).
			if last := toolResults[len(toolResults)-1].OfToolResult; last != nil {
				if prevCacheBlock != nil {
					prevCacheBlock.CacheControl = anthropic.CacheControlEphemeralParam{}
				}
				last.CacheControl = anthropic.NewCacheControlEphemeralParam()
				prevCacheBlock = last
			}
			messages = append(messages, assistantEchoFromRaw(msg))
			messages = append(messages, anthropic.NewUserMessage(toolResults...))
			continue
		}

		text := strings.TrimSpace(finalText.String())
		if text == "" {
			return "", errors.New("empty response from claude")
		}
		return parseStance(text), nil
	}
	return "", fmt.Errorf("exceeded max tool rounds (%d)", maxToolRounds)
}

// truncJSON marshals a server-tool result and caps its length so a large
// fetched page can't bloat the stored transcript. A truncated (non-JSON)
// string is rendered as-is by the transcript view.
func truncJSON(v any, max int) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	if len(b) > max {
		return string(b[:max]) + "…[truncated]"
	}
	return string(b)
}

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
		log.Printf("portfolio: Anthropic transient failure (attempt %d/%d, retry in %s): %v", i, maxAgentAttempts, delay, err)
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
assistantEchoFromRaw rebuilds an assistant MessageParam by round-tripping
each content block's raw server JSON. Same workaround the picker + execagent
use (the SDK drops the `type` field on some tool-result blocks going through
ToParam, which the next round rejects with a 400).
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
parseStance extracts the "stance" field from the model's terminal JSON
object. The moves are already recorded by the tool layer, so a missing or
malformed stance is non-fatal: we fall back to the raw trimmed text rather
than failing the session.
*/
func parseStance(raw string) string {
	cleaned := extractJSONObject(raw)
	if cleaned != "" {
		var shape struct {
			Stance string `json:"stance"`
		}
		if err := json.Unmarshal([]byte(cleaned), &shape); err == nil && strings.TrimSpace(shape.Stance) != "" {
			return strings.TrimSpace(shape.Stance)
		}
	}
	return strings.TrimSpace(raw)
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
buildPrompt interpolates today's date and the cap summary into the system
prompt. The live book and the prior session are NOT embedded here: the model
reads them itself through get_portfolio and get_recent_decisions, so the
prompt stays free of duplicated, per-day state.
*/
func buildPrompt(caps Caps) string {
	now := time.Now().In(easternTime())
	today := now.Format("2006-01-02")
	weekday := now.Weekday().String()
	return fmt.Sprintf(SystemPrompt, today, weekday, CapsSummary(caps))
}

func easternTime() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.UTC
	}
	return loc
}
