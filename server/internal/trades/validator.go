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
)

type Validator struct {
	client anthropic.Client
	model  string
	schwab *schwab.Client
}

func NewValidator(apiKey, model string, schwabClient *schwab.Client) *Validator {
	return &Validator{
		client: anthropic.NewClient(
			option.WithAPIKey(apiKey),
			option.WithRequestTimeout(120*time.Second),
		),
		model:  model,
		schwab: schwabClient,
	}
}

// Model returns the Anthropic model identifier this validator is configured with.
func (v *Validator) Model() string { return v.model }

// ValidateTrades sends GPT's picks to Claude for an independent score
// and rationale per trade. The returned slice has one Validation per
// input trade, indexed by symbol.
func (v *Validator) ValidateTrades(ctx context.Context, trades []Trade) ([]Validation, error) {
	if len(trades) == 0 {
		return nil, nil
	}

	tradesJSON, err := json.Marshal(trades)
	if err != nil {
		return nil, fmt.Errorf("marshal trades for claude: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	weekday := time.Now().Weekday().String()
	prompt := fmt.Sprintf(ClaudeValidationPrompt, today, weekday, string(tradesJSON))

	content, err := v.runConversation(ctx, prompt)
	if err != nil {
		return nil, err
	}

	var validations []Validation
	if err := json.Unmarshal([]byte(stripMarkdownCodeBlock(content)), &validations); err != nil {
		return nil, fmt.Errorf("parse claude validations: %w", err)
	}
	return validations, nil
}

func (v *Validator) buildTools() []anthropic.ToolUnionParam {
	tools := []anthropic.ToolUnionParam{
		{OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{
			MaxUses: anthropic.Int(8),
		}},
	}

	if v.schwab != nil && v.schwab.IsConnected() {
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

func (v *Validator) runConversation(ctx context.Context, prompt string) (string, error) {
	tools := v.buildTools()

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
	}

	const maxRounds = 10
	for round := 0; round < maxRounds; round++ {
		msg, err := v.client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(v.model),
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
				out := v.executeTool(ctx, b.Name, b.Input)
				log.Printf("Claude tool call: %s → %d bytes", b.Name, len(out))
				toolResults = append(toolResults, anthropic.NewToolResultBlock(b.ID, out, false))
			}
		}

		// If Claude returned tool calls we need to execute them and loop.
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

func (v *Validator) executeTool(_ context.Context, name string, input json.RawMessage) string {
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
		quotes, err := v.schwab.GetQuotes(symbols)
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
		chain, err := v.schwab.GetOptionChain(args.Symbol, args.ContractType, args.FromDate, args.ToDate, args.Strike)
		if err != nil {
			return fmt.Sprintf(`{"error": %q}`, err.Error())
		}
		out, _ := json.Marshal(chain)
		return string(out)

	default:
		return fmt.Sprintf(`{"error": "unknown function: %s"}`, name)
	}
}
