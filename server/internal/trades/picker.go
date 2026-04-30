package trades

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/sentiment"
)

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
)

type Trade struct {
	// ID is the trades.id row identifier, populated by store.SaveMorningTrades
	// + the various read paths. Exposed only to internal callers (omitted from
	// the public JSON wire) so dashboard responses don't leak the row id.
	ID int `json:"-"`

	Symbol         string  `json:"symbol"`
	ContractType   string  `json:"contract_type"`
	StrikePrice    float64 `json:"strike_price"`
	Expiration     string  `json:"expiration"`
	DTE            int     `json:"dte"`
	EstimatedPrice float64 `json:"estimated_price"`
	Thesis         string  `json:"thesis"`
	SentimentScore float64 `json:"sentiment_score"`
	CurrentPrice   float64 `json:"current_price"`
	TargetPrice    float64 `json:"target_price"`
	StopLoss       float64 `json:"stop_loss"`
	RiskLevel      string  `json:"risk_level"`
	Catalyst       string  `json:"catalyst"`
	MentionCount   int     `json:"mention_count"`
	Rank           int     `json:"rank"`

	Score     int    `json:"score"`
	Rationale string `json:"rationale"`
	Model     string `json:"model"`
}

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
	Symbol         string  `json:"symbol"`
	ContractType   string  `json:"contract_type"`
	StrikePrice    float64 `json:"strike_price"`
	Expiration     string  `json:"expiration"`
	DTE            int     `json:"dte"`
	EstimatedPrice float64 `json:"estimated_price"`
	CurrentPrice   float64 `json:"current_price"`
	TargetPrice    float64 `json:"target_price"`
	StopLoss       float64 `json:"stop_loss"`
	RiskLevel      string  `json:"risk_level"`
	Catalyst       string  `json:"catalyst"`
	Thesis         string  `json:"thesis"`
	Score          int     `json:"score"`
	Rationale      string  `json:"rationale"`
	Rank           int     `json:"rank"`
}

func (p *ClaudePicker) GetTopTrades(ctx context.Context, sentimentData []sentiment.TickerMention) ([]Trade, error) {
	sentimentJSON, err := json.Marshal(sentimentData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sentiment data: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	weekday := time.Now().Weekday().String()

	prompt := fmt.Sprintf(AnalysisPrompt, today, weekday, string(sentimentJSON))

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
			Symbol:         r.Symbol,
			ContractType:   r.ContractType,
			StrikePrice:    r.StrikePrice,
			Expiration:     r.Expiration,
			DTE:            r.DTE,
			EstimatedPrice: r.EstimatedPrice,
			Thesis:         r.Thesis,
			CurrentPrice:   r.CurrentPrice,
			TargetPrice:    r.TargetPrice,
			StopLoss:       r.StopLoss,
			RiskLevel:      r.RiskLevel,
			Catalyst:       r.Catalyst,
			Rank:           r.Rank,
			Score:          r.Score,
			Rationale:      r.Rationale,
			Model:          p.model,
		})
	}

	type sentimentInfo struct {
		Score    float64
		Mentions int
	}
	sentimentMap := make(map[string]sentimentInfo, len(sentimentData))
	for _, s := range sentimentData {
		sentimentMap[s.Symbol] = sentimentInfo{Score: s.Sentiment, Mentions: s.Mentions}
	}
	for i := range trades {
		if info, ok := sentimentMap[trades[i].Symbol]; ok {
			trades[i].SentimentScore = info.Score
			trades[i].MentionCount = info.Mentions
		}
	}

	return trades, nil
}

func (p *ClaudePicker) GetEndOfDayAnalysis(ctx context.Context, morningTrades []Trade) ([]TradeSummary, error) {
	tradesJSON, err := json.Marshal(morningTrades)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal morning trades: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	weekday := time.Now().Weekday().String()

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
		msg, err := p.client.Messages.New(ctx, params)
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
