package exec

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
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
	/*
		OptionAsk returns the current ask quote for an option contract.
		Called at order-submission time to size the morning open LIMIT
		price. Optional — when nil, the limit falls back to the trade's
		EstimatedPrice (Claude's modeled premium at pick time). Live
		mode wires this to schwab.Client.OptionAsk; tests can pass a
		stub.
	*/
	OptionAsk func(ctx context.Context, symbol, expiration, contractType string, strike float64) (float64, error)
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
HandleQualifyingPicks is the basket entry point fired by the morning
cron with the basket returned by selector.QualifyingBasket. Each
BasketEntry carries a (trade, quantity) pair; the selector has
already merged duplicates so the executor fires exactly one
quantity=N order per entry. Entries arrive in rank-ascending order.

For each entry:

  - resolves the LIMIT price from the live ask (or estimate fallback)
  - checks the entry's full cost (limit price × 100 × quantity) fits
    in the remaining Schwab cash (polled once at basket open and
    decremented locally per submission, since Schwab balance won't
    refresh until fills settle T+1)
  - submits the order via handleSingleEntry, decrements remaining
    cash, moves on; or skips when the entry exceeds remaining cash

The selector already enforced MaxDailyBasketUSD for phase-2 fills,
so the only governor on this layer is Schwab cash availability —
phase 1 (top 3) is guaranteed by the selector and will fire here as
long as the account has cash to cover it. Per-entry errors are
logged and don't abort the basket: rank-1's pricing failure can't
prevent rank-2 from firing.

Returns the count of orders the service successfully submitted
(filled, working, or rejected — every case where Schwab actually
saw the order). Caller logs the count.
*/
func (s *Service) HandleQualifyingPicks(ctx context.Context, basket []BasketEntry) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(basket) == 0 {
		return 0, nil
	}
	if s.cfg.Recipient == "" {
		return 0, errors.New("execution recipient not configured")
	}
	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		return 0, fmt.Errorf("account hash: %w", err)
	}

	/*
		Poll Schwab for live cash exactly once at basket open. The
		broker won't decrement available funds until a previous order
		actually fills (and for options that's typically T+1 settlement
		anyway), so re-querying mid-basket would over-report and risk
		double-spending the same cash across contracts. Local cash
		accounting is the source of truth from this point on.
	*/
	availableUSD, err := s.trader.AvailableFunds(ctx, hash)
	if err != nil {
		log.Printf("execution: AvailableFunds lookup failed, skipping basket: %v", err)
		return 0, fmt.Errorf("available funds: %w", err)
	}
	log.Printf("execution: basket has %d entries, Schwab available = $%.2f, daily fill target = $%.2f", len(basket), availableUSD, MaxDailyBasketUSD)

	remainingCash := availableUSD
	submitted := 0
	for i := range basket {
		entry := &basket[i]
		t := &entry.Trade
		if t.ID == 0 {
			log.Printf("execution: rank=%d %s qty=%d missing trade ID, skipping", t.Rank, t.Symbol, entry.Quantity)
			continue
		}
		costUSD, ok := s.checkCashAndPlace(ctx, entry, hash, remainingCash)
		if !ok {
			continue
		}
		remainingCash -= costUSD
		submitted++
	}
	log.Printf("execution: basket complete, submitted %d/%d orders, remaining cash $%.2f", submitted, len(basket), remainingCash)
	return submitted, nil
}

/*
checkCashAndPlace resolves the LIMIT price for one BasketEntry and
submits the order when its full cost (price × 100 × quantity) fits
in remainingCash. Returns (cost, true) on submit (so the caller can
decrement its local cash counter) or (0, false) when the entry was
skipped because pricing failed (already alerted via
sendOpenFailedEmail) or because cost exceeded remaining cash.
Errors after submission are logged inside handleSingleEntry; this
function only filters by cost.
*/
func (s *Service) checkCashAndPlace(ctx context.Context, entry *BasketEntry, hash string, remainingCash float64) (float64, bool) {
	t := &entry.Trade
	limitPrice, err := s.resolveLimitPrice(ctx, t)
	if err != nil {
		occ, _ := OCCSymbol(t.Symbol, t.Expiration, t.ContractType, t.StrikePrice)
		log.Printf("execution: rank=%d %s pricing failed: %v", t.Rank, t.Symbol, err)
		// Per-execution email removed; the 9:35 SendExecutionSummary cron
		// emits one consolidated email covering every open-side outcome
		// for the day. Until then this is a log-only event.
		_ = occ
		return 0, false
	}
	costUSD := limitPrice * 100 * float64(entry.Quantity)
	if costUSD > remainingCash {
		log.Printf("execution: rank=%d %s qty=%d skipped, cost $%.2f exceeds remaining cash $%.2f", t.Rank, t.Symbol, entry.Quantity, costUSD, remainingCash)
		return 0, false
	}
	if _, err := s.handleSingleEntry(ctx, entry, t.ID, hash); err != nil {
		log.Printf("execution: rank=%d %s qty=%d submit failed: %v", t.Rank, t.Symbol, entry.Quantity, err)
		// Even on submission failure, the contract reached the broker
		// (or tried to) — count the cost against remaining cash so a
		// retry storm can't blow through the account.
		return costUSD, true
	}
	return costUSD, true
}

/*
resolveLimitPrice fetches the live ask and returns the LIMIT price
the open order should carry, falling back to Claude's modeled premium
when the live quote is missing. Returns an error only when neither
basis is usable — caller treats that as "do not submit".
*/
func (s *Service) resolveLimitPrice(ctx context.Context, t *trades.Trade) (float64, error) {
	var askPrice float64
	if s.cfg.OptionAsk != nil {
		fetched, askErr := s.cfg.OptionAsk(ctx, t.Symbol, t.Expiration, t.ContractType, t.StrikePrice)
		if askErr != nil {
			log.Printf("execution: option ask unavailable, falling back to estimate (symbol=%s err=%v)", t.Symbol, askErr)
		} else {
			askPrice = fetched
		}
	}
	limitPrice := ComputeOpenLimitPrice(askPrice, t.EstimatedPrice)
	if limitPrice <= 0 {
		return 0, fmt.Errorf("could not resolve a LIMIT price (ask=%.2f, est=%.2f)", askPrice, t.EstimatedPrice)
	}
	return limitPrice, nil
}

/*
handleSingleEntry is the per-entry submission body that
HandleQualifyingPicks calls in a loop. Submits one quantity=N order
for the given BasketEntry. Returns the order id from Schwab on
success (empty string on early exit). Errors are non-nil for "did
not reach broker" cases AND for downstream lookup failures; the
caller decides whether to keep going.
*/
func (s *Service) handleSingleEntry(ctx context.Context, entry *BasketEntry, tradeID int, hash string) (string, error) {
	t := &entry.Trade
	occ, err := OCCSymbol(t.Symbol, t.Expiration, t.ContractType, t.StrikePrice)
	if err != nil {
		return "", fmt.Errorf("build OCC symbol: %w", err)
	}

	limitPrice, err := s.resolveLimitPrice(ctx, t)
	if err != nil {
		log.Printf("execution: %s", err.Error())
		// Per-execution email removed; the 9:35 SendExecutionSummary cron
		// emits one consolidated email covering every open-side outcome
		// for the day. Until then this is a log-only event.
		_ = occ
		return "", err
	}

	order, err := BuildOpenOrderForTrade(t, occ, limitPrice, entry.Quantity)
	if err != nil {
		return "", fmt.Errorf("build open order: %w", err)
	}

	execRow := Execution{
		TradeID:           tradeID,
		Mode:              s.cfg.Mode,
		Side:              "open",
		Status:            "pending",
		RequestedQuantity: entry.Quantity,
	}
	execID, err := s.store.InsertExecution(execRow)
	if err != nil {
		return "", fmt.Errorf("insert open execution: %w", err)
	}

	orderID, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", "", nil, 0, err.Error())
		// Per-execution email removed; the 9:35 SendExecutionSummary cron
		// emits one consolidated email covering every open-side outcome
		// for the day. Until then this is a log-only event.
		_ = occ
		return "", fmt.Errorf("place open order: %w", err)
	}

	/*
		Persist the broker order id IMMEDIATELY after PlaceOrder returns,
		before any further work. If the process crashes between PlaceOrder
		succeeding at the broker and the GetOrder status write below, the
		order is live at Schwab but used to be invisible to us, no
		reconcile, no kill-switch, no close. Flipping the row to
		'working' with the order id makes the per-minute reconcile cron
		pick it up on the next tick if anything downstream fails.
	*/
	if err := s.store.UpdateExecutionStatus(execID, "working", orderID, nil, 0, ""); err != nil {
		log.Printf("execution: warning: failed to persist orderID mid-flight (trade_id=%d, order=%s): %v", tradeID, orderID, err)
	}

	// Per-execution emails removed across this path; the consolidated
	// SendExecutionSummary at 9:35 ET emits one operator email
	// covering every open-side outcome for the day.
	_ = occ
	st, err := s.trader.GetOrder(ctx, hash, orderID)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", orderID, nil, 0, err.Error())
		return orderID, fmt.Errorf("get open order status: %w", err)
	}

	switch {
	case st.Filled:
		fp := st.FillPrice
		_ = s.store.UpdateExecutionStatus(execID, "filled", orderID, &fp, st.FilledQuantity, "")
		log.Printf("execution: open filled (trade_id=%d, qty=%d, mode=%s, fill=%.2f, order=%s)", tradeID, entry.Quantity, s.cfg.Mode, st.FillPrice, orderID)
	case st.Terminal:
		// Terminal-but-not-filled: REJECTED, CANCELED, EXPIRED, REPLACED.
		// Persist the broker's reason and let the 9:35 summary email
		// surface it; silent rejection is how three days of bad orders
		// went unnoticed historically, so the DB row itself carries the
		// reason string.
		reason := st.ErrorMessage
		if reason == "" {
			reason = st.RawStatus
		}
		_ = s.store.UpdateExecutionStatus(execID, "rejected", orderID, nil, 0, reason)
		log.Printf("execution: open order rejected (trade_id=%d, qty=%d, order=%s, status=%s, reason=%q)", tradeID, entry.Quantity, orderID, st.RawStatus, reason)
	default:
		_ = s.store.UpdateExecutionStatus(execID, "working", orderID, nil, 0, "")
		log.Printf("execution: open order working (trade_id=%d, qty=%d, order=%s, status=%s)", tradeID, entry.Quantity, orderID, st.RawStatus)
	}
	return orderID, nil
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
		s.sendCloseFailedEmail(p, fmt.Sprintf("account hash lookup failed: %v", err))
		return
	}

	closeQty := positionCloseQuantity(p)
	order, err := BuildCloseOrderForPosition(p, closeQty)
	if err != nil {
		s.sendCloseFailedEmail(p, fmt.Sprintf("build close order: %v", err))
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
		log.Printf("execution: insert close row: %v", err)
		s.sendCloseFailedEmail(p, fmt.Sprintf("insert close row: %v", err))
		return
	}

	orderID, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", "", nil, 0, err.Error())
		s.sendCloseFailedEmail(p, fmt.Sprintf("first PlaceOrder: %v", err))
		return
	}
	// Same orphan-prevention rationale as the open path: persist the
	// broker order id immediately so a crash before pollFilled returns
	// leaves a 'working' row the reconcile cron can finish.
	if err := s.store.UpdateExecutionStatus(execID, "working", orderID, nil, 0, ""); err != nil {
		log.Printf("execution: warning: failed to persist close orderID mid-flight (trade_id=%d, order=%s): %v", p.Execution.TradeID, orderID, err)
	}
	if s.pollFilled(ctx, hash, orderID, 8, 15*time.Second) {
		s.recordCloseAndEmail(ctx, p, execID, hash, orderID)
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
			s.recordCloseAndEmail(ctx, p, execID, hash, orderID)
			return
		}
		if postCancel.Terminal && postCancel.RawStatus != "CANCELED" && postCancel.RawStatus != "REPLACED" {
			_ = s.store.UpdateExecutionStatus(execID, "failed", orderID, nil, 0, fmt.Sprintf("close terminal mid-cancel as %s; cancel-replace skipped", postCancel.RawStatus))
			s.sendCloseFailedEmail(p, fmt.Sprintf("first close order ended as %s mid-cancel; manual review", postCancel.RawStatus))
			return
		}
	}

	orderID2, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", orderID, nil, 0, "cancel-replace failed: "+err.Error())
		s.sendCloseFailedEmail(p, fmt.Sprintf("cancel-replace PlaceOrder: %v", err))
		return
	}
	// Persist the replacement order id immediately, same rationale.
	if err := s.store.UpdateExecutionStatus(execID, "working", orderID2, nil, 0, ""); err != nil {
		log.Printf("execution: warning: failed to persist cancel-replace orderID mid-flight (trade_id=%d, order=%s): %v", p.Execution.TradeID, orderID2, err)
	}
	if s.pollFilled(ctx, hash, orderID2, 8, 15*time.Second) {
		s.recordCloseAndEmail(ctx, p, execID, hash, orderID2)
		return
	}

	_ = s.store.UpdateExecutionStatus(execID, "failed", orderID2, nil, 0, "unfilled after retry-cancel-replace")
	s.sendCloseFailedEmail(p, "Position did not fill within 4-minute retry-cancel-replace window. Close on Schwab manually before 4:00pm ET.")
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

func (s *Service) recordCloseAndEmail(ctx context.Context, p *OpenPosition, execID int, hash, orderID string) {
	st, err := s.trader.GetOrder(ctx, hash, orderID)
	if err != nil {
		log.Printf("execution: post-fill GetOrder: %v", err)
		return
	}
	fp := st.FillPrice
	_ = s.store.UpdateExecutionStatus(execID, "filled", orderID, &fp, st.FilledQuantity, "")

	openPrice := p.ContractPrice
	open, err := s.store.OpenExecutionForTrade(p.Execution.TradeID)
	if err == nil && open != nil && open.FillPrice != nil {
		openPrice = *open.FillPrice
	}
	// Use the broker's filled_quantity if Schwab reported one (qty=N
	// closes can partially fill in theory), otherwise fall back to the
	// quantity we requested. The open and close quantities will match
	// for any clean lifecycle.
	closedQty := st.FilledQuantity
	if closedQty < 1 {
		closedQty = positionCloseQuantity(p)
	}
	realized := (st.FillPrice - openPrice) * 100 * float64(closedQty)

	data := templates.ExecuteCloseReceiptData{
		Subject:            fmt.Sprintf("[%s] Position closed: %s %s, P&L $%.2f", strings.ToUpper(s.cfg.Mode), p.Symbol, p.ContractType, realized),
		Date:               time.Now().In(easternTime()).Format("Monday, Jan 2 (3:04 PM ET)"),
		Mode:               s.cfg.Mode,
		Symbol:             p.Symbol,
		ContractType:       p.ContractType,
		StrikePrice:        p.StrikePrice,
		Expiration:         p.Expiration,
		OpenPrice:          openPrice,
		ClosePrice:         st.FillPrice,
		RealizedPnL:        realized,
		SchwabPositionsURL: schwabPositionsURL,
	}
	html, err := templates.RenderExecuteCloseReceipt(data)
	if err != nil {
		log.Printf("execution: render close receipt: %v", err)
		return
	}
	if err := s.mail.SendTradeEmail(s.cfg.EmailFrom, []string{s.cfg.Recipient}, data.Subject, html); err != nil {
		log.Printf("execution: send close receipt: %v", err)
	}
}

func (s *Service) sendReceiptEmail(t *trades.Trade, occSymbol, orderID string, fillPrice float64, quantity int) {
	if quantity < 1 {
		quantity = 1
	}
	data := templates.ExecuteReceiptData{
		Subject:            fmt.Sprintf("[%s] Order filled: %s %s ×%d @ $%.2f", strings.ToUpper(s.cfg.Mode), t.Symbol, t.ContractType, quantity, fillPrice),
		Date:               time.Now().In(easternTime()).Format("Monday, Jan 2 (3:04 PM ET)"),
		Mode:               s.cfg.Mode,
		Symbol:             t.Symbol,
		ContractType:       t.ContractType,
		StrikePrice:        t.StrikePrice,
		Expiration:         t.Expiration,
		OCCSymbol:          occSymbol,
		FillPrice:          fillPrice,
		Quantity:           quantity,
		OrderID:            orderID,
		SchwabPositionsURL: schwabPositionsURL,
	}
	html, err := templates.RenderExecuteReceipt(data)
	if err != nil {
		log.Printf("execution: render receipt: %v", err)
		return
	}
	if err := s.mail.SendTradeEmail(s.cfg.EmailFrom, []string{s.cfg.Recipient}, data.Subject, html); err != nil {
		log.Printf("execution: send receipt: %v", err)
	}
}

/*
sendOpenFailedEmail alerts the operator when the morning open order
either errored at submission, errored on status lookup, or came back
in a terminal-but-not-filled state (REJECTED / CANCELED / EXPIRED).
Goes ONLY to cfg.Recipient — never the subscriber list — so live-mode
broker rejections don't leak to subscribers as a "we tried, it didn't
work" message. Best-effort: render or send failures are logged and
swallowed so the morning pipeline keeps running.
*/
func (s *Service) sendOpenFailedEmail(t *trades.Trade, occSymbol, orderID, errMsg string) {
	if s.cfg.Recipient == "" {
		return
	}
	data := templates.ExecuteOpenFailedData{
		Subject:            fmt.Sprintf("[%s] Open failed: %s %s", strings.ToUpper(s.cfg.Mode), t.Symbol, t.ContractType),
		Date:               time.Now().In(easternTime()).Format("Monday, Jan 2 (3:04 PM ET)"),
		Mode:               s.cfg.Mode,
		Symbol:             t.Symbol,
		ContractType:       t.ContractType,
		StrikePrice:        t.StrikePrice,
		Expiration:         t.Expiration,
		OCCSymbol:          occSymbol,
		OrderID:            orderID,
		ErrorMessage:       errMsg,
		SchwabPositionsURL: schwabPositionsURL,
	}
	html, err := templates.RenderExecuteOpenFailed(data)
	if err != nil {
		log.Printf("execution: render open-failed email: %v", err)
		return
	}
	if err := s.mail.SendTradeEmail(s.cfg.EmailFrom, []string{s.cfg.Recipient}, data.Subject, html); err != nil {
		log.Printf("execution: send open-failed email: %v", err)
	}
}

func (s *Service) sendCloseFailedEmail(p *OpenPosition, errMsg string) {
	occ, _ := OCCSymbol(p.Symbol, p.Expiration, p.ContractType, p.StrikePrice)
	data := templates.ExecuteCloseFailedData{
		Subject:            fmt.Sprintf("[ACTION REQUIRED] vibetradez close failed: %s", p.Symbol),
		Date:               time.Now().In(easternTime()).Format("Monday, Jan 2 (3:04 PM ET)"),
		Symbol:             p.Symbol,
		ContractType:       p.ContractType,
		StrikePrice:        p.StrikePrice,
		Expiration:         p.Expiration,
		OCCSymbol:          occ,
		ErrorMessage:       errMsg,
		SchwabPositionsURL: schwabPositionsURL,
	}
	html, err := templates.RenderExecuteCloseFailed(data)
	if err != nil {
		log.Printf("execution: render close-failed email: %v", err)
		return
	}
	if err := s.mail.SendTradeEmail(s.cfg.EmailFrom, []string{s.cfg.Recipient}, data.Subject, html); err != nil {
		log.Printf("execution: send close-failed email: %v", err)
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
easternTime returns the ET location for date formatting. Falls back
to UTC if the zone db isn't available (extremely unlikely).
*/
func easternTime() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.UTC
	}
	return loc
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
