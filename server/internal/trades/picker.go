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
// the WSB sentiment, do its own Schwab tool calls and web search, then
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
	if err := json.Unmarshal([]byte(stripMarkdownCodeBlock(content)), &raw); err != nil {
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

	const maxRounds = 10
	for round := 0; round < maxRounds; round++ {
		_ = round
		msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(p.model),
			MaxTokens: 8192,
			Messages:  messages,
			Tools:     tools,
		})
		if err != nil {
			return "", fmt.Errorf("anthropic messages.new: %w", err)
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
			messages = append(messages, msg.ToParam())
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
