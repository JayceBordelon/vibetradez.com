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
account so the drawdown breaker never trips on day one). There is no
session deployment budget: all settled cash is deployable.

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
		Equity:        equity,
		SettledCash:   settledCash,
		UnsettledCash: 0,
		HighWaterMark: highWater,
		Positions:     positions,
	}, nil
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
