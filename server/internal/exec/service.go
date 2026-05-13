package exec

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
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
HandleQualifyingPick is the legacy single-pick entry point. The
morning cron now uses HandleQualifyingPicks (basket of up to
MaxBasketRank picks with cash-check gating); this method is retained
for tests and any external caller that wants the old single-shot
semantics. It is a thin wrapper around handleSinglePick that does
NOT pre-check available funds — callers needing the cash gate must
go through HandleQualifyingPicks.
*/
func (s *Service) HandleQualifyingPick(ctx context.Context, t *trades.Trade, tradeID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.Recipient == "" {
		return errors.New("execution recipient not configured")
	}
	if tradeID == 0 {
		return errors.New("tradeID must be set for auto-execution")
	}
	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		return fmt.Errorf("account hash: %w", err)
	}
	_, err = s.handleSinglePick(ctx, t, tradeID, hash)
	return err
}

/*
HandleQualifyingPicks is the basket entry point fired by the morning
cron with the trades returned by selector.QualifyingPicks. It walks
the picks in rank order (rank-1 first), and for each:

  - resolves the LIMIT price from the live ask (or estimate fallback)
  - checks that the order's worst-case cost (limit price × 100) fits
    BOTH the remaining daily basket budget AND Schwab's reported
    available funds — re-queried on the first contract only, then
    locally decremented per submission since Schwab balance won't
    refresh until fills land
  - submits via handleSinglePick on success, decrements remaining
    budget, moves on; or skips and tries the next rank when the
    contract is too big for either gate

Per-pick errors are logged and don't abort the basket: if rank-1's
ask call fails or its order is rejected, rank-2 still gets its turn
with the local budget intact. Returns the count of contracts the
service successfully submitted (filled, working, or rejected — every
case where Schwab actually saw the order). Caller logs the count;
nothing on the success path requires the number.
*/
func (s *Service) HandleQualifyingPicks(ctx context.Context, picks []trades.Trade) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(picks) == 0 {
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
		double-spending the same cash across contracts. Local budget
		accounting is the source of truth from this point on.
	*/
	availableUSD, err := s.trader.AvailableFunds(ctx, hash)
	if err != nil {
		log.Printf("execution: AvailableFunds lookup failed, skipping basket: %v", err)
		return 0, fmt.Errorf("available funds: %w", err)
	}
	/*
		Two separate budgets:
		  - remainingCashUSD tracks what Schwab will let us spend right
		    now. Both rank-1 and the rank-2/3 tail decrement it. Schwab
		    won't refresh available funds mid-basket because option fills
		    settle T+1, so we decrement locally per submission.
		  - remainingBasketUSD is the daily policy ceiling on the rank-2/3
		    tail. Rank-1 is exempt — it gets to use the full cash pool
		    bounded only by MaxRank1ContractPremium — so a $1000 rank-1
		    doesn't starve the rank-2/3 basket.
	*/
	remainingCashUSD := availableUSD
	remainingBasketUSD := MaxDailyBasketUSD
	log.Printf("execution: basket budget = cash $%.2f, rank-2/3 tail cap $%.2f (rank-1 spends from cash only)", remainingCashUSD, remainingBasketUSD)

	submitted := 0
	for i := range picks {
		t := &picks[i]
		if t.ID == 0 {
			log.Printf("execution: pick rank=%d %s missing trade ID, skipping", t.Rank, t.Symbol)
			continue
		}
		budget := remainingCashUSD
		if t.Rank != 1 && remainingBasketUSD < budget {
			budget = remainingBasketUSD
		}
		costUSD, ok := s.checkBudgetAndPlace(ctx, t, hash, budget)
		if !ok {
			continue
		}
		remainingCashUSD -= costUSD
		if t.Rank != 1 {
			remainingBasketUSD -= costUSD
		}
		submitted++
	}
	log.Printf("execution: basket complete, submitted %d/%d contracts, remaining cash $%.2f, remaining rank-2/3 tail $%.2f", submitted, len(picks), remainingCashUSD, remainingBasketUSD)
	return submitted, nil
}

/*
checkBudgetAndPlace resolves the LIMIT price and submits the order
when it fits remainingUSD. Returns (cost, true) on submit (so the
caller can decrement its local budget) or (0, false) when the pick
was skipped — either because pricing failed (already alerted via
sendOpenFailedEmail) or because the contract's cost exceeded the
remaining budget. Errors after submission are logged inside
handleSinglePick; this function only filters by cost.
*/
func (s *Service) checkBudgetAndPlace(ctx context.Context, t *trades.Trade, hash string, remainingUSD float64) (float64, bool) {
	cap := PerContractPremiumCap(t.Rank)
	limitPrice, err := s.resolveLimitPrice(ctx, t, cap)
	if err != nil {
		occ, _ := OCCSymbol(t.Symbol, t.Expiration, t.ContractType, t.StrikePrice)
		log.Printf("execution: rank=%d %s pricing failed: %v", t.Rank, t.Symbol, err)
		s.sendOpenFailedEmail(t, occ, "", err.Error())
		return 0, false
	}
	costUSD := limitPrice * 100 * float64(MaxContracts)
	if costUSD > remainingUSD {
		log.Printf("execution: rank=%d %s skipped, cost $%.2f exceeds remaining budget $%.2f", t.Rank, t.Symbol, costUSD, remainingUSD)
		return 0, false
	}
	if _, err := s.handleSinglePick(ctx, t, t.ID, hash); err != nil {
		log.Printf("execution: rank=%d %s submit failed: %v", t.Rank, t.Symbol, err)
		// Even on submission failure, the contract reached the broker
		// (or tried to) — count the cost against the budget so a
		// retry storm can't blow through the daily cap.
		return costUSD, true
	}
	return costUSD, true
}

/*
resolveLimitPrice fetches the live ask and returns the LIMIT price
the open order should carry, falling back to Claude's modeled premium
when the live quote is missing. maxCap is the per-share ceiling that
already passed the selector gate (caller threads
PerContractPremiumCap(rank) here). Returns an error only when
neither basis is usable — caller treats that as "do not submit".
*/
func (s *Service) resolveLimitPrice(ctx context.Context, t *trades.Trade, maxCap float64) (float64, error) {
	var askPrice float64
	if s.cfg.OptionAsk != nil {
		fetched, askErr := s.cfg.OptionAsk(ctx, t.Symbol, t.Expiration, t.ContractType, t.StrikePrice)
		if askErr != nil {
			log.Printf("execution: option ask unavailable, falling back to estimate (symbol=%s err=%v)", t.Symbol, askErr)
		} else {
			askPrice = fetched
		}
	}
	limitPrice := ComputeOpenLimitPrice(askPrice, t.EstimatedPrice, maxCap)
	if limitPrice <= 0 {
		return 0, fmt.Errorf("could not resolve a LIMIT price (ask=%.2f, est=%.2f)", askPrice, t.EstimatedPrice)
	}
	return limitPrice, nil
}

/*
handleSinglePick is the per-contract submission body lifted out of
the old HandleQualifyingPick so HandleQualifyingPicks can call it in
a loop without re-doing the account-hash lookup. Returns the order
id from Schwab on success (empty string on early exit). Errors are
non-nil for "did not reach broker" cases AND for downstream lookup
failures; the caller decides whether to keep going.
*/
func (s *Service) handleSinglePick(ctx context.Context, t *trades.Trade, tradeID int, hash string) (string, error) {
	occ, err := OCCSymbol(t.Symbol, t.Expiration, t.ContractType, t.StrikePrice)
	if err != nil {
		return "", fmt.Errorf("build OCC symbol: %w", err)
	}

	limitPrice, err := s.resolveLimitPrice(ctx, t, PerContractPremiumCap(t.Rank))
	if err != nil {
		log.Printf("execution: %s", err.Error())
		s.sendOpenFailedEmail(t, occ, "", err.Error())
		return "", err
	}

	order, err := BuildOpenOrderForTrade(t, occ, limitPrice)
	if err != nil {
		return "", fmt.Errorf("build open order: %w", err)
	}

	execRow := Execution{
		TradeID:           tradeID,
		Mode:              s.cfg.Mode,
		Side:              "open",
		Status:            "pending",
		RequestedQuantity: MaxContracts,
	}
	execID, err := s.store.InsertExecution(execRow)
	if err != nil {
		return "", fmt.Errorf("insert open execution: %w", err)
	}

	orderID, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", "", nil, 0, err.Error())
		s.sendOpenFailedEmail(t, occ, "", err.Error())
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

	st, err := s.trader.GetOrder(ctx, hash, orderID)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", orderID, nil, 0, err.Error())
		s.sendOpenFailedEmail(t, occ, orderID, err.Error())
		return orderID, fmt.Errorf("get open order status: %w", err)
	}

	switch {
	case st.Filled:
		fp := st.FillPrice
		_ = s.store.UpdateExecutionStatus(execID, "filled", orderID, &fp, st.FilledQuantity, "")
		s.sendReceiptEmail(t, occ, orderID, st.FillPrice)
		log.Printf("execution: open filled (trade_id=%d, mode=%s, fill=%.2f, order=%s)", tradeID, s.cfg.Mode, st.FillPrice, orderID)
	case st.Terminal:
		// Terminal-but-not-filled: REJECTED, CANCELED, EXPIRED, REPLACED.
		// Persist the broker's reason and alert the operator — silent
		// rejection is how three days of bad orders went unnoticed.
		reason := st.ErrorMessage
		if reason == "" {
			reason = st.RawStatus
		}
		_ = s.store.UpdateExecutionStatus(execID, "rejected", orderID, nil, 0, reason)
		s.sendOpenFailedEmail(t, occ, orderID, reason)
		log.Printf("execution: open order rejected (trade_id=%d, order=%s, status=%s, reason=%q)", tradeID, orderID, st.RawStatus, reason)
	default:
		_ = s.store.UpdateExecutionStatus(execID, "working", orderID, nil, 0, "")
		log.Printf("execution: open order working (trade_id=%d, order=%s, status=%s)", tradeID, orderID, st.RawStatus)
	}
	return orderID, nil
}

/*
RepriceWorkingOpens is the one-shot post-open cron entry point that
walks every still-WORKING open order and, when the post-open ask has
moved enough to leave our original LIMIT underwater, cancel-and-
replaces at a fresh-ask-derived limit. The morning cron's order
limits are sized from pre-open Schwab quotes (often 0-ask on thin
contracts, in which case ComputeOpenLimitPrice falls back to Claude's
modeled premium), so a real-open jump or a Claude mispricing leaves
orders stranded WORKING for the rest of the day until the broker
auto-expires them at 16:00 ET. This pass closes that gap.

Decision tree per working position:

  - GetOrder at the broker first. If it has already filled or
    terminally rejected since the last reconcile tick, do nothing —
    the per-minute reconcile cron will pick up the new state on its
    next pass.
  - Fetch a fresh ask via cfg.OptionAsk. If unavailable, leave the
    order alone (the original limit might still fill on a pullback).
  - newLimit = round(freshAsk × LimitPriceMultiplier, cents).
  - If newLimit ≤ existing broker-side LIMIT (within rounding), leave
    the order alone. Cancel+replace at the same price is pure round-
    trip latency for zero benefit, plus a thin no-order-at-broker
    window in which a fast spike could miss.
  - If newLimit > MaxContractPremium, the post-open premium has run
    past the single-contract cap. Cancel the working order, mark the
    execution 'canceled', and email the operator. The pick is dead
    for the day.
  - Otherwise cancel the old order, mark the original execution row
    'canceled' with a "repriced post-open" reason, insert a new
    execution row for the replacement, and place a new LIMIT order at
    newLimit. The new row carries the same trade_id so the dashboard
    open-position join still points at the live order.

Errors are logged but never propagate — one bad pick must not block
the rest of the basket. Wrapped in s.mu so concurrent reconcile ticks
serialize behind us.
*/
func (s *Service) RepriceWorkingOpens(ctx context.Context, tradeDate string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("execution: RepriceWorkingOpens top-level panic: %v", r)
		}
	}()

	if s.cfg.OptionAsk == nil {
		return
	}

	positions, err := s.store.WorkingOpenPositionsForDate(tradeDate)
	if err != nil {
		log.Printf("execution: reprice: working positions: %v", err)
		return
	}
	if len(positions) == 0 {
		return
	}

	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		log.Printf("execution: reprice: account hash: %v", err)
		return
	}

	log.Printf("execution: reprice pass for %d working open(s)", len(positions))
	for i := range positions {
		s.repriceOne(ctx, hash, &positions[i])
	}
}

func (s *Service) repriceOne(ctx context.Context, hash string, p *OpenPosition) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("execution: repriceOne panic for trade %d: %v", p.Execution.TradeID, r)
		}
	}()

	if p.Execution.SchwabOrderID == nil || *p.Execution.SchwabOrderID == "" {
		return
	}
	oldOrderID := *p.Execution.SchwabOrderID

	st, err := s.trader.GetOrder(ctx, hash, oldOrderID)
	if err != nil {
		log.Printf("execution: reprice GetOrder (trade=%d, order=%s): %v", p.Execution.TradeID, oldOrderID, err)
		return
	}
	if !st.Working {
		// Already filled, canceled, or terminally rejected. The reconcile
		// cron owns the DB flip; reprice does nothing.
		return
	}
	oldLimit := st.LimitPrice

	freshAsk, err := s.cfg.OptionAsk(ctx, p.Symbol, p.Expiration, p.ContractType, p.StrikePrice)
	if err != nil {
		log.Printf("execution: reprice ask lookup failed (trade=%d, %s %s %.2f): %v", p.Execution.TradeID, p.Symbol, p.ContractType, p.StrikePrice, err)
		return
	}
	if freshAsk <= 0 {
		log.Printf("execution: reprice: fresh ask is %.2f, leaving order alone (trade=%d)", freshAsk, p.Execution.TradeID)
		return
	}

	newLimit := math.Round(freshAsk*LimitPriceMultiplier*100) / 100
	cap := PerContractPremiumCap(p.Rank)

	if oldLimit > 0 && newLimit <= oldLimit {
		log.Printf("execution: reprice no-op (trade=%d, rank=%d, old_limit=$%.2f, fresh_ask=$%.2f, new_limit=$%.2f)",
			p.Execution.TradeID, p.Rank, oldLimit, freshAsk, newLimit)
		return
	}

	syntheticTrade := &trades.Trade{
		Symbol:         p.Symbol,
		ContractType:   p.ContractType,
		StrikePrice:    p.StrikePrice,
		Expiration:     p.Expiration,
		EstimatedPrice: p.ContractPrice,
		Rank:           p.Rank,
	}
	occ, _ := OCCSymbol(p.Symbol, p.Expiration, p.ContractType, p.StrikePrice)

	if newLimit > cap {
		reason := fmt.Sprintf("post-open ask $%.2f × %.2f = $%.2f exceeds rank-%d cap $%.2f", freshAsk, LimitPriceMultiplier, newLimit, p.Rank, cap)
		if cancelErr := s.trader.CancelOrder(ctx, hash, oldOrderID); cancelErr != nil {
			log.Printf("execution: reprice cancel failed (trade=%d, order=%s): %v", p.Execution.TradeID, oldOrderID, cancelErr)
			// Continue — record the canceled state in our DB and notify.
			// If the broker rejected the cancel because the order already
			// filled, the next reconcile tick will correct to 'filled'.
		}
		_ = s.store.UpdateExecutionStatus(p.Execution.ID, "canceled", oldOrderID, nil, 0, reason)
		s.sendOpenFailedEmail(syntheticTrade, occ, oldOrderID, reason)
		log.Printf("execution: reprice canceled — over cap (trade=%d, order=%s, %s)", p.Execution.TradeID, oldOrderID, reason)
		return
	}

	if cancelErr := s.trader.CancelOrder(ctx, hash, oldOrderID); cancelErr != nil {
		log.Printf("execution: reprice cancel failed, leaving original order in place (trade=%d, order=%s): %v", p.Execution.TradeID, oldOrderID, cancelErr)
		return
	}
	_ = s.store.UpdateExecutionStatus(p.Execution.ID, "canceled", oldOrderID, nil, 0, fmt.Sprintf("repriced post-open from $%.2f to $%.2f", oldLimit, newLimit))

	order, err := BuildOpenOrderForTrade(syntheticTrade, occ, newLimit)
	if err != nil {
		log.Printf("execution: reprice build order (trade=%d): %v", p.Execution.TradeID, err)
		return
	}

	newExecRow := Execution{
		TradeID:           p.Execution.TradeID,
		Mode:              s.cfg.Mode,
		Side:              "open",
		Status:            "pending",
		RequestedQuantity: MaxContracts,
	}
	newExecID, err := s.store.InsertExecution(newExecRow)
	if err != nil {
		log.Printf("execution: reprice insert new execution (trade=%d): %v", p.Execution.TradeID, err)
		return
	}

	newOrderID, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(newExecID, "failed", "", nil, 0, err.Error())
		s.sendOpenFailedEmail(syntheticTrade, occ, "", err.Error())
		log.Printf("execution: reprice replace PlaceOrder failed (trade=%d): %v", p.Execution.TradeID, err)
		return
	}
	_ = s.store.UpdateExecutionStatus(newExecID, "working", newOrderID, nil, 0, "")

	if newSt, sErr := s.trader.GetOrder(ctx, hash, newOrderID); sErr == nil {
		switch {
		case newSt.Filled:
			fp := newSt.FillPrice
			_ = s.store.UpdateExecutionStatus(newExecID, "filled", newOrderID, &fp, newSt.FilledQuantity, "")
			s.sendReceiptEmail(syntheticTrade, occ, newOrderID, newSt.FillPrice)
		case newSt.Terminal:
			reason := newSt.ErrorMessage
			if reason == "" {
				reason = newSt.RawStatus
			}
			_ = s.store.UpdateExecutionStatus(newExecID, "rejected", newOrderID, nil, 0, reason)
			s.sendOpenFailedEmail(syntheticTrade, occ, newOrderID, reason)
		}
	}

	log.Printf("execution: reprice replaced (trade=%d, old=%s @$%.2f → new=%s @$%.2f, fresh_ask=$%.2f)",
		p.Execution.TradeID, oldOrderID, oldLimit, newOrderID, newLimit, freshAsk)
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
email that HandleQualifyingPick would have sent on a same-tick fill.
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
	syntheticTrade := &trades.Trade{
		Symbol:         p.Symbol,
		ContractType:   p.ContractType,
		StrikePrice:    p.StrikePrice,
		Expiration:     p.Expiration,
		EstimatedPrice: p.ContractPrice,
	}

	switch {
	case st.Filled:
		fp := st.FillPrice
		_ = s.store.UpdateExecutionStatus(p.Execution.ID, "filled", orderID, &fp, st.FilledQuantity, "")
		s.sendReceiptEmail(syntheticTrade, occ, orderID, st.FillPrice)
		log.Printf("execution: reconcile open filled (trade=%d, mode=%s, fill=%.2f, order=%s)", p.Execution.TradeID, p.Execution.Mode, st.FillPrice, orderID)
	case st.Terminal:
		reason := st.ErrorMessage
		if reason == "" {
			reason = st.RawStatus
		}
		_ = s.store.UpdateExecutionStatus(p.Execution.ID, "rejected", orderID, nil, 0, reason)
		s.sendOpenFailedEmail(syntheticTrade, occ, orderID, reason)
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

	order, err := BuildCloseOrderForPosition(p)
	if err != nil {
		s.sendCloseFailedEmail(p, fmt.Sprintf("build close order: %v", err))
		return
	}

	execRow := Execution{
		TradeID:           p.Execution.TradeID,
		Mode:              s.cfg.Mode,
		Side:              "close",
		Status:            "pending",
		RequestedQuantity: MaxContracts,
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
	realized := (st.FillPrice - openPrice) * 100 * float64(MaxContracts)

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

func (s *Service) sendReceiptEmail(t *trades.Trade, occSymbol, orderID string, fillPrice float64) {
	data := templates.ExecuteReceiptData{
		Subject:            fmt.Sprintf("[%s] Order filled: %s %s @ $%.2f", strings.ToUpper(s.cfg.Mode), t.Symbol, t.ContractType, fillPrice),
		Date:               time.Now().In(easternTime()).Format("Monday, Jan 2 (3:04 PM ET)"),
		Mode:               s.cfg.Mode,
		Symbol:             t.Symbol,
		ContractType:       t.ContractType,
		StrikePrice:        t.StrikePrice,
		Expiration:         t.Expiration,
		OCCSymbol:          occSymbol,
		FillPrice:          fillPrice,
		Quantity:           MaxContracts,
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
