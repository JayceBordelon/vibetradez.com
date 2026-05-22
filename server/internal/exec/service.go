package exec

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"vibetradez.com/internal/email"
	"vibetradez.com/internal/templates"
	"vibetradez.com/internal/trades"
)

/*
schwabPositionsURL is the deep link surfaced in receipt emails. Used
verbatim per the task spec, do not parameterize.
*/
const schwabPositionsURL = "https://client.schwab.com/app/accounts/positions/#/"

/*
DecisionStore is the slice of *store.Store that exec.Service needs.
Defined as an interface so tests don't need a real Postgres.
*/
type DecisionStore interface {
	InsertExecution(e Execution) (int, error)
	UpdateExecutionStatus(id int, status, schwabOrderID string, fillPrice *float64, filledQty int, errMsg string) error
	GetExecution(id int) (*Execution, error)
	OpenExecutionForTrade(tradeID int) (*Execution, error)
	LiveExecutionsForDate(tradeDate string) ([]Execution, error)
	OpenPositionsForDate(tradeDate string) ([]OpenPosition, error)
	WorkingOpenPositionsForDate(tradeDate string) ([]OpenPosition, error)
	/*
		BasketSummaryForDate returns one row per open-side execution
		for the given trade_date, joined to the trade row for symbol/
		strike/contract_type/rank. Used by SendExecutionSummary to
		render the single consolidated post-open email.
	*/
	BasketSummaryForDate(tradeDate string) ([]BasketSummaryRow, error)
	/*
		CloseSummaryForDate returns one row per close-side execution
		for the given trade_date with the matching open's fill price
		stitched in so realized P&L is computable per position. Used
		by SendCloseSummary to render the single consolidated 3:55 ET
		email that replaced per-trade close receipt + close-failed
		emails.
	*/
	CloseSummaryForDate(tradeDate string) ([]CloseSummaryRow, error)
	/*
		ResolveTradeContract writes the five at-open-resolved columns
		into the trades row in a single UPDATE: strike_price, expiration,
		dte, estimated_price, stop_loss. Called by PlaceBuyToOpenAgent
		when the at-open execagent picks a concrete contract for a
		candidate and fires the order, so the dashboard / EOD path /
		close cron all see the chosen contract spec on the trade row.
	*/
	ResolveTradeContract(tradeID int, strike float64, expiration string, dte int, estimatedPrice, stopLoss float64) error
}

/*
BasketSummaryRow is the single-execution snapshot the morning summary
email renders. Field semantics:

  - Rank, Symbol, ContractType, StrikePrice — pulled from the trade row
  - Mode — "live" or "paper"
  - Status — open-side execution status (filled / working / canceled /
    failed / rejected / pending)
  - LimitPrice — what we asked Schwab for (per share)
  - FillPrice — what we paid (per share). Zero if not filled.
  - Quantity — number of contracts on the open side
  - ErrorMessage — only populated for failed/canceled/rejected
*/
type BasketSummaryRow struct {
	Rank         int
	Symbol       string
	ContractType string
	StrikePrice  float64
	Mode         string
	Status       string
	LimitPrice   float64
	FillPrice    float64
	Quantity     int
	ErrorMessage string
}

/*
CloseSummaryRow is one close-side execution snapshot the close-summary
email renders. Open/close fill prices come from the executions table
(broker truth), realized P&L is computed at query time as
(close - open) × 100 × quantity. Status reflects the close-side
disposition (filled / failed / working / pending). ErrorMessage is
populated for the failure modes that previously fired a per-trade
"close failed" alert; the consolidated email surfaces them inline.
*/
type CloseSummaryRow struct {
	Rank         int
	Symbol       string
	ContractType string
	StrikePrice  float64
	Mode         string
	Status       string
	OpenPrice    float64
	ClosePrice   float64
	RealizedPnL  float64
	Quantity     int
	ErrorMessage string
}

// MailSender is the slice of *email.Client that exec.Service needs.
type MailSender interface {
	SendTradeEmail(from string, to []string, subject, htmlContent string) error
}

/*
ServiceConfig captures everything the executor needs to know about the
world. Built from cfg in main.go.
*/
type ServiceConfig struct {
	Mode              string
	Recipient         string
	EmailFrom         string
	ModelLabel        string
	SchwabAccountHash func(ctx context.Context) (string, error)
}

/*
Service orchestrates the auto-execution lifecycle. One instance per
process. Public methods that read/write executions are serialized
through `mu` because the cron schedule overlaps: at 15:55 ET the
per-minute reconcile cron AND the position-close cron both fire on
the same minute (robfig/cron runs each job in its own goroutine),
and either path can read 'working' positions and then mutate them.
Without the mutex, the close cron can read positions before the
reconcile cron flips a just-filled order from 'working' to 'filled',
which leaves the position open when the close path skips it.

The lock is held across the full method body (including Schwab
network calls) on purpose: these methods only run sequentially in
practice (once a minute reconcile, once a day morning/close), so the
extra latency cost is zero and the simplicity of "one big lock"
beats trying to scope it tighter.
*/
type Service struct {
	store  DecisionStore
	trader TraderClient
	mail   MailSender
	cfg    ServiceConfig
	mu     sync.Mutex
}

func NewService(store DecisionStore, trader TraderClient, mail MailSender, cfg ServiceConfig) *Service {
	return &Service{store: store, trader: trader, mail: mail, cfg: cfg}
}

/*
Mode returns the trading mode the service was constructed with
("paper" | "live"). Used by the /health endpoint to decide whether
schwab_trading auth failures are fatal (live) or merely a warning
(paper, trading scope isn't load-bearing in paper mode).
*/
func (s *Service) Mode() string { return s.cfg.Mode }

/*
PlaceBuyToOpenAgent is the at-open-execution-agent entry point. The
agent (internal/execagent) has chosen a specific strike, expiration,
and limit price from the now-live chain; this method:

 1. Builds the OCC symbol from the agent's chosen contract spec
 2. Persists those contract specs onto the trade row (so the EOD
    path / dashboard / close cron all have something to work with)
 3. Inserts a 'pending' open-side execution row
 4. Submits the LIMIT × 1 BUY_TO_OPEN order to Schwab
 5. Flips the row to 'working' with the broker order_id

Fire-and-forget by design: this does NOT poll for fill. The per-minute
ReconcileOpenOrders cron picks fills up; the 9:35 CancelDanglingOpens
cron handles anything still working at the cutoff. Returns the broker
order_id and the executions.id row id; caller surfaces the order_id to
the agent's tool response so Claude can reference it in its final JSON.

Single-contract only (quantity hard-coded to 1) — the execagent fires
at most one order per pick per day. Caller (the tool layer in
internal/execagent/tools.go) enforces the per-rank uniqueness, the
3-orders-per-day cap, and the off-list-symbol refusal. This method's
contract is "given a validated single-contract buy, submit it".
*/
func (s *Service) PlaceBuyToOpenAgent(
	ctx context.Context,
	t *trades.Trade,
	strike float64,
	expiration string,
	dte int,
	limitPrice float64,
) (orderID string, execID int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t == nil || t.ID == 0 {
		return "", 0, errors.New("trade is nil or missing ID")
	}
	if limitPrice <= 0 || limitPrice > MaxContractPremium {
		return "", 0, fmt.Errorf("limit price %.2f outside (0, %.2f]", limitPrice, MaxContractPremium)
	}

	occ, err := OCCSymbol(t.Symbol, expiration, t.ContractType, strike)
	if err != nil {
		return "", 0, fmt.Errorf("build OCC symbol: %w", err)
	}

	/*
		Stamp the agent's chosen contract spec onto the trade row before
		placing the order. The dashboard renders strike/expiration off
		the trade row; if PlaceOrder succeeds but the trade row never
		got the spec, the UI shows "Finding contracts..." indefinitely
		for a position that's actually live at the broker. stop_loss is
		set to half the entry premium as a placeholder. It is NOT
		currently auto-traded against; it is persisted purely for display
		on the dashboard / EOD path and for future stop-loss work.
	*/
	if err := s.store.ResolveTradeContract(t.ID, strike, expiration, dte, limitPrice, limitPrice*0.5); err != nil {
		return "", 0, fmt.Errorf("persist resolved contract on trade row: %w", err)
	}
	/*
		Mutate the in-memory trade so BuildOpenOrderForTrade can read
		strike/expiration off it without a DB re-read. The trade pointer
		comes from the agent's working copy, so this is safe to mutate.
	*/
	t.SetResolvedContract(strike, expiration, dte, limitPrice, limitPrice*0.5)

	order, err := BuildOpenOrderForTrade(t, occ, limitPrice, 1)
	if err != nil {
		return "", 0, fmt.Errorf("build open order: %w", err)
	}

	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("account hash: %w", err)
	}

	execRow := Execution{
		TradeID:           t.ID,
		Mode:              s.cfg.Mode,
		Side:              "open",
		Status:            "pending",
		RequestedQuantity: 1,
	}
	execID, err = s.store.InsertExecution(execRow)
	if err != nil {
		return "", 0, fmt.Errorf("insert open execution: %w", err)
	}

	orderID, err = s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", "", nil, 0, err.Error())
		return "", execID, fmt.Errorf("place open order: %w", err)
	}

	/*
		Persist the broker order id IMMEDIATELY (same orphan-prevention
		rationale as handleSingleEntry). If the process crashes between
		this and the next reconcile-cron tick, the order is still live
		at Schwab and the row is in 'working' state with the id so it
		can be picked up.
	*/
	if err := s.store.UpdateExecutionStatus(execID, "working", orderID, nil, 0, ""); err != nil {
		log.Printf("execution: warning: persist mid-flight orderID for trade %d (order=%s): %v", t.ID, orderID, err)
	}

	log.Printf("execagent: open order submitted (trade=%d, rank=%d, %s %s $%.2f exp=%s limit=$%.2f, order=%s)",
		t.ID, t.Rank, t.Symbol, t.ContractType, strike, expiration, limitPrice, orderID)
	return orderID, execID, nil
}

/*
InsertSkippedExecutionAgent writes a synthetic open-side execution row
with status='skipped' to record that the at-open agent looked at this
pick and explicitly declined to trade it. The reason goes into
error_message and is surfaced in the 9:35 SendExecutionSummary email
and on the dashboard.

Status 'skipped' is a deliberate new enum value distinct from:
  - 'pending'  — order in flight, never reached the broker (in-process)
  - 'failed'   — order attempted, broker rejected or DB write failed
  - 'rejected' — order reached broker, broker terminally refused
  - 'canceled' — order placed, then canceled (typically by 9:35 cron)

'skipped' means: no order was ever attempted, by deliberate model
choice. requested_quantity=0 reinforces the "nothing was placed" shape.
*/
func (s *Service) InsertSkippedExecutionAgent(tradeID int, reason string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tradeID == 0 {
		return 0, errors.New("trade ID required for skipped execution")
	}
	row := Execution{
		TradeID:           tradeID,
		Mode:              s.cfg.Mode,
		Side:              "open",
		Status:            "skipped",
		RequestedQuantity: 0,
		ErrorMessage:      reason,
	}
	id, err := s.store.InsertExecution(row)
	if err != nil {
		return 0, fmt.Errorf("insert skipped execution: %w", err)
	}
	if err := s.store.UpdateExecutionStatus(id, "skipped", "", nil, 0, reason); err != nil {
		log.Printf("execution: warning: persist skip status (trade=%d): %v", tradeID, err)
	}
	log.Printf("execagent: skip persisted (trade=%d, reason=%q)", tradeID, reason)
	return id, nil
}

/*
AvailableFundsAgent is the at-open-execution-agent's view of available
cash. Calls SchwabAccountHash + AvailableFunds under the service mutex
so it can't race with an in-flight PlaceBuyToOpenAgent on the same
process. Exposed to the agent as the `get_account_funds` tool.
*/
func (s *Service) AvailableFundsAgent(ctx context.Context) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		return 0, fmt.Errorf("account hash: %w", err)
	}
	return s.trader.AvailableFunds(ctx, hash)
}

/*
ReconcileOpenOrders re-polls Schwab for every open-side execution that
placed but hasn't yet been observed FILLED. Fired by a per-minute cron
between market open and the 3:55 close cron, this closes the gap
between order acceptance and confirmed fill — the morning cron's
single post-PlaceOrder GetOrder call is racy because pre-market LIMIT
orders typically remain WORKING for several seconds-to-minutes before
the broker fills.

Without this, working rows stay in 'working' forever:
  - the dashboard's deriveExecutionState would silently hide them
    (pre-fix), and
  - OpenPositionsForDate filters by status='filled', so the 3:55
    close cron would skip the position entirely, leaving a real
    open contract at the broker past market close.

When a working row reaches FILLED here, we send the same receipt
email that handleSinglePick would have sent on a same-tick fill.
Terminal-not-filled outcomes (REJECTED / CANCELED / EXPIRED) flip to
'rejected' and notify the operator. Errors are logged but never
propagated, the cron must keep running.
*/
func (s *Service) ReconcileOpenOrders(ctx context.Context, tradeDate string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("execution: ReconcileOpenOrders top-level panic: %v", r)
		}
	}()

	positions, err := s.store.WorkingOpenPositionsForDate(tradeDate)
	if err != nil {
		log.Printf("execution: working positions: %v", err)
		return
	}
	if len(positions) == 0 {
		return
	}

	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		log.Printf("execution: account hash for reconcile: %v", err)
		return
	}

	for i := range positions {
		s.reconcileOne(ctx, hash, &positions[i])
	}
}

func (s *Service) reconcileOne(ctx context.Context, hash string, p *OpenPosition) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("execution: reconcileOne panic for trade %d: %v", p.Execution.TradeID, r)
		}
	}()

	if p.Execution.SchwabOrderID == nil || *p.Execution.SchwabOrderID == "" {
		// WorkingOpenPositionsForDate filters this out, but be defensive.
		return
	}
	orderID := *p.Execution.SchwabOrderID

	st, err := s.trader.GetOrder(ctx, hash, orderID)
	if err != nil {
		log.Printf("execution: reconcile GetOrder (trade=%d, order=%s): %v", p.Execution.TradeID, orderID, err)
		return
	}

	occ, _ := OCCSymbol(p.Symbol, p.Expiration, p.ContractType, p.StrikePrice)
	// Reconcile is silent on a per-fill basis — the 9:35 consolidated
	// summary email covers every open-side outcome for the day.
	_ = occ
	switch {
	case st.Filled:
		fp := st.FillPrice
		_ = s.store.UpdateExecutionStatus(p.Execution.ID, "filled", orderID, &fp, st.FilledQuantity, "")
		filledQty := st.FilledQuantity
		if filledQty < 1 {
			filledQty = positionCloseQuantity(p)
		}
		log.Printf("execution: reconcile open filled (trade=%d, qty=%d, mode=%s, fill=%.2f, order=%s)", p.Execution.TradeID, filledQty, p.Execution.Mode, st.FillPrice, orderID)
	case st.Terminal:
		reason := st.ErrorMessage
		if reason == "" {
			reason = st.RawStatus
		}
		_ = s.store.UpdateExecutionStatus(p.Execution.ID, "rejected", orderID, nil, 0, reason)
		log.Printf("execution: reconcile open rejected (trade=%d, order=%s, status=%s, reason=%q)", p.Execution.TradeID, orderID, st.RawStatus, reason)
	default:
		// Still working at the broker — leave the row, try again next tick.
	}
}

/*
CloseAllPositionsForDate is called by the 3:55pm ET load-bearing
safety job. Wraps each position close in its own panic recovery so
one failure can't prevent another from running. Designed to NEVER
skip, even if the morning open path crashed, this cron will fire as
long as the binary is up.
*/
func (s *Service) CloseAllPositionsForDate(ctx context.Context, tradeDate string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("execution: CloseAllPositionsForDate top-level panic: %v", r)
		}
	}()

	positions, err := s.store.OpenPositionsForDate(tradeDate)
	if err != nil {
		log.Printf("execution: open positions: %v", err)
		return
	}
	if len(positions) == 0 {
		log.Printf("execution: no open positions to close for %s", tradeDate)
		return
	}
	for i := range positions {
		s.closeOne(ctx, &positions[i])
	}
}

func (s *Service) closeOne(ctx context.Context, p *OpenPosition) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("execution: closeOne panic for trade %d: %v", p.Execution.TradeID, r)
		}
	}()

	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		s.recordCloseFailure(p, 0, fmt.Sprintf("account hash lookup failed: %v", err))
		return
	}

	closeQty := positionCloseQuantity(p)
	order, err := BuildCloseOrderForPosition(p, closeQty)
	if err != nil {
		s.recordCloseFailure(p, 0, fmt.Sprintf("build close order: %v", err))
		return
	}

	execRow := Execution{
		TradeID:           p.Execution.TradeID,
		Mode:              s.cfg.Mode,
		Side:              "close",
		Status:            "pending",
		RequestedQuantity: closeQty,
	}
	execID, err := s.store.InsertExecution(execRow)
	if err != nil {
		// No execID to update; this is a DB outage, log and move on.
		// The close-summary email will surface a missing close row for
		// this position so the operator can investigate.
		log.Printf("execution: insert close row failed for trade %d: %v", p.Execution.TradeID, err)
		return
	}

	orderID, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", "", nil, 0, fmt.Sprintf("first PlaceOrder: %v", err))
		return
	}
	// Same orphan-prevention rationale as the open path: persist the
	// broker order id immediately so a crash before pollFilled returns
	// leaves a 'working' row the reconcile cron can finish.
	if err := s.store.UpdateExecutionStatus(execID, "working", orderID, nil, 0, ""); err != nil {
		log.Printf("execution: warning: failed to persist close orderID mid-flight (trade_id=%d, order=%s): %v", p.Execution.TradeID, orderID, err)
	}
	if s.pollFilled(ctx, hash, orderID, 8, 15*time.Second) {
		s.recordCloseFilled(ctx, p, execID, hash, orderID)
		return
	}

	_ = s.trader.CancelOrder(ctx, hash, orderID)

	/*
		Re-poll after the cancel. The race we are closing: the first
		SELL_TO_CLOSE order can fill between pollFilled returning
		false and CancelOrder reaching the broker. Schwab's
		CancelOrder on a filled order returns success but the position
		is already closed. Without this check we would submit a
		second SELL_TO_CLOSE on a position that no longer exists,
		creating a short option (writer) position the operator never
		intended to hold. If the first order is now filled (or any
		non-canceled terminal), record it and skip the replacement.
	*/
	if postCancel, gErr := s.trader.GetOrder(ctx, hash, orderID); gErr == nil {
		if postCancel.Filled {
			s.recordCloseFilled(ctx, p, execID, hash, orderID)
			return
		}
		if postCancel.Terminal && postCancel.RawStatus != "CANCELED" && postCancel.RawStatus != "REPLACED" {
			_ = s.store.UpdateExecutionStatus(execID, "failed", orderID, nil, 0, fmt.Sprintf("close terminal mid-cancel as %s; cancel-replace skipped", postCancel.RawStatus))
			return
		}
	}

	orderID2, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", orderID, nil, 0, "cancel-replace failed: "+err.Error())
		return
	}
	// Persist the replacement order id immediately, same rationale.
	if err := s.store.UpdateExecutionStatus(execID, "working", orderID2, nil, 0, ""); err != nil {
		log.Printf("execution: warning: failed to persist cancel-replace orderID mid-flight (trade_id=%d, order=%s): %v", p.Execution.TradeID, orderID2, err)
	}
	if s.pollFilled(ctx, hash, orderID2, 8, 15*time.Second) {
		s.recordCloseFilled(ctx, p, execID, hash, orderID2)
		return
	}

	_ = s.store.UpdateExecutionStatus(execID, "failed", orderID2, nil, 0, "Position did not fill within 4-minute retry-cancel-replace window. Close on Schwab manually before 4:00pm ET.")
}

func (s *Service) pollFilled(ctx context.Context, hash, orderID string, attempts int, interval time.Duration) bool {
	for i := 0; i < attempts; i++ {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(interval):
		}
		st, err := s.trader.GetOrder(ctx, hash, orderID)
		if err != nil {
			log.Printf("execution: poll get order: %v", err)
			continue
		}
		if st.Filled {
			return true
		}
		if st.Terminal {
			return false
		}
	}
	return false
}

/*
recordCloseFilled persists the broker-side fill on the close execution
row. No per-trade email is sent anymore: the 3:55 ET SendCloseSummary
cron emits one consolidated daily email after all positions have been
attempted, sourcing this row's status + fill price via the store.
*/
func (s *Service) recordCloseFilled(ctx context.Context, p *OpenPosition, execID int, hash, orderID string) {
	st, err := s.trader.GetOrder(ctx, hash, orderID)
	if err != nil {
		log.Printf("execution: post-fill GetOrder: %v", err)
		return
	}
	fp := st.FillPrice
	if err := s.store.UpdateExecutionStatus(execID, "filled", orderID, &fp, st.FilledQuantity, ""); err != nil {
		log.Printf("execution: persist close fill (trade=%d, order=%s): %v", p.Execution.TradeID, orderID, err)
	}
}

/*
recordCloseFailure writes a close-side failure into the executions
table for cases that happen BEFORE an execID exists (account-hash
lookup, build-order error). The downstream SendCloseSummary email
reads these rows and surfaces the error_message inline; no per-trade
alert fires anymore.

When _execID is non-zero the caller has already written the failure
via UpdateExecutionStatus and this function is a no-op log. When
execID is zero, we synthesize a new close-side execution row in the
failed state so the email has something to render.
*/
func (s *Service) recordCloseFailure(p *OpenPosition, execID int, errMsg string) {
	if execID != 0 {
		log.Printf("execution: close failed (trade=%d, exec=%d): %s", p.Execution.TradeID, execID, errMsg)
		return
	}
	row := Execution{
		TradeID:           p.Execution.TradeID,
		Mode:              s.cfg.Mode,
		Side:              "close",
		Status:            "failed",
		RequestedQuantity: positionCloseQuantity(p),
		ErrorMessage:      errMsg,
	}
	id, err := s.store.InsertExecution(row)
	if err != nil {
		log.Printf("execution: insert pre-orderID close-failed row (trade=%d): %v", p.Execution.TradeID, err)
		return
	}
	if err := s.store.UpdateExecutionStatus(id, "failed", "", nil, 0, errMsg); err != nil {
		log.Printf("execution: persist pre-orderID close-failed status (trade=%d): %v", p.Execution.TradeID, err)
	}
}

/*
positionCloseQuantity returns the number of contracts the close path
should sell for this position. Prefers the open execution's actually-
filled quantity over the requested quantity; falls back to 1 if both
are zero/unset (defensive against legacy rows pre-dating multi-
contract baskets).
*/
func positionCloseQuantity(p *OpenPosition) int {
	if p.Execution.FilledQuantity >= 1 {
		return p.Execution.FilledQuantity
	}
	if p.Execution.RequestedQuantity >= 1 {
		return p.Execution.RequestedQuantity
	}
	return 1
}

/*
Compile-time guarantee that *email.Client satisfies MailSender. If
the email package's signature changes, this file fails to compile.
*/
var _ MailSender = (*email.Client)(nil)

/*
CancelDanglingOpens cancels any open-side executions that are still
WORKING at the broker at the 9:35 ET cutoff (5 minutes after market
open). Any LIMIT that hasn't filled by then is stale by definition:
either the live ask ran above our 10% buffer, or Schwab routed the
order through a slow venue that won't honor it. Cancel it and move
on. The pick is dead for the day.

Mirrors RepriceWorkingOpens' shape but does NOT re-submit. The
audit-driven redesign removed the cancel-and-replace path because
on fast-moving names a re-submitted LIMIT goes stale within
seconds; the trade-off the user picked is "wider 10% buffer at
9:30, no second chance" rather than "1.05× with multiple retry
windows."

Called by the 9:35 ET cron immediately before SendExecutionSummary
so the summary email reflects the final state of every order.
*/
func (s *Service) CancelDanglingOpens(ctx context.Context, tradeDate string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("execution: CancelDanglingOpens top-level panic: %v", r)
		}
	}()

	positions, err := s.store.WorkingOpenPositionsForDate(tradeDate)
	if err != nil {
		log.Printf("execution: cancel-dangling: working positions: %v", err)
		return
	}
	if len(positions) == 0 {
		return
	}

	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		log.Printf("execution: cancel-dangling: account hash: %v", err)
		return
	}

	log.Printf("execution: cancel-dangling pass for %d working open(s)", len(positions))
	for i := range positions {
		p := &positions[i]
		if p.Execution.SchwabOrderID == nil || *p.Execution.SchwabOrderID == "" {
			_ = s.store.UpdateExecutionStatus(p.Execution.ID, "canceled", "", nil, 0, "dangling LIMIT past 5min post-open (no broker order id)")
			continue
		}
		orderID := *p.Execution.SchwabOrderID
		if err := s.trader.CancelOrder(ctx, hash, orderID); err != nil {
			// Broker side may have already terminally moved the order in
			// the gap between the per-minute reconcile and this cron tick.
			// Reconcile will reconcile DB state on its next pass; we just
			// log and continue.
			log.Printf("execution: cancel-dangling: CancelOrder (trade=%d, order=%s): %v", p.Execution.TradeID, orderID, err)
			continue
		}
		_ = s.store.UpdateExecutionStatus(p.Execution.ID, "canceled", orderID, nil, 0, "dangling LIMIT past 5min post-open")
		log.Printf("execution: cancel-dangling: canceled order %s (trade=%d)", orderID, p.Execution.TradeID)
	}
}

/*
SendExecutionSummary queries the day's open-side executions and
sends ONE consolidated email to the operator recipient summarizing
fills, working orders (rare — should be none after CancelDanglingOpens
ran), failures, and cancels. Replaces the previous per-execution
receipt + open-failed emails that fired throughout the morning.

Empty-result case (no executions at all for the date) still sends a
"no executions today" notice so the operator can tell the difference
between "executor ran cleanly and chose to fire zero orders" and
"executor failed silently."

Caller orchestrates the 9:35 ET cron sequence:
 1. CancelDanglingOpens — flips any still-WORKING to canceled
 2. SendExecutionSummary — emails the final slate
*/
func (s *Service) SendExecutionSummary(ctx context.Context, tradeDate string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("execution: SendExecutionSummary top-level panic: %v", r)
		}
	}()

	rows, err := s.store.BasketSummaryForDate(tradeDate)
	if err != nil {
		log.Printf("execution: summary: BasketSummaryForDate(%s): %v", tradeDate, err)
		return
	}

	data := templates.BasketSummaryData{
		Date:      tradeDate,
		Mode:      s.cfg.Mode,
		Rows:      make([]templates.BasketSummaryRow, 0, len(rows)),
		TotalCost: 0,
	}
	totalFilled := 0
	for _, r := range rows {
		costPerContract := r.FillPrice
		if costPerContract == 0 {
			// For non-filled rows show the LIMIT we asked for (or zero)
			// so the operator sees the order size that was attempted.
			costPerContract = r.LimitPrice
		}
		totalCents := costPerContract * 100 * float64(r.Quantity)
		if r.Status == "filled" {
			data.TotalCost += totalCents
			totalFilled++
		}
		data.Rows = append(data.Rows, templates.BasketSummaryRow{
			Rank:         r.Rank,
			Symbol:       r.Symbol,
			ContractType: r.ContractType,
			StrikePrice:  r.StrikePrice,
			Mode:         r.Mode,
			Status:       r.Status,
			LimitPrice:   r.LimitPrice,
			FillPrice:    r.FillPrice,
			Quantity:     r.Quantity,
			ErrorMessage: r.ErrorMessage,
		})
	}
	data.Filled = totalFilled
	data.Total = len(rows)

	html, err := templates.RenderBasketSummary(data)
	if err != nil {
		log.Printf("execution: summary: render: %v", err)
		return
	}
	subject := fmt.Sprintf("VibeTradez execution summary — %s (%d/%d filled, $%.2f)", tradeDate, totalFilled, len(rows), data.TotalCost)
	if err := s.mail.SendTradeEmail(s.cfg.EmailFrom, []string{s.cfg.Recipient}, subject, html); err != nil {
		log.Printf("execution: summary: send: %v", err)
		return
	}
	log.Printf("execution: summary sent to %s (%d/%d filled, $%.2f)", s.cfg.Recipient, totalFilled, len(rows), data.TotalCost)
}

/*
SendCloseSummary queries the day's close-side executions and sends ONE
consolidated email to the operator recipient summarizing every
position close attempted today: which filled and at what realized P&L,
which failed and why, and the day's net realized P&L across the
basket. Replaces the previous per-trade close-receipt + close-failed
emails that fired one-per-position throughout the 3:55 ET window.

Empty-result case (no close-side executions at all for the date)
still sends a "no positions closed today" notice so the operator can
distinguish between "executor ran cleanly and had nothing to close"
(zero opens earlier) and "executor failed silently" (opens existed
but no close rows were ever written).

Cron sequence (called by cmd/scanner/main.go after the 3:55 close
cron finishes):
 1. CloseAllPositionsForDate — sells every open position, retries
    once via cancel-replace, records each row's final status
 2. SendCloseSummary — emails the final slate
*/
func (s *Service) SendCloseSummary(ctx context.Context, tradeDate string) {
	_ = ctx // included for parity with SendExecutionSummary; future work may use it
	defer func() {
		if r := recover(); r != nil {
			log.Printf("execution: SendCloseSummary top-level panic: %v", r)
		}
	}()

	rows, err := s.store.CloseSummaryForDate(tradeDate)
	if err != nil {
		log.Printf("execution: close-summary: CloseSummaryForDate(%s): %v", tradeDate, err)
		return
	}

	data := templates.CloseSummaryData{
		Date:               tradeDate,
		Mode:               s.cfg.Mode,
		Rows:               make([]templates.CloseSummaryRow, 0, len(rows)),
		SchwabPositionsURL: schwabPositionsURL,
	}
	totalFilled := 0
	totalFailed := 0
	for _, r := range rows {
		if r.Status == "filled" {
			totalFilled++
			data.TotalRealizedPnL += r.RealizedPnL
		}
		// "failed" / "rejected" are the action-required statuses the
		// email's red callout gates on. "working" and "pending" are
		// transient and rare at 3:55 (the close path polls + retries),
		// but they show up in the body if they exist.
		if r.Status == "failed" || r.Status == "rejected" {
			totalFailed++
		}
		data.Rows = append(data.Rows, templates.CloseSummaryRow{
			Rank:         r.Rank,
			Symbol:       r.Symbol,
			ContractType: r.ContractType,
			StrikePrice:  r.StrikePrice,
			Mode:         r.Mode,
			Status:       r.Status,
			OpenPrice:    r.OpenPrice,
			ClosePrice:   r.ClosePrice,
			RealizedPnL:  r.RealizedPnL,
			Quantity:     r.Quantity,
			ErrorMessage: r.ErrorMessage,
		})
	}
	data.Filled = totalFilled
	data.Failed = totalFailed
	data.Total = len(rows)

	html, err := templates.RenderCloseSummary(data)
	if err != nil {
		log.Printf("execution: close-summary: render: %v", err)
		return
	}
	sign := "+"
	if data.TotalRealizedPnL < 0 {
		sign = ""
	}
	subject := fmt.Sprintf("VibeTradez close summary — %s (%d/%d closed, %s$%.2f P&L)", tradeDate, totalFilled, len(rows), sign, data.TotalRealizedPnL)
	if err := s.mail.SendTradeEmail(s.cfg.EmailFrom, []string{s.cfg.Recipient}, subject, html); err != nil {
		log.Printf("execution: close-summary: send: %v", err)
		return
	}
	log.Printf("execution: close-summary sent to %s (%d/%d closed, %s$%.2f P&L)", s.cfg.Recipient, totalFilled, len(rows), sign, data.TotalRealizedPnL)
}
