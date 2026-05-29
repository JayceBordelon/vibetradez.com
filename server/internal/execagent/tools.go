package execagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/trades"
)

/*
Maximum allowed per-share premium on any order this agent places.
Mirrors exec.MaxContractPremium so the cap is visible at the tool
layer; the underlying exec.Service re-validates so this constant can't
silently drift past the broker-side cap. $10/share × 100 shares =
$1,000 of capital exposure on a single contract. Hard ceiling.
*/
const MaxToolPremiumPerShare = 10.00

/*
Maximum number of place_options_order calls the tool layer accepts in
a single agent run. Equal to the candidate basket size — the agent
should never fire more orders than there are picks. Enforced even if
the model tries to spam the tool. Each call may request quantity > 1
(duplicate contracts) subject to MaxDailyExposure below.
*/
const MaxOrdersPerRun = 3

/*
MaxDailyExposure is the total dollar exposure the agent may commit
across ALL orders in a single run. Exposure for one order is
limit_price × 100 × quantity. The agent is encouraged to size up
(buy duplicate contracts) on high-conviction picks until it approaches
this budget, but the cumulative spend across every order it places is
hard-capped here — this is the master money ceiling for the day,
replacing the old per-contract-only model whose worst case was 3 ×
$1,000 = $3,000. With this budget the worst case is $1,000/day.

The exec.Service re-validates each single order's total cost against
exec.MaxOrderCost so a buggy tool-layer change can't widen a single
order past the day budget; the cumulative-across-orders accounting
lives here in the dispatcher because only the dispatcher sees the
whole run.
*/
const MaxDailyExposure = 1000.00

/*
ChainReader is the read-side dependency the tool layer needs. The
production implementation is *schwab.Client; tests use a stub. The
interface is intentionally narrow — only the two methods the tools
actually expose to Claude.
*/
type ChainReader interface {
	GetQuotes(symbols []string) (map[string]schwab.StockQuote, error)
	GetOptionChain(symbol, contractType, fromDate, toDate string, strike float64) (*schwab.OptionChain, error)
}

/*
ExecutionService is the write-side dependency. The production
implementation is *exec.Service; tests stub it. Method shapes mirror
the three Agent-* entry points exec.Service exposes — those names
deliberately include "Agent" so it's grep-clear which entrypoints
are part of this isolated tool surface.
*/
type ExecutionService interface {
	PlaceBuyToOpenAgent(ctx context.Context, t *trades.Trade, strike float64, expiration string, dte int, limitPrice float64, quantity int) (orderID string, execID int, err error)
	InsertSkippedExecutionAgent(tradeID int, reason string) (int, error)
	AvailableFundsAgent(ctx context.Context) (float64, error)
}

/*
ToolDispatcher holds the agent-run-scoped state (candidate allowlist,
orders-placed counter, per-rank dedup) and dispatches tool calls
issued by Claude to the underlying read/write services. One
ToolDispatcher per agent run; not safe for concurrent runs (one mu
serializes write side).

The dispatcher is the SECURITY BOUNDARY between the model and real
money. Every gate that exists "because the model might do something
weird" lives here. Adding new tools or relaxing gates without thinking
about blast radius is how this becomes a stage-1 incident.
*/
type ToolDispatcher struct {
	chain ChainReader
	exec  ExecutionService

	// Candidates the agent is allowed to act on. Keyed by rank for
	// O(1) lookup during tool dispatch. Symbol-allowlist + rank-dedup
	// both gate off this map.
	byRank map[int]*trades.Trade

	// Per-run state: written-through under mu.
	mu              sync.Mutex
	placedByRank    map[int]placedOrder // rank → details of the buy we already placed
	skippedByRank   map[int]string      // rank → reason we recorded a skip for
	ordersPlaced    int                 // count of successful place_options_order calls
	spentExposure   float64             // cumulative limit_price×100×qty committed this run; gated by MaxDailyExposure
	decisionsByRank map[int]Decision    // final decisions seeded as tools succeed (so the agent's reconciled JSON can be cross-checked)
}

type placedOrder struct {
	OrderID    string
	Strike     float64
	Expiration string
	LimitPrice float64
	Quantity   int
	ExecID     int
}

func NewToolDispatcher(chain ChainReader, exec ExecutionService, candidates []Candidate) *ToolDispatcher {
	byRank := make(map[int]*trades.Trade, len(candidates))
	for i := range candidates {
		t := &candidates[i].Trade
		if t.Rank < 1 {
			continue
		}
		byRank[t.Rank] = t
	}
	return &ToolDispatcher{
		chain:           chain,
		exec:            exec,
		byRank:          byRank,
		placedByRank:    make(map[int]placedOrder, MaxOrdersPerRun),
		skippedByRank:   make(map[int]string, MaxOrdersPerRun),
		decisionsByRank: make(map[int]Decision, MaxOrdersPerRun),
	}
}

/*
ToolDefinitions returns the anthropic.ToolUnionParam slice for the
agent's Messages.New invocation. Pure construction — no closure over
dispatcher state. Schemas are restrictive on purpose: every parameter
is required, every type is concrete, no free-form maps. This is what
the model sees as the tool surface.
*/
func ToolDefinitions() []anthropic.ToolUnionParam {
	return []anthropic.ToolUnionParam{
		{OfWebSearchTool20260209: &anthropic.WebSearchTool20260209Param{
			MaxUses: anthropic.Int(6),
		}},
		{OfTool: &anthropic.ToolParam{
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
		{OfTool: &anthropic.ToolParam{
			Name:        "get_option_chain",
			Description: anthropic.String("Get the LIVE option chain from Schwab for a candidate symbol. Returns bid/ask/last/mark, greeks (delta, gamma, theta, vega), open interest, and volume. Use this to pick a real strike + expiration to trade against."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"symbol":        map[string]any{"type": "string", "description": "Stock ticker (must be one of the 3 candidate symbols)"},
					"contract_type": map[string]any{"type": "string", "enum": []string{"CALL", "PUT", "ALL"}, "description": "Filter by contract type. Default ALL."},
					"from_date":     map[string]any{"type": "string", "description": "Start date for expiration range (YYYY-MM-DD). Defaults to today."},
					"to_date":       map[string]any{"type": "string", "description": "End date for expiration range (YYYY-MM-DD). Defaults to 14 days out."},
					"strike":        map[string]any{"type": "number", "description": "Optional: filter to a specific strike price."},
				},
				Required: []string{"symbol"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "get_account_funds",
			Description: anthropic.String("Returns the available USD cash in the Schwab trading account. Informational: your real constraint today is the $1000 daily exposure budget enforced by place_options_order, not account cash, but you can check funds if you want to confirm settlement headroom before sizing up."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{},
				Required:   []string{},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name: "place_options_order",
			Description: anthropic.String("Submit a BUY_TO_OPEN LIMIT order for a single contract spec, with a quantity you choose. Fire-and-forget — returns the Schwab order id immediately, fills land via the per-minute reconcile cron. " +
				"Quantity lets you buy DUPLICATE contracts of the same strike/expiration on a high-conviction pick: order exposure = limit_price × 100 × quantity. " +
				"Hard rules enforced by the tool: (1) symbol must match one of the 3 candidates, (2) one order per rank, (3) maximum 3 orders per run, (4) limit_price must be in (0, $10.00] per share, (5) quantity >= 1, (6) the day's cumulative exposure across all orders must stay <= $1000.00, (7) expiration must be YYYY-MM-DD and in the future. " +
				"Tool returns a clear error string on refusal (including how much budget is left) — copy that error verbatim into skip_reason if it forces a skip."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"rank":          map[string]any{"type": "integer", "description": "Candidate rank 1..3 — must match the candidate this order is for."},
					"symbol":        map[string]any{"type": "string", "description": "Ticker (must match the candidate at this rank)."},
					"contract_type": map[string]any{"type": "string", "enum": []string{"CALL", "PUT"}, "description": "CALL or PUT (must match the candidate at this rank)."},
					"strike":        map[string]any{"type": "number", "description": "Strike price (must exist on the live chain)."},
					"expiration":    map[string]any{"type": "string", "description": "Expiration date YYYY-MM-DD (must exist on the live chain)."},
					"limit_price":   map[string]any{"type": "number", "description": "Per-share LIMIT price in USD. Must be > 0 and <= 10.00. Suggest ~1.10× the live ask."},
					"quantity":      map[string]any{"type": "integer", "description": "Number of contracts to buy at this spec (>= 1). Use >1 to double/triple up on a high-conviction pick. Order exposure = limit_price × 100 × quantity and must fit in the $1000 daily budget remaining. Defaults to 1 if omitted."},
				},
				Required: []string{"rank", "symbol", "contract_type", "strike", "expiration", "limit_price"},
			},
		}},
	}
}

/*
Dispatch is the single entry point for executing a tool call Claude
issued. Returns the tool's stringified result (which the agent loop
threads back into the conversation as a tool_result block). Tool
errors are returned as a stringified JSON {"error": "..."} payload —
the model sees the refusal and can either retry with corrected args
or fall back to a skip.

Dispatch never panics. Every tool name dispatches through this single
switch; an unknown tool returns an error string rather than panicking
the agent loop.
*/
func (d *ToolDispatcher) Dispatch(ctx context.Context, name string, input json.RawMessage) string {
	switch name {
	case "get_stock_quotes":
		return d.dispatchGetStockQuotes(input)
	case "get_option_chain":
		return d.dispatchGetOptionChain(input)
	case "get_account_funds":
		return d.dispatchGetAccountFunds(ctx)
	case "place_options_order":
		return d.dispatchPlaceOptionsOrder(ctx, input)
	default:
		return jsonError(fmt.Sprintf("unknown tool: %s", name))
	}
}

func (d *ToolDispatcher) dispatchGetStockQuotes(input json.RawMessage) string {
	var args struct {
		Symbols string `json:"symbols"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("invalid arguments to get_stock_quotes")
	}
	symbols := splitCSV(args.Symbols)
	if len(symbols) == 0 {
		return jsonError("get_stock_quotes: symbols is empty")
	}
	quotes, err := d.chain.GetQuotes(symbols)
	if err != nil {
		return jsonError(fmt.Sprintf("get_stock_quotes: %v", err))
	}
	out, _ := json.Marshal(quotes)
	return string(out)
}

func (d *ToolDispatcher) dispatchGetOptionChain(input json.RawMessage) string {
	var args struct {
		Symbol       string  `json:"symbol"`
		ContractType string  `json:"contract_type"`
		FromDate     string  `json:"from_date"`
		ToDate       string  `json:"to_date"`
		Strike       float64 `json:"strike"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("invalid arguments to get_option_chain")
	}
	args.Symbol = strings.ToUpper(strings.TrimSpace(args.Symbol))
	if !d.symbolOnAllowlist(args.Symbol) {
		return jsonError(fmt.Sprintf("get_option_chain: symbol %q is not a candidate (allowed: %s)", args.Symbol, d.allowlistString()))
	}
	if args.FromDate == "" {
		args.FromDate = time.Now().Format("2006-01-02")
	}
	if args.ToDate == "" {
		args.ToDate = time.Now().AddDate(0, 0, 14).Format("2006-01-02")
	}
	chain, err := d.chain.GetOptionChain(args.Symbol, args.ContractType, args.FromDate, args.ToDate, args.Strike)
	if err != nil {
		return jsonError(fmt.Sprintf("get_option_chain: %v", err))
	}
	out, _ := json.Marshal(chain)
	return string(out)
}

func (d *ToolDispatcher) dispatchGetAccountFunds(ctx context.Context) string {
	usd, err := d.exec.AvailableFundsAgent(ctx)
	if err != nil {
		return jsonError(fmt.Sprintf("get_account_funds: %v", err))
	}
	out, _ := json.Marshal(map[string]any{"available_usd": usd, "currency": "USD"})
	return string(out)
}

/*
placeOptionsOrderArgs mirrors the place_options_order tool schema.
Defined outside the dispatch function so the validation rules can be
exercised directly by tools_test.go.
*/
type placeOptionsOrderArgs struct {
	Rank         int     `json:"rank"`
	Symbol       string  `json:"symbol"`
	ContractType string  `json:"contract_type"`
	Strike       float64 `json:"strike"`
	Expiration   string  `json:"expiration"`
	LimitPrice   float64 `json:"limit_price"`
	Quantity     int     `json:"quantity"`
}

func (d *ToolDispatcher) dispatchPlaceOptionsOrder(ctx context.Context, input json.RawMessage) string {
	var args placeOptionsOrderArgs
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("place_options_order: invalid arguments")
	}
	args.Symbol = strings.ToUpper(strings.TrimSpace(args.Symbol))
	args.ContractType = strings.ToUpper(strings.TrimSpace(args.ContractType))
	// Omitted quantity means a single contract — the model can leave it
	// off when it isn't sizing up.
	if args.Quantity == 0 {
		args.Quantity = 1
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Validate the args against the candidate allowlist + hard caps
	// BEFORE touching the broker. Every refusal here is a single-line
	// error the model sees and can react to.
	if err := d.validatePlaceArgsLocked(&args); err != nil {
		return jsonError(err.Error())
	}

	candidate := d.byRank[args.Rank] // safe — validatePlaceArgsLocked checked

	// Per-run cap. validatePlaceArgsLocked already checked
	// placedByRank[rank] presence; this guards against running out of
	// budget across all ranks combined.
	if d.ordersPlaced >= MaxOrdersPerRun {
		return jsonError(fmt.Sprintf("place_options_order: refused, already placed %d orders this run (max %d)", d.ordersPlaced, MaxOrdersPerRun))
	}

	// Cumulative daily exposure cap. Exposure for this order is
	// limit_price × 100 shares × quantity. The check is here (not in
	// validatePlaceArgsLocked) because it reads run-scoped spentExposure
	// and the refusal message reports the remaining budget so the model
	// can size a smaller quantity and retry.
	orderExposure := args.LimitPrice * 100 * float64(args.Quantity)
	remaining := MaxDailyExposure - d.spentExposure
	if orderExposure > remaining+1e-6 {
		return jsonError(fmt.Sprintf(
			"place_options_order: refused, order exposure $%.2f (%d × $%.2f × 100) exceeds remaining daily budget $%.2f (cap $%.2f, already committed $%.2f). Reduce quantity or limit_price.",
			orderExposure, args.Quantity, args.LimitPrice, remaining, MaxDailyExposure, d.spentExposure))
	}

	/*
		Compute DTE from the chosen expiration. The execagent doesn't
		ask Claude for DTE explicitly — it's derived. ResolveTradeContract
		on the underlying exec.Service writes it to the trade row so
		downstream consumers (dashboard, EOD analysis) see a consistent
		value.
	*/
	dte := daysToExpiration(args.Expiration)
	if dte < 0 {
		return jsonError(fmt.Sprintf("place_options_order: expiration %q is in the past", args.Expiration))
	}

	orderID, execID, err := d.exec.PlaceBuyToOpenAgent(ctx, candidate, args.Strike, args.Expiration, dte, args.LimitPrice, args.Quantity)
	if err != nil {
		// Tool returns the error string verbatim. Claude copies this
		// into skip_reason when it can't recover (typical case: broker
		// rejected the order at submit time).
		log.Printf("execagent: place_options_order failed for rank=%d %s: %v", args.Rank, args.Symbol, err)
		return jsonError(fmt.Sprintf("place_options_order: %v", err))
	}

	// Success — record state for dedup + budget accounting + seed the
	// per-rank decision so reconciliation against the agent's final JSON
	// can detect drift.
	d.placedByRank[args.Rank] = placedOrder{
		OrderID:    orderID,
		Strike:     args.Strike,
		Expiration: args.Expiration,
		LimitPrice: args.LimitPrice,
		Quantity:   args.Quantity,
		ExecID:     execID,
	}
	d.decisionsByRank[args.Rank] = Decision{
		Rank:        args.Rank,
		Symbol:      args.Symbol,
		Action:      ActionBuy,
		OrderID:     orderID,
		Strike:      args.Strike,
		Expiration:  args.Expiration,
		LimitPrice:  args.LimitPrice,
		Quantity:    args.Quantity,
		ExecutionID: execID,
	}
	d.ordersPlaced++
	d.spentExposure += orderExposure

	out, _ := json.Marshal(map[string]any{
		"ok":               true,
		"order_id":         orderID,
		"execution_id":     execID,
		"rank":             args.Rank,
		"symbol":           args.Symbol,
		"strike":           args.Strike,
		"expiration":       args.Expiration,
		"limit_price":      args.LimitPrice,
		"quantity":         args.Quantity,
		"contract_type":    args.ContractType,
		"order_exposure":   orderExposure,
		"orders_placed":    d.ordersPlaced,
		"orders_left":      MaxOrdersPerRun - d.ordersPlaced,
		"budget_spent":     d.spentExposure,
		"budget_remaining": MaxDailyExposure - d.spentExposure,
	})
	return string(out)
}

/*
validatePlaceArgsLocked checks all the hard-cap rules. Mutates nothing
EXCEPT it requires the caller to already hold d.mu (it reads
placedByRank). Returns the first failed rule as an error, or nil on
success.

The order of checks is intentional — argument-shape errors first
(Claude can correct), then allowlist / dedup checks (Claude can pivot
to another rank), then numeric bound checks (last because they require
parsing).
*/
func (d *ToolDispatcher) validatePlaceArgsLocked(args *placeOptionsOrderArgs) error {
	if args.Rank < 1 || args.Rank > MaxOrdersPerRun {
		return fmt.Errorf("place_options_order: rank %d outside [1, %d]", args.Rank, MaxOrdersPerRun)
	}
	candidate, ok := d.byRank[args.Rank]
	if !ok {
		return fmt.Errorf("place_options_order: no candidate at rank %d (allowed ranks: %s)", args.Rank, d.rankString())
	}
	if args.Symbol == "" {
		return errors.New("place_options_order: symbol required")
	}
	candidateSymbol := strings.ToUpper(strings.TrimSpace(candidate.Symbol))
	if args.Symbol != candidateSymbol {
		return fmt.Errorf("place_options_order: symbol %q does not match candidate at rank %d (%s)", args.Symbol, args.Rank, candidateSymbol)
	}
	candidateContractType := strings.ToUpper(strings.TrimSpace(candidate.ContractType))
	if args.ContractType != candidateContractType {
		return fmt.Errorf("place_options_order: contract_type %q does not match candidate at rank %d (%s)", args.ContractType, args.Rank, candidateContractType)
	}
	if args.Strike <= 0 {
		return fmt.Errorf("place_options_order: strike %.2f must be > 0", args.Strike)
	}
	if args.Expiration == "" {
		return errors.New("place_options_order: expiration required (YYYY-MM-DD)")
	}
	if _, err := time.Parse("2006-01-02", args.Expiration); err != nil {
		return fmt.Errorf("place_options_order: expiration %q is not YYYY-MM-DD", args.Expiration)
	}
	if args.LimitPrice <= 0 {
		return fmt.Errorf("place_options_order: limit_price %.2f must be > 0", args.LimitPrice)
	}
	if args.LimitPrice > MaxToolPremiumPerShare {
		return fmt.Errorf("place_options_order: limit_price %.2f exceeds per-share cap %.2f", args.LimitPrice, MaxToolPremiumPerShare)
	}
	if args.Quantity < 1 {
		return fmt.Errorf("place_options_order: quantity %d must be >= 1", args.Quantity)
	}
	if _, already := d.placedByRank[args.Rank]; already {
		return fmt.Errorf("place_options_order: already placed an order for rank %d this run", args.Rank)
	}
	return nil
}

func (d *ToolDispatcher) symbolOnAllowlist(symbol string) bool {
	want := strings.ToUpper(strings.TrimSpace(symbol))
	for _, t := range d.byRank {
		if strings.ToUpper(strings.TrimSpace(t.Symbol)) == want {
			return true
		}
	}
	return false
}

func (d *ToolDispatcher) allowlistString() string {
	parts := make([]string, 0, len(d.byRank))
	for _, t := range d.byRank {
		parts = append(parts, t.Symbol)
	}
	return strings.Join(parts, ", ")
}

func (d *ToolDispatcher) rankString() string {
	parts := make([]string, 0, len(d.byRank))
	for r := range d.byRank {
		parts = append(parts, fmt.Sprintf("%d", r))
	}
	return strings.Join(parts, ", ")
}

/*
PlacedDecisions returns a deep copy of the per-rank decisions seeded
by successful place_options_order calls. Used by the agent runner to
reconcile against Claude's final JSON output — buys persisted via the
tool layer are the source of truth, the JSON is just for skip reasons.
*/
func (d *ToolDispatcher) PlacedDecisions() map[int]Decision {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[int]Decision, len(d.decisionsByRank))
	for k, v := range d.decisionsByRank {
		out[k] = v
	}
	return out
}

/*
RecordSkip persists a synthetic 'skipped' execution row for a rank
the agent chose not to trade. Returns the executions.id row id or an
error. Caller passes the reason verbatim from the agent's final JSON.

Idempotent per rank: repeated calls for the same rank are a no-op
returning the previously-recorded execution id (the agent shouldn't
double-skip, but a JSON quirk shouldn't double-write).
*/
func (d *ToolDispatcher) RecordSkip(rank int, reason string) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if existing, ok := d.skippedByRank[rank]; ok {
		_ = existing
		// already recorded; find the execution id in decisionsByRank
		if dec, ok2 := d.decisionsByRank[rank]; ok2 {
			return dec.SkipExecID, nil
		}
		return 0, nil
	}
	t, ok := d.byRank[rank]
	if !ok {
		return 0, fmt.Errorf("RecordSkip: no candidate at rank %d", rank)
	}
	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		cleanReason = "(no reason provided)"
	}
	id, err := d.exec.InsertSkippedExecutionAgent(t.ID, cleanReason)
	if err != nil {
		return 0, fmt.Errorf("RecordSkip: persist skip for rank %d: %w", rank, err)
	}
	d.skippedByRank[rank] = cleanReason
	d.decisionsByRank[rank] = Decision{
		Rank:       rank,
		Symbol:     t.Symbol,
		Action:     ActionSkip,
		SkipReason: cleanReason,
		SkipExecID: id,
	}
	return id, nil
}

/*
splitCSV is permissive: splits on commas, trims whitespace, drops
empty entries. Tolerates trailing commas and double-commas without
returning empty strings to the broker.
*/
func splitCSV(s string) []string {
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		t := strings.TrimSpace(r)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

/*
daysToExpiration returns the calendar-day delta between today (UTC)
and the given expiration (YYYY-MM-DD). Negative when the expiration is
in the past. Always anchors at midnight UTC for both dates so DST
transitions don't perturb the count.
*/
func daysToExpiration(expiration string) int {
	exp, err := time.Parse("2006-01-02", expiration)
	if err != nil {
		return -1
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	exp = exp.UTC().Truncate(24 * time.Hour)
	return int(exp.Sub(today).Hours() / 24)
}

func jsonError(msg string) string {
	out, _ := json.Marshal(map[string]string{"error": msg})
	return string(out)
}
