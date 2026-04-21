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

// ClaudePicker runs the same morning trade analysis pipeline as the
// OpenAI Analyzer, but powered by Anthropic Claude. Both pickers consume
// raw sentiment data, do the full Schwab + web-search workflow with the
// same AnalysisPrompt, and independently produce 10 ranked trades. The
// cron pipeline unions both pick sets so the dashboard / emails can show
// what each model would have picked on its own.
type ClaudePicker struct {
	client anthropic.Client
	model  string
	schwab *schwab.Client
}

func NewClaudePicker(apiKey, model string, schwabClient *schwab.Client) *ClaudePicker {
	return &ClaudePicker{
		client: anthropic.NewClient(
			option.WithAPIKey(apiKey),
			option.WithRequestTimeout(10*time.Minute),
		),
		model:  model,
		schwab: schwabClient,
	}
}

// Model returns the Anthropic model identifier this picker is configured with.
func (p *ClaudePicker) Model() string { return p.model }

// GetTopTrades runs the same workflow the OpenAI Analyzer does — pull in
// market signals, do its own Schwab tool calls and web search, then
// emit 10 ranked trades. Crucially, Claude is not given GPT's picks; it
// generates its own from scratch. The returned trades have ClaudeScore /
// ClaudeRationale populated (because Claude is the model that produced
// them); GPTScore / GPTRationale are left at zero for the cron pipeline
// to merge in if GPT also picked the same ticker.
func (p *ClaudePicker) GetTopTrades(ctx context.Context, sentimentData []sentiment.TickerMention) ([]Trade, error) {
	sentimentJSON, err := json.Marshal(sentimentData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sentiment data: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	weekday := time.Now().Weekday().String()

	prompt := fmt.Sprintf(AnalysisPrompt, today, weekday, string(sentimentJSON))

	content, err := p.runConversation(ctx, prompt)
	if err != nil {
		return nil, err
	}

	var raw []gptTradeOutput
	if err := parseJSONResponse(content, &raw, "Claude trades"); err != nil {
		return nil, fmt.Errorf("failed to parse trades from claude response: %w", err)
	}

	trades := make([]Trade, 0, len(raw))
	for _, r := range raw {
		trades = append(trades, Trade{
			Symbol:          r.Symbol,
			ContractType:    r.ContractType,
			StrikePrice:     r.StrikePrice,
			Expiration:      r.Expiration,
			DTE:             r.DTE,
			EstimatedPrice:  r.EstimatedPrice,
			Thesis:          r.Thesis,
			CurrentPrice:    r.CurrentPrice,
			TargetPrice:     r.TargetPrice,
			StopLoss:        r.StopLoss,
			ProfitTarget:    r.ProfitTarget,
			RiskLevel:       r.RiskLevel,
			Catalyst:        r.Catalyst,
			Rank:            r.Rank,
			ClaudeScore:     r.Score,
			ClaudeRationale: r.Rationale,
			PickedByClaude:  true,
		})
	}

	// Enrich with sentiment data the same way the OpenAI Analyzer does so
	// downstream consumers see consistent SentimentScore and MentionCount
	// regardless of which model picked the trade.
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

// WriteVerdicts runs the cross-examination pass for Claude: given the
// other model's pick list (and Claude's own picks for context), returns
// a one-sentence verdict per symbol. No tools are granted; this is a
// pure reasoning pass over the rationales already produced. Errors are
// non-fatal at the caller level.
func (p *ClaudePicker) WriteVerdicts(ctx context.Context, ownTrades, otherTrades []Trade, ownModelName, otherModelName string) (map[string]string, error) {
	if len(otherTrades) == 0 {
		return map[string]string{}, nil
	}

	ownJSON, err := json.Marshal(verdictTradeView(ownTrades))
	if err != nil {
		return nil, fmt.Errorf("marshal own trades: %w", err)
	}
	otherJSON, err := json.Marshal(verdictTradeView(otherTrades))
	if err != nil {
		return nil, fmt.Errorf("marshal other trades: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	weekday := time.Now().Weekday().String()
	prompt := fmt.Sprintf(CrossExaminationPrompt, today, weekday, ownModelName, otherModelName, string(ownJSON), otherModelName, string(otherJSON))

	msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic verdicts messages.new: %w", err)
	}

	var text strings.Builder
	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(tb.Text)
		}
	}
	out := strings.TrimSpace(text.String())
	if out == "" {
		return nil, fmt.Errorf("empty verdict response from claude")
	}

	var verdicts map[string]string
	if err := parseJSONResponse(out, &verdicts, "Claude verdicts"); err != nil {
		return nil, fmt.Errorf("parse verdict json: %w", err)
	}
	return verdicts, nil
}

// verdictTradeView projects a Trade down to the fields a model needs to
// reason about a pick during cross-examination. Strips out scoring
// fields from the OTHER side so each model judges the trade on its
// merits, not on the opposing model's confidence.
type verdictTradeRow struct {
	Rank         int     `json:"rank"`
	Symbol       string  `json:"symbol"`
	ContractType string  `json:"contract_type"`
	StrikePrice  float64 `json:"strike_price"`
	Expiration   string  `json:"expiration"`
	DTE          int     `json:"dte"`
	CurrentPrice float64 `json:"current_price"`
	OptionPrice  float64 `json:"option_price"`
	Catalyst     string  `json:"catalyst"`
	Thesis       string  `json:"thesis"`
	RiskLevel    string  `json:"risk_level"`
}

func verdictTradeView(ts []Trade) []verdictTradeRow {
	out := make([]verdictTradeRow, len(ts))
	for i, t := range ts {
		out[i] = verdictTradeRow{
			Rank:         t.Rank,
			Symbol:       t.Symbol,
			ContractType: t.ContractType,
			StrikePrice:  t.StrikePrice,
			Expiration:   t.Expiration,
			DTE:          t.DTE,
			CurrentPrice: t.CurrentPrice,
			OptionPrice:  t.EstimatedPrice,
			Catalyst:     t.Catalyst,
			Thesis:       t.Thesis,
			RiskLevel:    t.RiskLevel,
		}
	}
	return out
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

func (p *ClaudePicker) runConversation(ctx context.Context, prompt string) (string, error) {
	tools := p.buildTools()

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
	}

	var containerID string

	const maxRounds = 10
	for round := 0; round < maxRounds; round++ {
		_ = round
		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(p.model),
			MaxTokens: 8192,
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

		// Track the code-execution container so follow-up requests can
		// reattach to it (required when web_search triggers code execution).
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
				log.Printf("Claude tool call: %s → %d bytes", b.Name, len(out))
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

	return "", fmt.Errorf("exceeded max claude tool rounds (%d)", maxRounds)
}

// assistantEchoFromRaw rebuilds an assistant MessageParam by round-tripping
// each content block's raw server JSON through param.Override. We avoid
// msg.ToParam() because anthropic-sdk-go v1.35.1 drops the required `type`
// field on code_execution_tool_result error content (and the same pattern
// affects other server-tool result errors), which causes a 400 from
// /v1/messages on the next round.
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
