package trades

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/sentiment"
)

type Trade struct {
	Symbol         string  `json:"symbol"`
	ContractType   string  `json:"contract_type"` // CALL or PUT
	StrikePrice    float64 `json:"strike_price"`
	Expiration     string  `json:"expiration"`
	DTE            int     `json:"dte"`
	EstimatedPrice float64 `json:"estimated_price"`
	Thesis         string  `json:"thesis"`
	SentimentScore float64 `json:"sentiment_score"`
	CurrentPrice   float64 `json:"current_price"`
	TargetPrice    float64 `json:"target_price"`
	StopLoss       float64 `json:"stop_loss"`
	ProfitTarget   float64 `json:"profit_target"`
	RiskLevel      string  `json:"risk_level"`
	Catalyst       string  `json:"catalyst"`
	MentionCount   int     `json:"mention_count"`
	Rank           int     `json:"rank"`

	// Dual-model scoring. Each side rates the trade 1-10 and explains why.
	// Rank above is the final ordering after combining both scores.
	GPTScore        int     `json:"gpt_score"`
	GPTRationale    string  `json:"gpt_rationale"`
	ClaudeScore     int     `json:"claude_score"`
	ClaudeRationale string  `json:"claude_rationale"`
	CombinedScore   float64 `json:"combined_score"`

	// Picker attribution. Both models run the full AnalysisPrompt
	// independently and each return their own 10 picks; the union of
	// both pick sets is persisted, and these flags record which model(s)
	// actually picked the trade. The All view shows everything; the
	// OpenAI / Claude filter views show only the rows where the matching
	// flag is true.
	PickedByOpenAI bool `json:"picked_by_openai"`
	PickedByClaude bool `json:"picked_by_claude"`
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

type Analyzer struct {
	client openai.Client
	model  string
	schwab *schwab.Client
}

func NewAnalyzer(apiKey, model string, schwabClient *schwab.Client) *Analyzer {
	return &Analyzer{
		client: openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithRequestTimeout(120*time.Second),
		),
		model:  model,
		schwab: schwabClient,
	}
}

// Model returns the OpenAI model identifier this analyzer is configured with.
func (a *Analyzer) Model() string { return a.model }

// gptTradeOutput is the JSON shape we ask GPT to return. We map it onto
// Trade and stamp GPTScore / GPTRationale from the score / rationale fields.
type gptTradeOutput struct {
	Symbol         string  `json:"symbol"`
	ContractType   string  `json:"contract_type"`
	StrikePrice    float64 `json:"strike_price"`
	Expiration     string  `json:"expiration"`
	DTE            int     `json:"dte"`
	EstimatedPrice float64 `json:"estimated_price"`
	CurrentPrice   float64 `json:"current_price"`
	TargetPrice    float64 `json:"target_price"`
	StopLoss       float64 `json:"stop_loss"`
	ProfitTarget   float64 `json:"profit_target"`
	RiskLevel      string  `json:"risk_level"`
	Catalyst       string  `json:"catalyst"`
	Thesis         string  `json:"thesis"`
	Score          int     `json:"score"`
	Rationale      string  `json:"rationale"`
	Rank           int     `json:"rank"`
}

func (a *Analyzer) GetTopTrades(ctx context.Context, sentimentData []sentiment.TickerMention) ([]Trade, error) {
	sentimentJSON, err := json.Marshal(sentimentData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sentiment data: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	weekday := time.Now().Weekday().String()

	prompt := fmt.Sprintf(AnalysisPrompt, today, weekday, string(sentimentJSON))

	content, err := a.callWithTools(ctx, prompt, 0.7)
	if err != nil {
		return nil, err
	}

	var raw []gptTradeOutput
	if err := json.Unmarshal([]byte(stripMarkdownCodeBlock(content)), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse trades from OpenAI response: %w", err)
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
			ProfitTarget:   r.ProfitTarget,
			RiskLevel:      r.RiskLevel,
			Catalyst:       r.Catalyst,
			Rank:           r.Rank,
			GPTScore:       r.Score,
			GPTRationale:   r.Rationale,
			PickedByOpenAI: true,
		})
	}

	// Enrich with sentiment data
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

func (a *Analyzer) GetEndOfDayAnalysis(ctx context.Context, morningTrades []Trade) ([]TradeSummary, error) {
	tradesJSON, err := json.Marshal(morningTrades)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal morning trades: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	weekday := time.Now().Weekday().String()

	prompt := fmt.Sprintf(EndOfDayPrompt, today, weekday, string(tradesJSON))

	content, err := a.callWithTools(ctx, prompt, 0.3)
	if err != nil {
		return nil, err
	}

	var summaries []TradeSummary
	if err := json.Unmarshal([]byte(stripMarkdownCodeBlock(content)), &summaries); err != nil {
		return nil, fmt.Errorf("failed to parse summaries from OpenAI response: %w", err)
	}
	return summaries, nil
}

// ── Multi-turn function calling with Schwab market data tools ──

func (a *Analyzer) buildTools() []responses.ToolUnionParam {
	tools := []responses.ToolUnionParam{
		{OfWebSearchPreview: &responses.WebSearchPreviewToolParam{
			Type: responses.WebSearchPreviewToolTypeWebSearchPreview,
		}},
	}

	if a.schwab != nil && a.schwab.IsConnected() {
		tools = append(tools,
			responses.ToolUnionParam{OfFunction: &responses.FunctionToolParam{
				Name:        "get_stock_quotes",
				Description: openai.String("Get real-time stock quotes from Schwab. Returns last price, bid, ask, open, high, low, volume, and day change for each symbol."),
				Parameters:  schwabQuotesSchema,
			}},
			responses.ToolUnionParam{OfFunction: &responses.FunctionToolParam{
				Name:        "get_option_chain",
				Description: openai.String("Get live option chain from Schwab for a symbol. Returns bid/ask/last/mark, greeks (delta, gamma, theta, vega), open interest, and volume for matching contracts."),
				Parameters:  schwabOptionChainSchema,
			}},
		)
	}
	return tools
}

var schwabQuotesSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"symbols": map[string]any{
			"type":        "string",
			"description": "Comma-separated stock ticker symbols (e.g. 'AAPL,MSFT,TSLA')",
		},
	},
	"required":             []string{"symbols"},
	"additionalProperties": false,
}

var schwabOptionChainSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"symbol": map[string]any{
			"type":        "string",
			"description": "Stock ticker symbol (e.g. 'AAPL')",
		},
		"contract_type": map[string]any{
			"type":        "string",
			"enum":        []string{"CALL", "PUT", "ALL"},
			"description": "Filter by contract type. Default ALL.",
		},
		"from_date": map[string]any{
			"type":        "string",
			"description": "Start date for expiration range (YYYY-MM-DD). Defaults to today.",
		},
		"to_date": map[string]any{
			"type":        "string",
			"description": "End date for expiration range (YYYY-MM-DD). Defaults to 7 days out.",
		},
		"strike": map[string]any{
			"type":        "number",
			"description": "Filter to a specific strike price.",
		},
	},
	"required":             []string{"symbol"},
	"additionalProperties": false,
}

func (a *Analyzer) callWithTools(ctx context.Context, prompt string, temp float64) (string, error) {
	tools := a.buildTools()

	params := responses.ResponseNewParams{
		Model:       shared.ResponsesModel(a.model),
		Input:       responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
		Tools:       tools,
		Temperature: openai.Float(temp),
	}

	const maxRounds = 10
	for round := 0; round < maxRounds; round++ {
		resp, err := a.client.Responses.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("openai responses.new: %w", err)
		}

		var funcCalls []responses.ResponseFunctionToolCall
		for _, item := range resp.Output {
			if item.Type == "function_call" {
				funcCalls = append(funcCalls, item.AsFunctionCall())
			}
		}

		if len(funcCalls) == 0 {
			text := resp.OutputText()
			if text == "" {
				return "", fmt.Errorf("empty response from OpenAI")
			}
			return text, nil
		}

		outputs := make(responses.ResponseInputParam, 0, len(funcCalls))
		for _, fc := range funcCalls {
			result := a.executeFunction(ctx, fc.Name, fc.Arguments)
			log.Printf("Tool call: %s → %d bytes", fc.Name, len(result))
			outputs = append(outputs, responses.ResponseInputItemUnionParam{
				OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
					CallID: fc.CallID,
					Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
						OfString: openai.String(result),
					},
				},
			})
		}

		params = responses.ResponseNewParams{
			Model:              shared.ResponsesModel(a.model),
			Input:              responses.ResponseNewParamsInputUnion{OfInputItemList: outputs},
			Tools:              tools,
			Temperature:        openai.Float(temp),
			PreviousResponseID: openai.String(resp.ID),
		}
	}

	return "", fmt.Errorf("exceeded max function call rounds (%d)", maxRounds)
}

func (a *Analyzer) executeFunction(ctx context.Context, name, arguments string) string {
	switch name {
	case "get_stock_quotes":
		return a.execGetStockQuotes(ctx, arguments)
	case "get_option_chain":
		return a.execGetOptionChain(ctx, arguments)
	default:
		return fmt.Sprintf(`{"error": "unknown function: %s"}`, name)
	}
}

func (a *Analyzer) execGetStockQuotes(_ context.Context, arguments string) string {
	var args struct {
		Symbols string `json:"symbols"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return `{"error": "invalid arguments"}`
	}

	symbols := strings.Split(args.Symbols, ",")
	for i := range symbols {
		symbols[i] = strings.TrimSpace(symbols[i])
	}

	quotes, err := a.schwab.GetQuotes(symbols)
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}

	result, _ := json.Marshal(quotes)
	return string(result)
}

func (a *Analyzer) execGetOptionChain(_ context.Context, arguments string) string {
	var args struct {
		Symbol       string  `json:"symbol"`
		ContractType string  `json:"contract_type"`
		FromDate     string  `json:"from_date"`
		ToDate       string  `json:"to_date"`
		Strike       float64 `json:"strike"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return `{"error": "invalid arguments"}`
	}

	if args.FromDate == "" {
		args.FromDate = time.Now().Format("2006-01-02")
	}
	if args.ToDate == "" {
		args.ToDate = time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	}

	chain, err := a.schwab.GetOptionChain(args.Symbol, args.ContractType, args.FromDate, args.ToDate, args.Strike)
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}

	result, _ := json.Marshal(chain)
	return string(result)
}

// stripMarkdownCodeBlock removes ```json fences if a model wraps its JSON.
func stripMarkdownCodeBlock(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}
