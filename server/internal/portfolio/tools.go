package portfolio

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"

	"vibetradez.com/internal/schwab"
)

/*
PortfolioReader is the read-side dependency the tool layer needs. The
production implementation (wired in cmd/scanner/main.go, task 4) binds
these to the real Schwab client; tests use a fake. The interface is
intentionally narrow — only what the tools expose to Claude plus the
session snapshot and the per-buy liquidity gather.
*/
type PortfolioReader interface {
	// Snapshot builds the session-start account state: positions, settled
	// and unsettled cash, equity, high-water mark, and the computed
	// deployment budget for the session.
	Snapshot(ctx context.Context) (Snapshot, error)
	// GetQuotes returns live Schwab equity quotes for the symbols.
	GetQuotes(symbols []string) (map[string]schwab.StockQuote, error)
	// GetOptionChain returns the live Schwab option chain for a symbol.
	GetOptionChain(symbol, contractType, fromDate, toDate string, strike float64) (*schwab.OptionChain, error)
	// Liquidity gathers the tradability facts (underlying price, market
	// cap, and for options the OI/volume/spread) for a proposed buy so the
	// liquidity floor is enforced on live market data, not the model's say-so.
	Liquidity(ctx context.Context, m Move) (LiquidityCtx, error)
	// GetDailyPriceHistory returns ~1y of daily candles for trend / 52-week
	// range / volatility context.
	GetDailyPriceHistory(symbol string) (*schwab.PriceHistory, error)
	// GetFundamentals returns valuation + earnings + dividend context.
	GetFundamentals(symbol string) (*schwab.Fundamentals, error)
	// RecentDecisions / RecentStances surface the agent's own recent history
	// so it keeps a coherent thesis across days.
	RecentDecisions(limit int) ([]HistoryEntry, error)
	RecentStances(limit int) ([]StanceEntry, error)
	// OrderStatus reads the live broker state of a previously placed order
	// (working / filled / canceled, fill price). Reading-side: it lets the
	// agent see whether a resting close actually filled before re-acting.
	OrderStatus(ctx context.Context, orderID string) (OrderStatus, error)
	// PriorSession returns the most recent prior session's synopsis + action
	// items, so the agent starts the day where it left off. ok is false when
	// there is no prior session on record.
	PriorSession() (synopsis, actionItems string, ok bool, err error)
}

/*
OrderStatus is the trimmed broker order state surfaced to the agent via
get_order_status. Quantity/FilledQuantity are share or contract counts.
*/
type OrderStatus struct {
	OrderID        string  `json:"order_id"`
	Status         string  `json:"status"` // verbatim broker status (WORKING, FILLED, CANCELED, ...)
	Filled         bool    `json:"filled"`
	Working        bool    `json:"working"`
	Terminal       bool    `json:"terminal"`
	Quantity       int     `json:"quantity"`
	FilledQuantity int     `json:"filled_quantity"`
	FillPrice      float64 `json:"fill_price"`
	LimitPrice     float64 `json:"limit_price"`
}

/*
PortfolioExecutor is the write-side dependency: the broker entry points
the tool layer commits decisions through. The production implementation
wraps exec.Service (task 2 adds the equity/sell entry points). Each method
mirrors the boundary exec already uses for options: it persists an order
row and returns the broker order id plus the internal execution id, and it
re-validates the hard caps on the broker side so a buggy tool-layer change
cannot widen them.
*/
type PortfolioExecutor interface {
	BuyEquity(ctx context.Context, symbol string, quantity float64, limitPrice float64) (orderID string, execID int, err error)
	SellEquity(ctx context.Context, symbol string, quantity float64, limitPrice float64) (orderID string, execID int, err error)
	BuyOption(ctx context.Context, occSymbol, underlying, contractType string, strike float64, expiration string, contracts int, limitPrice float64) (orderID string, execID int, err error)
	SellOption(ctx context.Context, occSymbol string, contracts int, limitPrice float64) (orderID string, execID int, err error)
	// CancelOrder cancels a resting order by broker order id (best-effort: a
	// filled order can't be canceled). Execution-side, used to clear a stale
	// closing order before re-pricing it.
	CancelOrder(ctx context.Context, orderID string) error
}

/*
ToolDispatcher holds the agent-run-scoped state and dispatches tool calls
to the read/write services. One per Run; not safe for concurrent runs (mu
serializes the write side).

It is the SECURITY BOUNDARY between the model and real money. Every cap
gate lives here (delegating the math to caps.go). The dispatcher keeps a
mutable WORKING snapshot that starts as the session snapshot and is updated
after every committed move, so cumulative exposure across multiple buys in
one session is enforced correctly (the second buy sees the first buy's
exposure). Adding tools or relaxing gates without thinking about blast
radius is how this becomes a real-money incident.
*/
type ToolDispatcher struct {
	reader PortfolioReader
	exec   PortfolioExecutor
	caps   Caps

	mu          sync.Mutex
	work        Snapshot // mutable working snapshot; updated as moves commit
	decisions   []Decision
	summary     string // synopsis of the day, from write_summary
	actionItems string // plan for the next session, from write_summary
}

func NewToolDispatcher(reader PortfolioReader, exec PortfolioExecutor, caps Caps, snap Snapshot) *ToolDispatcher {
	return &ToolDispatcher{
		reader: reader,
		exec:   exec,
		caps:   caps,
		work:   snap,
	}
}

/*
ToolDefinitions returns the anthropic tool surface for the portfolio
agent. Schemas are restrictive on purpose: every parameter is required,
every type concrete, no free-form maps. This is what the model sees.
*/
func ToolDefinitions() []anthropic.ToolUnionParam {
	num := func(desc string) map[string]any { return map[string]any{"type": "number", "description": desc} }
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	return []anthropic.ToolUnionParam{
		{OfWebSearchTool20260209: &anthropic.WebSearchTool20260209Param{MaxUses: anthropic.Int(8)}},
		// web_fetch lets the model open and read a specific page (a news
		// article, a company IR / filing page, a finance site like Yahoo
		// Finance / Reuters / CNBC / Bloomberg / Finviz / SEC EDGAR) rather
		// than only seeing search snippets.
		{OfWebFetchTool20260209: &anthropic.WebFetchTool20260209Param{MaxUses: anthropic.Int(8), MaxContentTokens: anthropic.Int(40000)}},
		{OfTool: &anthropic.ToolParam{
			Name:        "get_portfolio",
			Description: anthropic.String("Returns the live account state: current positions (equity + options with cost basis, mark value, and unrealized P&L), settled and unsettled cash, total equity, the high-water mark, and the dollar budget remaining for NEW buys this session."),
			InputSchema: anthropic.ToolInputSchemaParam{Properties: map[string]any{}, Required: []string{}},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "get_stock_quotes",
			Description: anthropic.String("Live Schwab equity quotes. Comma-separated symbols. Returns last/bid/ask/mark/open/high/low/volume and day change."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{"symbols": str("Comma-separated tickers, e.g. 'AAPL,MSFT,TSLA'")},
				Required:   []string{"symbols"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "get_option_chain",
			Description: anthropic.String("Live Schwab option chain for a symbol. Returns bid/ask/last/mark, greeks, open interest, and volume. Use it to pick a real strike + expiration and to read the OCC option symbol for buy_option."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"symbol":        str("Underlying ticker."),
					"contract_type": map[string]any{"type": "string", "enum": []string{"CALL", "PUT", "ALL"}, "description": "Filter by type. Default ALL."},
					"from_date":     str("Expiration range start (YYYY-MM-DD). Defaults to today."),
					"to_date":       str("Expiration range end (YYYY-MM-DD). Defaults to ~60 days out."),
					"strike":        num("Optional: filter to a specific strike."),
				},
				Required: []string{"symbol"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "get_price_history",
			Description: anthropic.String("Daily price history and trend context for a symbol: last close, 20/50/200-day moving averages, the 52-week high/low and how far price sits from each, 1-month and 3-month returns, and recent 20-day volatility. Use it to judge trend and whether a name is extended or beaten down before sizing."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{"symbol": str("Equity ticker.")},
				Required:   []string{"symbol"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "get_fundamentals",
			Description: anthropic.String("Valuation and catalyst context for a symbol: market cap, P/E, trailing EPS, 52-week range, beta, dividend yield/amount, average volume, and the next earnings date. Check earnings dates before holding into a print."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{"symbol": str("Equity ticker.")},
				Required:   []string{"symbol"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "get_cap_headroom",
			Description: anthropic.String("How much room you have left under every hard cap right now: per-order limit, per-name capacity (with current exposure for each name you hold), options-sleeve room, remaining daily deployment budget, and whether the drawdown breaker has halted new buys. Use it to size precisely instead of getting orders rejected."),
			InputSchema: anthropic.ToolInputSchemaParam{Properties: map[string]any{}, Required: []string{}},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "get_recent_decisions",
			Description: anthropic.String("Your own recent moves and daily stance notes across prior sessions, plus the synopsis and action items you wrote at the end of your last session (your plan for today), so you can keep a coherent thesis instead of starting from scratch. Returns the latest moves, the last several stances, and prior_synopsis + prior_action_items."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{"limit": map[string]any{"type": "integer", "description": "Max moves to return (default 20)."}},
				Required:   []string{},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "get_order_status",
			Description: anthropic.String("Reading tool. Returns the live broker state of an order you previously placed (by order id, e.g. from get_recent_decisions): working / filled / canceled, filled quantity, and fill price. Use it to confirm whether a resting close actually executed before you re-act."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{"order_id": str("The broker order id returned when the order was placed (also shown in get_recent_decisions).")},
				Required:   []string{"order_id"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "buy_equity",
			Description: anthropic.String("Submit a BUY LIMIT order for shares of an equity. Notional (quantity × limit_price) is checked against the per-order, concentration, settled-cash, daily-deployment, and drawdown caps before anything reaches the broker. Returns the order id on success or a clear refusal string."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"symbol":      str("Equity ticker."),
					"quantity":    num("Number of shares (whole shares)."),
					"limit_price": num("Per-share LIMIT price in USD, > 0."),
					"rationale":   str("One to three sentences a human will read explaining this buy."),
				},
				Required: []string{"symbol", "quantity", "limit_price", "rationale"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "sell_equity",
			Description: anthropic.String("Submit a SELL LIMIT order for shares you currently hold. Refused if you do not hold the position or try to sell more than you own. De-risking is always allowed."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"symbol":      str("Equity ticker you hold."),
					"quantity":    num("Number of shares to sell, <= shares held."),
					"limit_price": num("Per-share LIMIT price in USD, > 0."),
					"rationale":   str("One to three sentences explaining this sell."),
				},
				Required: []string{"symbol", "quantity", "limit_price", "rationale"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "buy_option",
			Description: anthropic.String("Submit a BUY_TO_OPEN LIMIT for a long single-leg option. Notional (contracts × limit_price × 100) is checked against the per-order, concentration (counts toward the underlying alongside any equity), options-sleeve, settled-cash, daily-deployment, drawdown, and liquidity caps. occ_symbol and underlying both come from get_option_chain."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"occ_symbol":    str("The 21-char OCC option symbol from get_option_chain."),
					"underlying":    str("The underlying equity ticker (for concentration accounting)."),
					"contract_type": map[string]any{"type": "string", "enum": []string{"CALL", "PUT"}, "description": "CALL or PUT."},
					"strike":        num("Strike price (must exist on the live chain)."),
					"expiration":    str("Expiration date YYYY-MM-DD (must exist on the live chain)."),
					"contracts":     map[string]any{"type": "integer", "description": "Number of contracts (whole)."},
					"limit_price":   num("Per-share LIMIT premium in USD, > 0 (×100 per contract)."),
					"rationale":     str("One to three sentences explaining this buy."),
				},
				Required: []string{"occ_symbol", "underlying", "contract_type", "strike", "expiration", "contracts", "limit_price", "rationale"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "sell_option",
			Description: anthropic.String("Submit a SELL_TO_CLOSE LIMIT for an option you hold. Refused if you do not hold it or oversell. De-risking is always allowed."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"occ_symbol":  str("The OCC option symbol you hold."),
					"contracts":   map[string]any{"type": "integer", "description": "Number of contracts to sell, <= held."},
					"limit_price": num("Per-share LIMIT premium in USD, > 0."),
					"rationale":   str("One to three sentences explaining this sell."),
				},
				Required: []string{"occ_symbol", "contracts", "limit_price", "rationale"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "cancel_order",
			Description: anthropic.String("Execution tool. Cancels a resting (working) order by its broker order id. Best-effort: a filled or already-terminal order cannot be canceled. Use it to clear a stale closing order so you can re-submit the exit at a price that will fill."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"order_id":  str("The broker order id to cancel (from get_recent_decisions or the place response)."),
					"rationale": str("One to three sentences explaining why you are canceling."),
				},
				Required: []string{"order_id", "rationale"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "hold",
			Description: anthropic.String("Record that you are CONTINUING to hold a position unchanged. Call it if and only if you already held this name as of the last trading day and are choosing to keep it as-is today. It is never required and never applies to a position you opened today; skip it entirely if there is nothing being continued."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"symbol":    str("The carried-over position you are continuing to hold unchanged (equity ticker or OCC option symbol)."),
					"rationale": str("Why you are continuing to hold this position unchanged today."),
				},
				Required: []string{"symbol", "rationale"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "write_summary",
			Description: anthropic.String("Documentation tool (it neither reads data nor moves money). Call it ONCE near the end of every session. It has two parts: synopsis (what happened today) and action_items (the plan for the NEXT trading session). You will read both back at the start of your next session, so write the action items as concrete things to do or check."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"synopsis":     str("Today's synopsis: what you saw in the book and the market, what you did and why, and what you left alone. A short paragraph or two, for a human reading the daily record."),
					"action_items": str("Action items for the next trading session (tomorrow, or after the weekend if today is Friday): orders to confirm filled, positions to watch, setups you are waiting on, and what would change your mind. Concrete and forward-looking."),
				},
				Required: []string{"synopsis", "action_items"},
			},
		}},
	}
}

/*
Dispatch is the single entry point for a tool call. Returns the tool's
stringified result, threaded back into the conversation as a tool_result.
Refusals come back as {"error": "..."} so the model can react. Dispatch
never panics; an unknown tool returns an error string.
*/
func (d *ToolDispatcher) Dispatch(ctx context.Context, name string, input json.RawMessage) string {
	switch name {
	case "get_portfolio":
		return d.dispatchGetPortfolio()
	case "get_stock_quotes":
		return d.dispatchGetStockQuotes(input)
	case "get_option_chain":
		return d.dispatchGetOptionChain(input)
	case "get_price_history":
		return d.dispatchGetPriceHistory(input)
	case "get_fundamentals":
		return d.dispatchGetFundamentals(input)
	case "get_cap_headroom":
		return d.dispatchGetCapHeadroom()
	case "get_recent_decisions":
		return d.dispatchGetRecentDecisions(input)
	case "get_order_status":
		return d.dispatchGetOrderStatus(ctx, input)
	case "cancel_order":
		return d.dispatchCancelOrder(ctx, input)
	case "buy_equity":
		return d.dispatchBuyEquity(ctx, input)
	case "sell_equity":
		return d.dispatchSellEquity(ctx, input)
	case "buy_option":
		return d.dispatchBuyOption(ctx, input)
	case "sell_option":
		return d.dispatchSellOption(ctx, input)
	case "hold":
		return d.dispatchHold(input)
	case "write_summary":
		return d.dispatchWriteSummary(input)
	default:
		return jsonError(fmt.Sprintf("unknown tool: %s", name))
	}
}

func (d *ToolDispatcher) dispatchGetPortfolio() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out, _ := json.Marshal(map[string]any{
		"equity":                  d.work.Equity,
		"settled_cash":            d.work.SettledCash,
		"unsettled_cash":          d.work.UnsettledCash,
		"high_water_mark":         d.work.HighWaterMark,
		"deployment_budget_total": d.work.DeploymentBudget,
		"deployment_budget_left":  d.work.DeploymentBudget - d.work.DeployedThisSession,
		"new_buys_halted":         d.caps.DrawdownHalted(d.work),
		"positions":               d.work.Positions,
	})
	return string(out)
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
	quotes, err := d.reader.GetQuotes(symbols)
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
	if args.Symbol == "" {
		return jsonError("get_option_chain: symbol required")
	}
	chain, err := d.reader.GetOptionChain(args.Symbol, args.ContractType, args.FromDate, args.ToDate, args.Strike)
	if err != nil {
		return jsonError(fmt.Sprintf("get_option_chain: %v", err))
	}
	out, _ := json.Marshal(chain)
	return string(out)
}

func (d *ToolDispatcher) dispatchGetPriceHistory(input json.RawMessage) string {
	var args struct {
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("invalid arguments to get_price_history")
	}
	sym := strings.ToUpper(strings.TrimSpace(args.Symbol))
	if sym == "" {
		return jsonError("get_price_history: symbol required")
	}
	ph, err := d.reader.GetDailyPriceHistory(sym)
	if err != nil {
		return jsonError(fmt.Sprintf("get_price_history: %v", err))
	}
	out, _ := json.Marshal(priceSummary(sym, ph))
	return string(out)
}

func (d *ToolDispatcher) dispatchGetFundamentals(input json.RawMessage) string {
	var args struct {
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("invalid arguments to get_fundamentals")
	}
	sym := strings.ToUpper(strings.TrimSpace(args.Symbol))
	if sym == "" {
		return jsonError("get_fundamentals: symbol required")
	}
	f, err := d.reader.GetFundamentals(sym)
	if err != nil {
		return jsonError(fmt.Sprintf("get_fundamentals: %v", err))
	}
	out, _ := json.Marshal(f)
	return string(out)
}

func (d *ToolDispatcher) dispatchGetCapHeadroom() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.work
	c := d.caps

	perNameCap := s.Equity * c.MaxPerUnderlyingPct
	exposures := map[string]float64{}
	for _, p := range s.Positions {
		u := strings.ToUpper(strings.TrimSpace(p.Underlying))
		exposures[u] += p.MarkValue
	}
	heldNames := make([]map[string]any, 0, len(exposures))
	for u, ex := range exposures {
		heldNames = append(heldNames, map[string]any{
			"underlying": u,
			"exposure":   round2(ex),
			"remaining":  round2(perNameCap - ex),
		})
	}
	sleeveCap := s.Equity * c.MaxOptionsSleevePct
	sleeveUsed := optionsSleeveValue(s)

	out, _ := json.Marshal(map[string]any{
		"equity":                   round2(s.Equity),
		"settled_cash":             round2(s.SettledCash),
		"new_buys_halted":          c.DrawdownHalted(s),
		"high_water_mark":          round2(s.HighWaterMark),
		"per_order_cap":            round2(s.Equity * c.MaxPerOrderPct),
		"per_name_cap":             round2(perNameCap),
		"options_sleeve_cap":       round2(sleeveCap),
		"options_sleeve_used":      round2(sleeveUsed),
		"options_sleeve_remaining": round2(sleeveCap - sleeveUsed),
		"deployment_budget_total":  round2(s.DeploymentBudget),
		"deployment_budget_left":   round2(s.DeploymentBudget - s.DeployedThisSession),
		"held_name_exposure":       heldNames,
	})
	return string(out)
}

func (d *ToolDispatcher) dispatchGetRecentDecisions(input json.RawMessage) string {
	var args struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal(input, &args)
	if args.Limit <= 0 {
		args.Limit = 20
	}
	decisions, err := d.reader.RecentDecisions(args.Limit)
	if err != nil {
		return jsonError(fmt.Sprintf("get_recent_decisions: %v", err))
	}
	stances, err := d.reader.RecentStances(7)
	if err != nil {
		return jsonError(fmt.Sprintf("get_recent_decisions: %v", err))
	}
	out := map[string]any{"decisions": decisions, "stances": stances}
	// The prior session's synopsis + action items: the plan you wrote for today.
	if synopsis, actionItems, ok, perr := d.reader.PriorSession(); perr == nil && ok {
		out["prior_synopsis"] = synopsis
		out["prior_action_items"] = actionItems
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func (d *ToolDispatcher) dispatchGetOrderStatus(ctx context.Context, input json.RawMessage) string {
	var args struct {
		OrderID string `json:"order_id"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("get_order_status: invalid arguments")
	}
	id := strings.TrimSpace(args.OrderID)
	if id == "" {
		return jsonError("get_order_status: order_id required")
	}
	st, err := d.reader.OrderStatus(ctx, id)
	if err != nil {
		return jsonError(fmt.Sprintf("get_order_status: %v", err))
	}
	out, _ := json.Marshal(st)
	return string(out)
}

/*
dispatchCancelOrder cancels a resting order at the broker. It does not touch
the working snapshot: that snapshot is the cap-accounting view of holdings,
not a live order book, and cancels target resting (often prior-session)
orders rather than committed positions.
*/
func (d *ToolDispatcher) dispatchCancelOrder(ctx context.Context, input json.RawMessage) string {
	var args struct {
		OrderID   string `json:"order_id"`
		Rationale string `json:"rationale"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("cancel_order: invalid arguments")
	}
	id := strings.TrimSpace(args.OrderID)
	if id == "" {
		return jsonError("cancel_order: order_id required")
	}
	if err := d.exec.CancelOrder(ctx, id); err != nil {
		log.Printf("portfolio: cancel_order failed for %s: %v", id, err)
		return jsonError(fmt.Sprintf("cancel_order: broker rejected: %v", err))
	}
	out, _ := json.Marshal(map[string]any{"ok": true, "action": "cancel_order", "order_id": id})
	return string(out)
}

/*
priceSummary condenses a year of daily candles into the trend signals the
model actually reasons over, instead of dumping ~250 raw bars into the
context. Candles arrive oldest-first from Schwab.
*/
func priceSummary(symbol string, ph *schwab.PriceHistory) map[string]any {
	c := ph.Candles
	n := len(c)
	if n == 0 {
		return map[string]any{"symbol": symbol, "error": "no candles"}
	}
	closes := make([]float64, n)
	hi, lo := c[0].High, c[0].Low
	for i, k := range c {
		closes[i] = k.Close
		if k.High > hi {
			hi = k.High
		}
		if k.Low < lo {
			lo = k.Low
		}
	}
	last := closes[n-1]
	sma := func(p int) float64 {
		if n < p {
			return 0
		}
		var s float64
		for _, v := range closes[n-p:] {
			s += v
		}
		return s / float64(p)
	}
	ret := func(back int) float64 {
		if n <= back || closes[n-1-back] == 0 {
			return 0
		}
		return (last/closes[n-1-back] - 1) * 100
	}
	return map[string]any{
		"symbol":                 symbol,
		"last_close":             round2(last),
		"sma_20":                 round2(sma(20)),
		"sma_50":                 round2(sma(50)),
		"sma_200":                round2(sma(200)),
		"high_52w":               round2(hi),
		"low_52w":                round2(lo),
		"pct_from_52w_high":      round2(pctOf(last-hi, hi)),
		"pct_from_52w_low":       round2(pctOf(last-lo, lo)),
		"return_1m_pct":          round2(ret(21)),
		"return_3m_pct":          round2(ret(63)),
		"annualized_vol_20d_pct": round2(annualizedVol(closes, 20)),
		"trading_days":           n,
	}
}

// annualizedVol is the stdev of the last `window` daily returns, scaled to
// an annualized percentage (sqrt(252)). Returns 0 when there isn't enough
// history.
func annualizedVol(closes []float64, window int) float64 {
	n := len(closes)
	if n < window+1 {
		return 0
	}
	rets := make([]float64, 0, window)
	for i := n - window; i < n; i++ {
		if closes[i-1] == 0 {
			continue
		}
		rets = append(rets, closes[i]/closes[i-1]-1)
	}
	if len(rets) < 2 {
		return 0
	}
	var mean float64
	for _, r := range rets {
		mean += r
	}
	mean /= float64(len(rets))
	var ss float64
	for _, r := range rets {
		dv := r - mean
		ss += dv * dv
	}
	variance := ss / float64(len(rets)-1)
	return math.Sqrt(variance) * math.Sqrt(252) * 100
}

func pctOf(num, den float64) float64 {
	if den == 0 {
		return 0
	}
	return num / den * 100
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// ── write tools ──

func (d *ToolDispatcher) dispatchBuyEquity(ctx context.Context, input json.RawMessage) string {
	var args struct {
		Symbol     string  `json:"symbol"`
		Quantity   float64 `json:"quantity"`
		LimitPrice float64 `json:"limit_price"`
		Rationale  string  `json:"rationale"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("buy_equity: invalid arguments")
	}
	args.Symbol = strings.ToUpper(strings.TrimSpace(args.Symbol))
	if args.Symbol == "" || args.Quantity <= 0 || args.LimitPrice <= 0 {
		return jsonError("buy_equity: symbol, positive quantity, and positive limit_price are required")
	}
	m := Move{
		Action:     ActionBuyEquity,
		AssetType:  AssetEquity,
		Symbol:     args.Symbol,
		Underlying: args.Symbol,
		Quantity:   args.Quantity,
		Notional:   args.Quantity * args.LimitPrice,
	}
	return d.commitBuy(ctx, m, args.LimitPrice, args.Rationale, func() (string, int, error) {
		return d.exec.BuyEquity(ctx, args.Symbol, args.Quantity, args.LimitPrice)
	})
}

func (d *ToolDispatcher) dispatchBuyOption(ctx context.Context, input json.RawMessage) string {
	var args struct {
		OCCSymbol    string  `json:"occ_symbol"`
		Underlying   string  `json:"underlying"`
		ContractType string  `json:"contract_type"`
		Strike       float64 `json:"strike"`
		Expiration   string  `json:"expiration"`
		Contracts    int     `json:"contracts"`
		LimitPrice   float64 `json:"limit_price"`
		Rationale    string  `json:"rationale"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("buy_option: invalid arguments")
	}
	args.OCCSymbol = strings.ToUpper(strings.TrimSpace(args.OCCSymbol))
	args.Underlying = strings.ToUpper(strings.TrimSpace(args.Underlying))
	args.ContractType = strings.ToUpper(strings.TrimSpace(args.ContractType))
	if args.OCCSymbol == "" || args.Underlying == "" || args.Contracts <= 0 || args.LimitPrice <= 0 {
		return jsonError("buy_option: occ_symbol, underlying, positive contracts, and positive limit_price are required")
	}
	m := Move{
		Action:     ActionBuyOption,
		AssetType:  AssetOption,
		Symbol:     args.OCCSymbol,
		Underlying: args.Underlying,
		Quantity:   float64(args.Contracts),
		Notional:   float64(args.Contracts) * args.LimitPrice * 100,
	}
	return d.commitBuy(ctx, m, args.LimitPrice, args.Rationale, func() (string, int, error) {
		return d.exec.BuyOption(ctx, args.OCCSymbol, args.Underlying, args.ContractType, args.Strike, args.Expiration, args.Contracts, args.LimitPrice)
	})
}

func (d *ToolDispatcher) dispatchSellEquity(ctx context.Context, input json.RawMessage) string {
	var args struct {
		Symbol     string  `json:"symbol"`
		Quantity   float64 `json:"quantity"`
		LimitPrice float64 `json:"limit_price"`
		Rationale  string  `json:"rationale"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("sell_equity: invalid arguments")
	}
	args.Symbol = strings.ToUpper(strings.TrimSpace(args.Symbol))
	if args.Symbol == "" || args.Quantity <= 0 || args.LimitPrice <= 0 {
		return jsonError("sell_equity: symbol, positive quantity, and positive limit_price are required")
	}
	m := Move{Action: ActionSellEquity, AssetType: AssetEquity, Symbol: args.Symbol, Underlying: args.Symbol, Quantity: args.Quantity}
	return d.commitSell(ctx, m, args.LimitPrice, args.Rationale, func() (string, int, error) {
		return d.exec.SellEquity(ctx, args.Symbol, args.Quantity, args.LimitPrice)
	})
}

func (d *ToolDispatcher) dispatchSellOption(ctx context.Context, input json.RawMessage) string {
	var args struct {
		OCCSymbol  string  `json:"occ_symbol"`
		Contracts  int     `json:"contracts"`
		LimitPrice float64 `json:"limit_price"`
		Rationale  string  `json:"rationale"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("sell_option: invalid arguments")
	}
	args.OCCSymbol = strings.ToUpper(strings.TrimSpace(args.OCCSymbol))
	if args.OCCSymbol == "" || args.Contracts <= 0 || args.LimitPrice <= 0 {
		return jsonError("sell_option: occ_symbol, positive contracts, and positive limit_price are required")
	}
	m := Move{Action: ActionSellOption, AssetType: AssetOption, Symbol: args.OCCSymbol, Quantity: float64(args.Contracts)}
	return d.commitSell(ctx, m, args.LimitPrice, args.Rationale, func() (string, int, error) {
		return d.exec.SellOption(ctx, args.OCCSymbol, args.Contracts, args.LimitPrice)
	})
}

func (d *ToolDispatcher) dispatchHold(input json.RawMessage) string {
	var args struct {
		Symbol    string `json:"symbol"`
		Rationale string `json:"rationale"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("hold: invalid arguments")
	}
	symbol := strings.ToUpper(strings.TrimSpace(args.Symbol))
	if symbol == "" {
		return jsonError("hold: symbol is required (hold is only for continuing a position you already held)")
	}
	reason := strings.TrimSpace(args.Rationale)
	if reason == "" {
		reason = "(no reason provided)"
	}
	d.mu.Lock()
	d.decisions = append(d.decisions, Decision{Action: ActionHold, Symbol: symbol, Underlying: symbol, Rationale: reason})
	d.mu.Unlock()
	out, _ := json.Marshal(map[string]any{"ok": true, "action": "hold", "symbol": symbol})
	return string(out)
}

/*
dispatchWriteSummary records the model's plain-language session summary. It
is a documentation tool: it neither reads market data nor moves money, it
just stores the day's writeup (the last call wins if it writes more than
once). The cron persists it after the run.
*/
func (d *ToolDispatcher) dispatchWriteSummary(input json.RawMessage) string {
	var args struct {
		Synopsis    string `json:"synopsis"`
		ActionItems string `json:"action_items"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return jsonError("write_summary: invalid arguments")
	}
	synopsis := strings.TrimSpace(args.Synopsis)
	actionItems := strings.TrimSpace(args.ActionItems)
	if synopsis == "" {
		return jsonError("write_summary: synopsis is empty")
	}
	d.mu.Lock()
	d.summary = synopsis
	d.actionItems = actionItems
	d.mu.Unlock()
	out, _ := json.Marshal(map[string]any{"ok": true, "action": "write_summary", "stored": true})
	return string(out)
}

// Summary returns today's synopsis recorded via write_summary (empty if the
// model never called it).
func (d *ToolDispatcher) Summary() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.summary
}

// ActionItems returns the next-session plan recorded via write_summary.
func (d *ToolDispatcher) ActionItems() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.actionItems
}

/*
commitBuy is the shared buy path for equity + options: gather live
liquidity, run the cap checks against the WORKING snapshot under the lock,
place the order through the broker, then update the working snapshot and
record the decision. place is the broker call (closes over the typed args).
*/
func (d *ToolDispatcher) commitBuy(ctx context.Context, m Move, limitPrice float64, rationale string, place func() (string, int, error)) string {
	// Liquidity gather happens outside the lock (it makes network calls).
	liq, err := d.reader.Liquidity(ctx, m)
	if err != nil {
		return jsonError(fmt.Sprintf("%s: liquidity check failed: %v", m.Action, err))
	}
	m.Liquidity = liq

	d.mu.Lock()
	defer d.mu.Unlock()

	if capErr := d.caps.CheckBuy(d.work, m); capErr != nil {
		return jsonError(fmt.Sprintf("%s refused (%s): %s", m.Action, capErr.Code, capErr.Message))
	}

	orderID, execID, err := place()
	if err != nil {
		log.Printf("portfolio: %s failed for %s: %v", m.Action, m.Symbol, err)
		return jsonError(fmt.Sprintf("%s: broker rejected: %v", m.Action, err))
	}

	// Update the working snapshot so subsequent moves see this exposure.
	d.work.SettledCash -= m.Notional
	d.work.DeployedThisSession += m.Notional
	d.applyBuyToPositionsLocked(m)

	dec := Decision{
		Action:      m.Action,
		AssetType:   m.AssetType,
		Symbol:      m.Symbol,
		Underlying:  m.Underlying,
		Quantity:    m.Quantity,
		Notional:    m.Notional,
		LimitPrice:  limitPrice,
		OrderID:     orderID,
		ExecutionID: execID,
		Rationale:   strings.TrimSpace(rationale),
	}
	d.decisions = append(d.decisions, dec)

	out, _ := json.Marshal(map[string]any{
		"ok":                     true,
		"action":                 m.Action,
		"symbol":                 m.Symbol,
		"quantity":               m.Quantity,
		"limit_price":            limitPrice,
		"notional":               m.Notional,
		"order_id":               orderID,
		"execution_id":           execID,
		"deployment_budget_left": d.work.DeploymentBudget - d.work.DeployedThisSession,
		"settled_cash_left":      d.work.SettledCash,
	})
	return string(out)
}

func (d *ToolDispatcher) commitSell(ctx context.Context, m Move, limitPrice float64, rationale string, place func() (string, int, error)) string {
	d.mu.Lock()
	defer d.mu.Unlock()

	if capErr := d.caps.CheckSell(d.work, m); capErr != nil {
		return jsonError(fmt.Sprintf("%s refused (%s): %s", m.Action, capErr.Code, capErr.Message))
	}

	orderID, execID, err := place()
	if err != nil {
		log.Printf("portfolio: %s failed for %s: %v", m.Action, m.Symbol, err)
		return jsonError(fmt.Sprintf("%s: broker rejected: %v", m.Action, err))
	}

	d.applySellToPositionsLocked(m)

	dec := Decision{
		Action:      m.Action,
		AssetType:   m.AssetType,
		Symbol:      m.Symbol,
		Quantity:    m.Quantity,
		LimitPrice:  limitPrice,
		OrderID:     orderID,
		ExecutionID: execID,
		Rationale:   strings.TrimSpace(rationale),
	}
	d.decisions = append(d.decisions, dec)
	out, _ := json.Marshal(map[string]any{"ok": true, "action": m.Action, "symbol": m.Symbol, "quantity": m.Quantity, "order_id": orderID, "execution_id": execID})
	return string(out)
}

/*
applyBuyToPositionsLocked folds a committed buy into the working snapshot:
add to an existing position with the same symbol, or append a new one.
MarkValue is approximated as the notional just committed (cost), which is
the right basis for the concentration/sleeve caps on subsequent moves.
Caller holds d.mu.
*/
func (d *ToolDispatcher) applyBuyToPositionsLocked(m Move) {
	for i := range d.work.Positions {
		if strings.EqualFold(d.work.Positions[i].Symbol, m.Symbol) {
			d.work.Positions[i].Quantity += m.Quantity
			d.work.Positions[i].MarkValue += m.Notional
			d.work.Positions[i].CostBasis += m.Notional
			return
		}
	}
	d.work.Positions = append(d.work.Positions, Position{
		Symbol:     m.Symbol,
		Underlying: m.Underlying,
		AssetType:  m.AssetType,
		Quantity:   m.Quantity,
		MarkValue:  m.Notional,
		CostBasis:  m.Notional,
	})
}

/*
applySellToPositionsLocked reduces (or removes) the matching position by
the sold quantity. Proceeds land in unsettled cash in a cash account (they
do not settle until T+1), so SettledCash is intentionally not credited.
Caller holds d.mu.
*/
func (d *ToolDispatcher) applySellToPositionsLocked(m Move) {
	for i := range d.work.Positions {
		p := &d.work.Positions[i]
		if !strings.EqualFold(p.Symbol, m.Symbol) {
			continue
		}
		soldFraction := 0.0
		if p.Quantity > 0 {
			soldFraction = m.Quantity / p.Quantity
		}
		proceedsBasis := p.MarkValue * soldFraction
		p.Quantity -= m.Quantity
		p.MarkValue -= proceedsBasis
		p.CostBasis -= p.CostBasis * soldFraction
		d.work.UnsettledCash += proceedsBasis
		if p.Quantity <= floatTol {
			d.work.Positions = append(d.work.Positions[:i], d.work.Positions[i+1:]...)
		}
		return
	}
}

/*
Decisions returns a copy of every move committed this session, in commit
order (the source of truth for what happened). Holds are included.
*/
func (d *ToolDispatcher) Decisions() []Decision {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]Decision, len(d.decisions))
	copy(out, d.decisions)
	return out
}

// CapsSummary renders the cap sheet as the human-readable block the prompt
// interpolates, so the prompt text and the enforced numbers never drift.
func CapsSummary(c Caps) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- Max exposure per underlying: %.0f%% of equity (equity + option premium on the same name count together).\n", c.MaxPerUnderlyingPct*100)
	fmt.Fprintf(&b, "- Max total option premium (the leverage sleeve): %.0f%% of equity.\n", c.MaxOptionsSleevePct*100)
	fmt.Fprintf(&b, "- Max single order: %.0f%% of equity.\n", c.MaxPerOrderPct*100)
	fmt.Fprintf(&b, "- Daily new-deployment budget: %.0f%% of settled cash per session (see deployment_budget_left in get_portfolio).\n", DefaultDailyDeploymentPct*100)
	fmt.Fprintf(&b, "- New buys are HALTED once equity is down %.0f%% from the high-water mark (de-risking still allowed).\n", c.DrawdownHaltPct*100)
	fmt.Fprintf(&b, "- Liquidity floor: stock >= $%.0f, market cap >= $%.0fB, option OI >= %d and volume >= %d, spread <= %.0f%%.\n", c.MinStockPrice, c.MinUnderlyingMktCap/1e9, c.MinOptionOpenInterest, c.MinOptionVolume, c.MaxSpreadPct*100)
	fmt.Fprintf(&b, "- You spend SETTLED cash only; unsettled sale proceeds free up at T+1.")
	return b.String()
}

func splitCSV(s string) []string {
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		if t := strings.TrimSpace(r); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func jsonError(msg string) string {
	out, _ := json.Marshal(map[string]string{"error": msg})
	return string(out)
}
