/*
Package portfoliowire binds the abstract portfolio.PortfolioReader and
portfolio.PortfolioExecutor interfaces to the concrete subsystems: the
Schwab market-data + Trader client (via exec.Service) and the store. It is
the seam between the decision engine (internal/portfolio, which knows
nothing about Schwab or Postgres) and the live infrastructure.

Kept as its own package — rather than inline in cmd/scanner — so the
adapters are unit-testable against fakes and so cmd/scanner stays thin
wiring. Construction is in cmd/scanner/main.go behind the
PORTFOLIO_MODE_ENABLED flag.
*/
package portfoliowire

import (
	"context"

	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/portfolio"
	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/store"
)

/*
MarketData is the read slice of *schwab.Client the reader needs. An
interface so tests can fake it.
*/
type MarketData interface {
	GetQuotes(symbols []string) (map[string]schwab.StockQuote, error)
	GetOptionChain(symbol, contractType, fromDate, toDate string, strike float64) (*schwab.OptionChain, error)
	// MarketCap returns the underlying's market capitalization in absolute
	// USD (via the Schwab /instruments fundamental projection), for the
	// liquidity floor.
	MarketCap(symbol string) (float64, error)
	// GetDailyPriceHistory + GetFundamentals back the agent's
	// get_price_history and get_fundamentals tools.
	GetDailyPriceHistory(symbol string) (*schwab.PriceHistory, error)
	GetFundamentals(symbol string) (*schwab.Fundamentals, error)
}

/*
Broker is the slice of *exec.Service the reader + executor need: the
account read side (positions, funds) and the portfolio order entry points.
*/
type Broker interface {
	GetPositionsAgent(ctx context.Context) ([]exec.BrokerPosition, error)
	AvailableFundsAgent(ctx context.Context) (float64, error)
	PlaceEquityOrderAgent(ctx context.Context, symbol, side string, quantity int, limitPrice float64) (string, error)
	PlaceOptionOrderAgent(ctx context.Context, occSymbol, instruction string, quantity int, limitPrice float64) (string, error)
	OrderStatusAgent(ctx context.Context, orderID string) (exec.OrderStatus, error)
	CancelOrderAgent(ctx context.Context, orderID string) error
}

// PortfolioStore is the slice of *store.Store the reader needs: the
// high-water mark for the drawdown breaker, and the recent decision /
// stance history for the get_recent_decisions tool.
type PortfolioStore interface {
	GetHighWaterMark() (mark float64, ok bool, err error)
	RecentPortfolioDecisions(limit int) ([]store.PortfolioDecisionRow, error)
	RecentPortfolioStances(limit int) ([]store.PortfolioStanceRow, error)
	LatestPortfolioSession() (synopsis, actionItems string, ok bool, err error)
}

/*
Reader implements portfolio.PortfolioReader. It assembles the session
snapshot from the broker (positions + settled cash) and the store
(high-water mark), and serves the agent's market-data tools straight from
Schwab.
*/
type Reader struct {
	md MarketData
	bk Broker
	st PortfolioStore
}

func NewReader(md MarketData, bk Broker, st PortfolioStore) *Reader {
	return &Reader{md: md, bk: bk, st: st}
}

func (r *Reader) GetQuotes(symbols []string) (map[string]schwab.StockQuote, error) {
	return r.md.GetQuotes(symbols)
}

// PriorSession surfaces the most recent prior session's synopsis + action
// items so the agent starts the day where it left off.
func (r *Reader) PriorSession() (synopsis, actionItems string, ok bool, err error) {
	return r.st.LatestPortfolioSession()
}

// OrderStatus maps the broker's order state into the portfolio view the
// get_order_status tool returns.
func (r *Reader) OrderStatus(ctx context.Context, orderID string) (portfolio.OrderStatus, error) {
	st, err := r.bk.OrderStatusAgent(ctx, orderID)
	if err != nil {
		return portfolio.OrderStatus{}, err
	}
	return portfolio.OrderStatus{
		OrderID:        st.OrderID,
		Status:         st.RawStatus,
		Filled:         st.Filled,
		Working:        st.Working,
		Terminal:       st.Terminal,
		Quantity:       st.Quantity,
		FilledQuantity: st.FilledQuantity,
		FillPrice:      st.FillPrice,
		LimitPrice:     st.LimitPrice,
	}, nil
}

func (r *Reader) GetOptionChain(symbol, contractType, fromDate, toDate string, strike float64) (*schwab.OptionChain, error) {
	return r.md.GetOptionChain(symbol, contractType, fromDate, toDate, strike)
}

func (r *Reader) GetDailyPriceHistory(symbol string) (*schwab.PriceHistory, error) {
	return r.md.GetDailyPriceHistory(symbol)
}

func (r *Reader) GetFundamentals(symbol string) (*schwab.Fundamentals, error) {
	return r.md.GetFundamentals(symbol)
}

// RecentDecisions maps the store's recent decision rows to the agent's
// history view (newest first).
func (r *Reader) RecentDecisions(limit int) ([]portfolio.HistoryEntry, error) {
	rows, err := r.st.RecentPortfolioDecisions(limit)
	if err != nil {
		return nil, err
	}
	out := make([]portfolio.HistoryEntry, 0, len(rows))
	for _, d := range rows {
		out = append(out, portfolio.HistoryEntry{
			Date:       d.Date,
			Action:     d.Action,
			Symbol:     d.Symbol,
			Underlying: d.Underlying,
			Notional:   d.Notional,
			Rationale:  d.Rationale,
		})
	}
	return out, nil
}

// RecentStances maps the store's recent stance rows to the agent's view.
func (r *Reader) RecentStances(limit int) ([]portfolio.StanceEntry, error) {
	rows, err := r.st.RecentPortfolioStances(limit)
	if err != nil {
		return nil, err
	}
	out := make([]portfolio.StanceEntry, 0, len(rows))
	for _, s := range rows {
		out = append(out, portfolio.StanceEntry{Date: s.Date, Stance: s.Stance})
	}
	return out, nil
}

/*
Snapshot builds the session-start account state. Equity is the marked
value of every position plus settled cash. The high-water mark comes from
the persisted equity curve (falling back to current equity for a fresh
account so the drawdown breaker never trips on day one). DeploymentBudget
is the per-session new-buy budget: settled cash times the daily-deployment
cap.

Note: UnsettledCash is reported as 0 here. AvailableFundsAgent returns the
cash available for a new trade (which already excludes unsettled proceeds
in a cash account), so settled cash is correct for the caps; surfacing the
unsettled portion separately needs the fuller Schwab balances payload and
is a follow-up.
*/
func (r *Reader) Snapshot(ctx context.Context) (portfolio.Snapshot, error) {
	brokerPositions, err := r.bk.GetPositionsAgent(ctx)
	if err != nil {
		return portfolio.Snapshot{}, err
	}
	settledCash, err := r.bk.AvailableFundsAgent(ctx)
	if err != nil {
		return portfolio.Snapshot{}, err
	}

	positions := make([]portfolio.Position, 0, len(brokerPositions))
	var positionsValue float64
	for _, bp := range brokerPositions {
		positionsValue += bp.MarketValue
		positions = append(positions, toPortfolioPosition(bp))
	}

	equity := positionsValue + settledCash
	highWater := equity
	if hwm, ok, herr := r.st.GetHighWaterMark(); herr == nil && ok && hwm > highWater {
		highWater = hwm
	}

	return portfolio.Snapshot{
		Equity:              equity,
		SettledCash:         settledCash,
		UnsettledCash:       0,
		HighWaterMark:       highWater,
		DeploymentBudget:    settledCash * portfolio.DefaultDailyDeploymentPct,
		DeployedThisSession: 0,
		Positions:           positions,
	}, nil
}

/*
Liquidity gathers the tradability facts for a proposed buy. Price and
spread come from the live equity quote; market cap from the Schwab
/instruments fundamental projection; for options the open interest, volume,
and contract spread come from the live chain row. All four floors
(price, market cap, spread, and option OI/volume) are enforced.

Market cap fails OPEN: if the /instruments call errors or returns no value,
this passes the market-cap gate (with a logged caveat) rather than blocking
a buy on a transient data hiccup. The other three floors still bite, and a
successful fetch enforces the real value.
*/
func (r *Reader) Liquidity(ctx context.Context, m portfolio.Move) (portfolio.LiquidityCtx, error) {
	_ = ctx
	out := portfolio.LiquidityCtx{}
	quotes, err := r.md.GetQuotes([]string{m.Underlying})
	if err != nil {
		return out, err
	}
	q := quotes[m.Underlying]
	out.StockPrice = q.Mark
	if out.StockPrice <= 0 {
		out.StockPrice = q.LastPrice
	}

	if mc, mcErr := r.md.MarketCap(m.Underlying); mcErr == nil && mc > 0 {
		out.UnderlyingMktCap = mc
	} else {
		// Fail open: don't block a buy because the fundamentals call
		// hiccuped. The other floors still apply.
		out.UnderlyingMktCap = marketCapUnavailableSentinel
		logMarketCapUnavailableOnce()
	}

	switch m.AssetType {
	case portfolio.AssetEquity:
		out.SpreadPct = spreadPct(q.BidPrice, q.AskPrice, out.StockPrice)
	case portfolio.AssetOption:
		// Decode the OCC symbol to find the contract on the chain.
		_, exp, ctype, strike, derr := exec.DecodeOCCSymbol(m.Symbol)
		if derr != nil {
			return out, derr
		}
		chain, cerr := r.md.GetOptionChain(m.Underlying, ctype, exp, exp, strike)
		if cerr != nil {
			return out, cerr
		}
		if oc := findContract(chain, ctype, strike); oc != nil {
			out.OptionOpenInterest = oc.OpenInterest
			out.OptionVolume = oc.TotalVolume
			mark := oc.Mark
			if mark <= 0 {
				mark = (oc.Bid + oc.Ask) / 2
			}
			out.SpreadPct = spreadPct(oc.Bid, oc.Ask, mark)
		}
	}

	return out, nil
}

func toPortfolioPosition(bp exec.BrokerPosition) portfolio.Position {
	at := portfolio.AssetEquity
	if bp.AssetType == "OPTION" {
		at = portfolio.AssetOption
	}
	p := portfolio.Position{
		Symbol:       bp.Symbol,
		Underlying:   bp.Underlying,
		AssetType:    at,
		Quantity:     bp.Quantity,
		MarkValue:    bp.MarketValue,
		ContractType: bp.ContractType,
		Strike:       bp.Strike,
		Expiration:   bp.Expiration,
	}
	// CostBasis: average cost × quantity × multiplier.
	mult := 1.0
	if at == portfolio.AssetOption {
		mult = 100
	}
	p.CostBasis = bp.AverageCost * bp.Quantity * mult
	// Recover the option contract spec + DTE from the OCC symbol when the
	// broker payload didn't carry them.
	if at == portfolio.AssetOption && (p.Strike == 0 || p.Expiration == "") {
		if _, exp, ctype, strike, err := exec.DecodeOCCSymbol(bp.Symbol); err == nil {
			p.Expiration = exp
			p.ContractType = ctype
			p.Strike = strike
		}
	}
	if at == portfolio.AssetOption && p.Expiration != "" {
		p.DTE = daysToExpiration(p.Expiration)
	}
	return p
}

func spreadPct(bid, ask, mark float64) float64 {
	if mark <= 0 || ask <= 0 || bid <= 0 {
		return 0
	}
	return (ask - bid) / mark
}

func findContract(chain *schwab.OptionChain, contractType string, strike float64) *schwab.OptionContract {
	if chain == nil {
		return nil
	}
	list := chain.Calls
	if contractType == "PUT" {
		list = chain.Puts
	}
	for i := range list {
		if list[i].StrikePrice == strike {
			return &list[i]
		}
	}
	return nil
}

/*
Executor implements portfolio.PortfolioExecutor by translating each move
into the matching exec.Service broker entry point. It does not persist —
the caller records portfolio_decisions after the agent run from the
dispatcher's recorded decisions (which carry the rationale). execID is
returned as 0; the broker order id is the meaningful handle.
*/
type Executor struct {
	bk Broker
}

func NewExecutor(bk Broker) *Executor {
	return &Executor{bk: bk}
}

func (e *Executor) BuyEquity(ctx context.Context, symbol string, quantity float64, limitPrice float64) (string, int, error) {
	id, err := e.bk.PlaceEquityOrderAgent(ctx, symbol, "BUY", int(quantity), limitPrice)
	return id, 0, err
}

func (e *Executor) SellEquity(ctx context.Context, symbol string, quantity float64, limitPrice float64) (string, int, error) {
	id, err := e.bk.PlaceEquityOrderAgent(ctx, symbol, "SELL", int(quantity), limitPrice)
	return id, 0, err
}

func (e *Executor) BuyOption(ctx context.Context, occSymbol, underlying, contractType string, strike float64, expiration string, contracts int, limitPrice float64) (string, int, error) {
	_ = underlying
	_ = contractType
	_ = strike
	_ = expiration
	id, err := e.bk.PlaceOptionOrderAgent(ctx, occSymbol, "BUY_TO_OPEN", contracts, limitPrice)
	return id, 0, err
}

func (e *Executor) SellOption(ctx context.Context, occSymbol string, contracts int, limitPrice float64) (string, int, error) {
	id, err := e.bk.PlaceOptionOrderAgent(ctx, occSymbol, "SELL_TO_CLOSE", contracts, limitPrice)
	return id, 0, err
}

func (e *Executor) CancelOrder(ctx context.Context, orderID string) error {
	return e.bk.CancelOrderAgent(ctx, orderID)
}

// Compile-time checks that the adapters satisfy the portfolio interfaces.
var (
	_ portfolio.PortfolioReader   = (*Reader)(nil)
	_ portfolio.PortfolioExecutor = (*Executor)(nil)
)

// marketCapUnavailableSentinel passes the market-cap floor when the
// /instruments fundamentals call is unavailable, so a transient data hiccup
// fails open rather than blocking a buy (see Reader.Liquidity).
const marketCapUnavailableSentinel = 1e15
