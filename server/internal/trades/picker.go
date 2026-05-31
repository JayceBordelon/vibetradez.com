package trades

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

	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/sentiment"
)

/*
easternTime returns America/New_York for prompt-date anchoring.
Falls back to UTC if the tzdata bundle is missing (treated the same
as 'panic later'; the alpine runtime image installs tzdata so this
should always succeed in production).
*/
func easternTime() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.UTC
	}
	return loc
}

const (
	maxOutputTokensPicks = 16384
	maxOutputTokensEOD   = 16384

	/*
		maxToolRounds is generous on purpose. The picker walks Schwab quotes,
		option chains, and web search across many tickers; complex mornings
		(earnings clusters, macro events) can chain dozens of tool calls
		before Claude's ready to commit. We'd rather burn API spend than
		short-circuit the analysis with a "too many rounds" abort.
	*/
	maxToolRounds = 30

	/*
		httpRequestTimeout caps a single Anthropic API call (one round of the
		conversation, not the whole conversation). Set high so streaming
		responses with long tool plans don't get cut off mid-flight. The
		conversation-level deadline lives on ctx in the caller.
	*/
	httpRequestTimeout = 30 * time.Minute

	/*
		Retry tuning for transient Anthropic API failures (529 overloaded,
		429 rate-limited, 5xx gateway). Each tool-conversation round retries
		independently, so a single transient blip doesn't discard the
		Schwab tool work already done in earlier rounds. Worst-case wait is
		~2+4+8+16+30 ≈ 60s, well inside the 60-minute morning-cron
		wall-clock budget set by httpRequestTimeout.
	*/
	maxPickerAttempts = 5
	pickerBackoffBase = 2 * time.Second
	pickerBackoffCeil = 30 * time.Second
)

type Trade struct {
	// ID is the trades.id row identifier, populated by store.SaveMorningTrades
	// + the various read paths. Exposed only to internal callers (omitted from
	// the public JSON wire) so dashboard responses don't leak the row id.
	ID int `json:"-"`

	Symbol       string  `json:"symbol"`
	ContractType string  `json:"contract_type"`
	Thesis       string  `json:"thesis"`
	CurrentPrice float64 `json:"current_price"`
	TargetPrice  float64 `json:"target_price"`
	RiskLevel    string  `json:"risk_level"`
	Catalyst     string  `json:"catalyst"`
	MentionCount int     `json:"mention_count"`
	Rank         int     `json:"rank"`

	Score     int    `json:"score"`
	Rationale string `json:"rationale"`
	Model     string `json:"model"`

	// Contract intent: written by the 9:25 picker, consumed by the 9:30
	// executor when resolving against the live option chain. TargetOTMPct
	// is signed-OTM in percent (e.g. 1.5 for a put 1.5% below spot or a
	// call 1.5% above spot; sign is implicit from ContractType). MinDTE is
	// the minimum acceptable days-to-expiration; the executor picks the
	// nearest expiration >= MinDTE.
	TargetOTMPct float64 `json:"target_otm_pct"`
	MinDTE       int     `json:"min_dte"`

	// Resolved at open. Nil until the 9:30 at-open agent picks the
	// contract from the live chain and PlaceBuyToOpenAgent stamps the
	// chosen strike/expiration/dte/estimated_price/stop_loss onto the
	// trade row. The frontend renders "Finding contracts..." for any
	// pick whose StrikePrice is still nil. Stays nil forever for
	// candidates the agent chose to skip.
	StrikePrice    *float64 `json:"strike_price"`
	Expiration     *string  `json:"expiration"`
	DTE            *int     `json:"dte"`
	EstimatedPrice *float64 `json:"estimated_price"`
	StopLoss       *float64 `json:"stop_loss"`
}

// ContractResolved is true once the at-open executor has filled in the
// strike/expiration/dte/estimated_price/stop_loss fields. Use this when
// deciding whether to render a trade card, build an OCC symbol, or run
// it through the basket selector.
func (t *Trade) ContractResolved() bool {
	return t.StrikePrice != nil && t.Expiration != nil && t.EstimatedPrice != nil
}

// Strike returns the resolved strike, or 0 when unresolved.
func (t *Trade) Strike() float64 {
	if t.StrikePrice == nil {
		return 0
	}
	return *t.StrikePrice
}

// ExpirationStr returns the resolved expiration, or "" when unresolved.
func (t *Trade) ExpirationStr() string {
	if t.Expiration == nil {
		return ""
	}
	return *t.Expiration
}

// DTEVal returns the resolved DTE, or 0 when unresolved. (0DTE is a valid
// resolved value, so callers must gate on ContractResolved before
// interpreting a zero here.)
func (t *Trade) DTEVal() int {
	if t.DTE == nil {
		return 0
	}
	return *t.DTE
}

// EstPrice returns the resolved estimated_price, or 0 when unresolved.
func (t *Trade) EstPrice() float64 {
	if t.EstimatedPrice == nil {
		return 0
	}
	return *t.EstimatedPrice
}

// StopLossVal returns the resolved stop_loss, or 0 when unresolved.
func (t *Trade) StopLossVal() float64 {
	if t.StopLoss == nil {
		return 0
	}
	return *t.StopLoss
}

// SetResolvedContract is a convenience setter for tests + the at-open
// resolver that flips a pick from pre-resolution to resolved with one
// call. Production code path also goes through this so the contract-
// resolution invariant lives in one place: ContractResolved() returns
// true iff all five fields are set.
func (t *Trade) SetResolvedContract(strike float64, expiration string, dte int, estimatedPrice, stopLoss float64) {
	s := strike
	e := expiration
	d := dte
	p := estimatedPrice
	sl := stopLoss
	t.StrikePrice = &s
	t.Expiration = &e
	t.DTE = &d
	t.EstimatedPrice = &p
	t.StopLoss = &sl
}

// PtrFloat / PtrString / PtrInt are tiny helpers for tests that build
// Trade literals with selectively-set pointer fields. Exported so they
// can be reused by tests in other packages.
func PtrFloat(v float64) *float64 { return &v }
func PtrString(v string) *string  { return &v }
func PtrInt(v int) *int           { return &v }

type TradeSummary struct {
	Symbol       string  `json:"symbol"`
	ContractType string  `json:"contract_type"`
	StrikePrice  float64 `json:"strike_price"`
	Expiration   string  `json:"expiration"`
	EntryPrice   float64 `json:"entry_price"`
	ClosingPrice float64 `json:"closing_price"`
	StockOpen    float64 `json:"stock_open"`
	StockClose   float64 `json:"stock_close"`
	Notes        string  `json:"notes"`
}

type ClaudePicker struct {
	client anthropic.Client
	model  string
	schwab *schwab.Client
}

func NewClaudePicker(apiKey, model string, schwabClient *schwab.Client) *ClaudePicker {
	return &ClaudePicker{
		client: anthropic.NewClient(
			option.WithAPIKey(apiKey),
			option.WithRequestTimeout(httpRequestTimeout),
		),
		model:  model,
		schwab: schwabClient,
	}
}

func (p *ClaudePicker) Model() string { return p.model }

type claudeTradeOutput struct {
	Symbol       string  `json:"symbol"`
	ContractType string  `json:"contract_type"`
	CurrentPrice float64 `json:"current_price"`
	TargetPrice  float64 `json:"target_price"`
	RiskLevel    string  `json:"risk_level"`
	Catalyst     string  `json:"catalyst"`
	Thesis       string  `json:"thesis"`
	Score        int     `json:"score"`
	Rationale    string  `json:"rationale"`
	Rank         int     `json:"rank"`
	TargetOTMPct float64 `json:"target_otm_pct"`
	MinDTE       int     `json:"min_dte"`
}

func (p *ClaudePicker) GetTopTrades(ctx context.Context, trendingData []sentiment.TickerMention) ([]Trade, error) {
	trendingJSON, err := json.Marshal(trendingData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trending data: %w", err)
	}

	// Container runs in UTC. The morning cron firing at 09:30 ET is
	// safe (calendar date matches UTC at that hour), but any operator-
	// initiated run between ~19:00 ET and midnight ET would otherwise
	// tell Claude "today is X" while the rest of the system saves to
	// date X-1 (ET) via todayDate() in cmd/scanner/main.go.
	nowET := time.Now().In(easternTime())
	today := nowET.Format("2006-01-02")
	weekday := nowET.Weekday().String()

	prompt := fmt.Sprintf(AnalysisPrompt, today, weekday, string(trendingJSON))

	content, err := p.runConversation(ctx, prompt, maxOutputTokensPicks)
	if err != nil {
		return nil, err
	}

	var raw []claudeTradeOutput
	if err := parseJSONResponse(content, &raw, "Claude trades"); err != nil {
		return nil, fmt.Errorf("failed to parse trades from claude response: %w", err)
	}

	trades := make([]Trade, 0, len(raw))
	for _, r := range raw {
		trades = append(trades, Trade{
			Symbol:       r.Symbol,
			ContractType: r.ContractType,
			Thesis:       r.Thesis,
			CurrentPrice: r.CurrentPrice,
			TargetPrice:  r.TargetPrice,
			RiskLevel:    r.RiskLevel,
			Catalyst:     r.Catalyst,
			Rank:         r.Rank,
			Score:        r.Score,
			Rationale:    r.Rationale,
			Model:        p.model,
			TargetOTMPct: r.TargetOTMPct,
			MinDTE:       r.MinDTE,
		})
	}

	mentionMap := make(map[string]int, len(trendingData))
	for _, s := range trendingData {
		mentionMap[s.Symbol] = s.Mentions
	}
	for i := range trades {
		if mentions, ok := mentionMap[trades[i].Symbol]; ok {
			trades[i].MentionCount = mentions
		}
	}

	return trades, nil
}

func (p *ClaudePicker) GetEndOfDayAnalysis(ctx context.Context, morningTrades []Trade) ([]TradeSummary, error) {
	tradesJSON, err := json.Marshal(morningTrades)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal morning trades: %w", err)
	}

	// Same rationale as GetTopTrades — anchor on ET so prompt date
	// matches what cmd/scanner persists.
	nowET := time.Now().In(easternTime())
	today := nowET.Format("2006-01-02")
	weekday := nowET.Weekday().String()

	prompt := fmt.Sprintf(EndOfDayPrompt, today, weekday, string(tradesJSON))

	content, err := p.runConversation(ctx, prompt, maxOutputTokensEOD)
	if err != nil {
		return nil, err
	}

	var summaries []TradeSummary
	if err := parseJSONResponse(content, &summaries, "Claude EOD summaries"); err != nil {
		return nil, fmt.Errorf("failed to parse summaries from claude response: %w", err)
	}
	return summaries, nil
}

func (p *ClaudePicker) buildTools() []anthropic.ToolUnionParam {
	tools := []anthropic.ToolUnionParam{
		{OfWebSearchTool20260209: &anthropic.WebSearchTool20260209Param{
			MaxUses: anthropic.Int(8),
		}},
	}

	if p.schwab != nil && p.schwab.IsConnected() {
		tools = append(tools,
			anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
				Name:        "get_stock_quotes",
				Description: anthropic.String("Get real-time stock quotes from Schwab. Pass comma-separated symbols. Returns last price, bid, ask, open, high, low, volume, and day change."),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: map[string]any{
						"symbols": map[string]any{
							"type":        "string",
							"description": "Comma-separated stock ticker symbols (e.g. 'AAPL,MSFT,TSLA')",
						},
					},
					Required: []string{"symbols"},
				},
			}},
			anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
				Name:        "get_option_chain",
				Description: anthropic.String("Get live option chain from Schwab for a symbol. Returns bid/ask/last/mark, greeks (delta, gamma, theta, vega), open interest, and volume for matching contracts."),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: map[string]any{
						"symbol":        map[string]any{"type": "string", "description": "Stock ticker (e.g. 'AAPL')"},
						"contract_type": map[string]any{"type": "string", "enum": []string{"CALL", "PUT", "ALL"}, "description": "Filter by contract type. Default ALL."},
						"from_date":     map[string]any{"type": "string", "description": "Start date for expiration range (YYYY-MM-DD). Defaults to today."},
						"to_date":       map[string]any{"type": "string", "description": "End date for expiration range (YYYY-MM-DD). Defaults to 7 days out."},
						"strike":        map[string]any{"type": "number", "description": "Filter to a specific strike price."},
					},
					Required: []string{"symbol"},
				},
			}},
		)
	}

	return tools
}

// sendWithRetry calls Messages.New with exponential backoff + jitter on
// transient API failures (529 overloaded, 429 rate-limited, 5xx gateway).
// Non-retryable errors (4xx auth/config, ctx cancel) fast-fail unchanged.
// A previous morning run (2026-05-11) lost the day's picks to a single 529
// after ~15 successful Schwab tool calls; retrying just this call recovers
// without re-running the tool conversation from scratch.
func (p *ClaudePicker) sendWithRetry(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
	return retryWithBackoff(ctx, maxPickerAttempts, backoffDelay, func() (*anthropic.Message, error) {
		return p.client.Messages.New(ctx, params)
	})
}

// retryWithBackoff runs attempt up to maxAttempts times, sleeping delayFor(n)
// between attempt n and attempt n+1 when the error is retryable. Split out
// from sendWithRetry so the retry semantics (call counts, fast-fail on
// non-retryable errors, ctx cancellation mid-sleep) can be unit-tested
// without standing up an Anthropic client.
func retryWithBackoff(
	ctx context.Context,
	maxAttempts int,
	delayFor func(attempt int) time.Duration,
	attempt func() (*anthropic.Message, error),
) (*anthropic.Message, error) {
	var lastErr error
	for i := 1; i <= maxAttempts; i++ {
		msg, err := attempt()
		if err == nil {
			return msg, nil
		}
		lastErr = err
		if !isRetryableAPIError(err) || i == maxAttempts {
			return nil, err
		}
		delay := delayFor(i)
		log.Printf("Anthropic transient failure (attempt %d/%d, retry in %s): %v", i, maxAttempts, delay, err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}

// isRetryableAPIError returns true for Anthropic-side transient failures
// where another attempt has a real chance of succeeding.
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

// backoffDelay returns pickerBackoffBase * 2^(attempt-1) capped at
// pickerBackoffCeil, with ±25% jitter to avoid retry stampedes when
// Anthropic is recovering from an overload spike.
func backoffDelay(attempt int) time.Duration {
	d := pickerBackoffBase << (attempt - 1)
	if d > pickerBackoffCeil {
		d = pickerBackoffCeil
	}
	jitter := time.Duration(rand.Int63n(int64(d) / 2))
	return d - d/4 + jitter
}

func (p *ClaudePicker) runConversation(ctx context.Context, prompt string, maxTokens int64) (string, error) {
	tools := p.buildTools()

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
	}

	var containerID string

	for round := 0; round < maxToolRounds; round++ {
		_ = round
		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(p.model),
			MaxTokens: maxTokens,
			Messages:  messages,
			Tools:     tools,
		}
		if containerID != "" {
			params.Container = param.NewOpt(containerID)
		}
		msg, err := p.sendWithRetry(ctx, params)
		if err != nil {
			return "", fmt.Errorf("anthropic messages.new: %w", err)
		}

		/*
			Track the code-execution container so follow-up requests can
			reattach to it (required when web_search triggers code execution).
		*/
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
				out := p.executeTool(ctx, b.Name, b.Input)
				log.Printf("Claude tool call: %s -> %d bytes", b.Name, len(out))
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
			return "", fmt.Errorf("empty response from claude")
		}
		return text, nil
	}

	return "", fmt.Errorf("exceeded max claude tool rounds (%d)", maxToolRounds)
}

/*
assistantEchoFromRaw rebuilds an assistant MessageParam by round-tripping
each content block's raw server JSON through param.Override. We avoid
msg.ToParam() because anthropic-sdk-go v1.35.1 drops the required `type`
field on code_execution_tool_result error content (and the same pattern
affects other server-tool result errors), which causes a 400 from
/v1/messages on the next round.
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

func (p *ClaudePicker) executeTool(_ context.Context, name string, input json.RawMessage) string {
	switch name {
	case "get_stock_quotes":
		var args struct {
			Symbols string `json:"symbols"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			return `{"error": "invalid arguments"}`
		}
		symbols := strings.Split(args.Symbols, ",")
		for i := range symbols {
			symbols[i] = strings.TrimSpace(symbols[i])
		}
		quotes, err := p.schwab.GetQuotes(symbols)
		if err != nil {
			return fmt.Sprintf(`{"error": %q}`, err.Error())
		}
		out, _ := json.Marshal(quotes)
		return string(out)

	case "get_option_chain":
		var args struct {
			Symbol       string  `json:"symbol"`
			ContractType string  `json:"contract_type"`
			FromDate     string  `json:"from_date"`
			ToDate       string  `json:"to_date"`
			Strike       float64 `json:"strike"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			return `{"error": "invalid arguments"}`
		}
		if args.FromDate == "" {
			args.FromDate = time.Now().Format("2006-01-02")
		}
		if args.ToDate == "" {
			args.ToDate = time.Now().AddDate(0, 0, 7).Format("2006-01-02")
		}
		chain, err := p.schwab.GetOptionChain(args.Symbol, args.ContractType, args.FromDate, args.ToDate, args.Strike)
		if err != nil {
			return fmt.Sprintf(`{"error": %q}`, err.Error())
		}
		out, _ := json.Marshal(chain)
		return string(out)

	default:
		return fmt.Sprintf(`{"error": "unknown function: %s"}`, name)
	}
}

func extractJSON(s string) string {
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

	start := -1
	var open byte
	for i := 0; i < len(s); i++ {
		if s[i] == '{' || s[i] == '[' {
			start = i
			open = s[i]
			break
		}
	}
	if start < 0 {
		return s
	}
	closeB := byte('}')
	if open == '[' {
		closeB = ']'
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
		case open:
			depth++
		case closeB:
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}

func parseJSONResponse(raw string, dst any, source string) error {
	if err := json.Unmarshal([]byte(extractJSON(raw)), dst); err != nil {
		log.Printf("Failed to parse %s JSON (%d bytes raw): %s", source, len(raw), truncateForLog(raw, 2000))
		return err
	}
	return nil
}

func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...[truncated]"
}
