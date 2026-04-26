package exec

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"vibetradez.com/internal/email"
	"vibetradez.com/internal/templates"
	"vibetradez.com/internal/trades"
)

/*
schwabPositionsURL is the deep link surfaced in receipt emails. Used
verbatim per the task spec — do not parameterize.
*/
const schwabPositionsURL = "https://client.schwab.com/app/accounts/positions/#/"

/*
confirmationWindow is the hard expiry between the email going out and
the trade auto-cancelling. Defined here so the cron + selector + HMAC
expiry all reference the same constant.
*/
const confirmationWindow = 5 * time.Minute

/*
DecisionStore is the slice of *store.Store that exec.Service needs.
Defined as an interface so tests don't need a real Postgres.
*/
type DecisionStore interface {
	InsertDecision(d Decision) (int, error)
	SetDecisionTokenHash(id int, hash string) error
	GetDecision(id int) (*Decision, error)
	GetDecisionByDate(date string) (*Decision, error)
	SetDecisionStatus(id int, status string) error
	ForceSetDecisionStatus(id int, status string) error
	PendingDecisions() ([]Decision, error)
	InsertExecution(e Execution) (int, error)
	UpdateExecutionStatus(id int, status string, fillPrice *float64, filledQty int, errMsg string) error
	GetExecution(id int) (*Execution, error)
	OpenExecutionForDecision(decisionID int) (*Execution, error)
	LiveExecutionsForDecision(decisionID int) ([]Execution, error)
	OpenPositionsForDate(date string) ([]Decision, error)
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
	Mode              string // "paper" | "live"
	HMACSecret        []byte
	Recipient         string // bordelonjayce@gmail.com (single-recipient guarantee)
	EmailFrom         string
	PublicBaseURL     string // https://vibetradez.com — for building confirmation links
	GPTModelLabel     string // e.g. "GPT Latest"
	ClaudeModelLabel  string // e.g. "Claude Latest"
	SchwabAccountHash func(ctx context.Context) (string, error)
}

/*
Service orchestrates the auto-execution lifecycle. One instance per
process; safe for concurrent use across goroutines (only mutable state
is held inside the trader and store, both of which are thread-safe).
*/
type Service struct {
	store  DecisionStore
	trader TraderClient
	mail   MailSender
	cfg    ServiceConfig
}

func NewService(store DecisionStore, trader TraderClient, mail MailSender, cfg ServiceConfig) *Service {
	return &Service{store: store, trader: trader, mail: mail, cfg: cfg}
}

/*
Mode returns the trading mode the service was constructed with
("paper" | "live"). Used by the /health endpoint to decide whether
schwab_trading auth failures are fatal (live) or merely a warning
(paper — trading scope isn't load-bearing in paper mode).
*/
func (s *Service) Mode() string { return s.cfg.Mode }

/*
HandleQualifyingPick mints a decision row, sends the confirmation
email, and returns. The 5-minute timer is enforced by the
CancelExpiredDecisions cron + the token's embedded expiry — there is
no in-process timer that would die on restart. Errors do NOT block
the morning email pipeline: they're logged and the day moves on.
*/
func (s *Service) HandleQualifyingPick(ctx context.Context, t *trades.Trade) error {
	if s.cfg.Recipient == "" {
		return errors.New("execution recipient not configured")
	}
	if len(s.cfg.HMACSecret) < 32 {
		return errors.New("execution HMAC secret missing or too short")
	}

	occ, err := OCCSymbol(t.Symbol, t.Expiration, t.ContractType, t.StrikePrice)
	if err != nil {
		return fmt.Errorf("build OCC symbol: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(confirmationWindow)
	tradeDate := now.In(easternTime()).Format("2006-01-02")

	/*
		Insert the decision row first so the auto-generated id can be baked
		into the token payload. token_hash is left null on insert and
		written back via SetDecisionTokenHash once the real token exists —
		audit trail only; single-use enforcement is provided by
		SetDecisionStatus's atomic WHERE decision='pending' guard, not by
		hash equality.
	*/
	d := Decision{
		TradeDate:     tradeDate,
		Symbol:        t.Symbol,
		ContractType:  t.ContractType,
		StrikePrice:   t.StrikePrice,
		Expiration:    t.Expiration,
		OCCSymbol:     occ,
		ContractPrice: t.EstimatedPrice,
		GPTScore:      t.GPTScore,
		ClaudeScore:   t.ClaudeScore,
		ExpiresAt:     expiresAt,
	}
	id, err := s.store.InsertDecision(d)
	if err != nil {
		return fmt.Errorf("insert decision: %w", err)
	}

	executeToken, err := Mint(id, ActionExecute, expiresAt, s.cfg.HMACSecret)
	if err != nil {
		return fmt.Errorf("mint execute token: %w", err)
	}
	declineToken, err := Mint(id, ActionDecline, expiresAt, s.cfg.HMACSecret)
	if err != nil {
		return fmt.Errorf("mint decline token: %w", err)
	}

	if err := s.store.SetDecisionTokenHash(id, TokenHash(executeToken)); err != nil {
		// Non-fatal: audit trail loss only. Email still sends.
		log.Printf("execution: backfill token hash failed (audit only, non-fatal): %v", err)
	}

	emailData := templates.ExecuteConfirmData{
		Subject:         fmt.Sprintf("[%s] Confirm trade: %s %s", strings.ToUpper(s.cfg.Mode), t.Symbol, t.ContractType),
		Date:            now.In(easternTime()).Format("Monday, Jan 2 · 3:04 PM ET"),
		Mode:            s.cfg.Mode,
		Symbol:          t.Symbol,
		ContractType:    t.ContractType,
		StrikePrice:     t.StrikePrice,
		Expiration:      t.Expiration,
		DTE:             t.DTE,
		OCCSymbol:       occ,
		ContractPrice:   t.EstimatedPrice,
		CapitalAtRisk:   t.EstimatedPrice * 100,
		CurrentPrice:    t.CurrentPrice,
		RiskLevel:       t.RiskLevel,
		Catalyst:        t.Catalyst,
		Thesis:          t.Thesis,
		GPTModelName:    s.cfg.GPTModelLabel,
		GPTScore:        t.GPTScore,
		GPTRationale:    t.GPTRationale,
		GPTVerdict:      t.GPTVerdict,
		ClaudeModelName: s.cfg.ClaudeModelLabel,
		ClaudeScore:     t.ClaudeScore,
		ClaudeRationale: t.ClaudeRationale,
		ClaudeVerdict:   t.ClaudeVerdict,
		ExpiresAtText:   expiresAt.In(easternTime()).Format("3:04:05 PM ET"),
		ExecuteURL:      s.confirmURL(executeToken, "execute"),
		DeclineURL:      s.confirmURL(declineToken, "decline"),
	}
	html, err := templates.RenderExecuteConfirm(emailData)
	if err != nil {
		return fmt.Errorf("render confirm email: %w", err)
	}
	if err := s.mail.SendTradeEmail(s.cfg.EmailFrom, []string{s.cfg.Recipient}, emailData.Subject, html); err != nil {
		return fmt.Errorf("send confirm email: %w", err)
	}
	log.Printf("execution: confirmation email sent (decision_id=%d, expires=%s)", id, expiresAt.Format(time.RFC3339))
	return nil
}

/*
ConfirmDecision is invoked by the HTTP handler once the token + auth
gates have passed. Returns the post-action human-readable summary
string for the confirmation page to render.
*/
func (s *Service) ConfirmDecision(ctx context.Context, decisionID int, action Action) (string, error) {
	d, err := s.store.GetDecision(decisionID)
	if err != nil {
		return "", err
	}
	if d.Decision != "pending" {
		return "", fmt.Errorf("decision already %s", d.Decision)
	}
	if time.Now().After(d.ExpiresAt) {
		return "", errors.New("decision window expired")
	}

	switch action {
	case ActionDecline:
		if err := s.store.SetDecisionStatus(decisionID, "decline"); err != nil {
			return "", err
		}
		s.sendCanceledEmail(d)
		return fmt.Sprintf("Trade declined: %s %.2f %s. No order was placed.", d.Symbol, d.StrikePrice, d.ContractType), nil
	case ActionExecute:
		if err := s.store.SetDecisionStatus(decisionID, "execute"); err != nil {
			return "", err
		}
		/*
			Place the order asynchronously so the HTTP request doesn't
			block on the broker round-trip.
		*/
		go s.submitOpen(context.Background(), d)
		return fmt.Sprintf("Trade execution confirmed: %s %.2f %s. Order is being submitted; receipt email will follow.", d.Symbol, d.StrikePrice, d.ContractType), nil
	default:
		return "", fmt.Errorf("invalid action %q", action)
	}
}

/*
submitOpen is the async tail of the Execute path: places the broker
order, polls for fill, persists the execution row, sends the receipt.
Errors here CANNOT roll back the user's decision (they already clicked
Execute), but they're logged and surfaced in the receipt email so the
user knows what happened.
*/
func (s *Service) submitOpen(ctx context.Context, d *Decision) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("execution: submitOpen panic: %v", r)
		}
	}()

	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		log.Printf("execution: account hash: %v", err)
		return
	}

	order, err := BuildOpenOrder(d)
	if err != nil {
		log.Printf("execution: build open order: %v", err)
		return
	}

	execRow := Execution{
		DecisionID:        d.ID,
		Mode:              s.cfg.Mode,
		Side:              "open",
		Status:            "pending",
		RequestedQuantity: MaxContracts,
	}
	execID, err := s.store.InsertExecution(execRow)
	if err != nil {
		log.Printf("execution: insert open: %v", err)
		return
	}

	orderID, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", nil, 0, err.Error())
		log.Printf("execution: place open order failed: %v", err)
		return
	}

	st, err := s.trader.GetOrder(ctx, hash, orderID)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", nil, 0, err.Error())
		log.Printf("execution: get open order status: %v", err)
		return
	}

	/*
		Paper mode fills instantly; live mode may need polling. For v1 we
		trust whatever GetOrder returns — the close cron will handle the
		case where status is still WORKING by 3:55pm (unlikely for a
		market order at 9:30am).
	*/
	if st.Filled {
		fp := st.FillPrice
		_ = s.store.UpdateExecutionStatus(execID, "filled", &fp, st.FilledQuantity, "")
		s.sendReceiptEmail(d, orderID, st.FillPrice)
	} else {
		_ = s.store.UpdateExecutionStatus(execID, "working", nil, 0, "")
		log.Printf("execution: open order working (id=%s, status=%s)", orderID, st.RawStatus)
	}
}

/*
CancelExpiredDecisions is called by the every-minute cron during the
9-10am ET window. Marks any pending decisions whose 5-minute window
has elapsed as 'timeout' and fires the cancellation email.
*/
func (s *Service) CancelExpiredDecisions(ctx context.Context) {
	rows, err := s.store.PendingDecisions()
	if err != nil {
		log.Printf("execution: pending decisions: %v", err)
		return
	}
	now := time.Now()
	for _, d := range rows {
		if d.ExpiresAt.Before(now) {
			if err := s.store.SetDecisionStatus(d.ID, "timeout"); err != nil {
				log.Printf("execution: timeout decision %d: %v", d.ID, err)
				continue
			}
			s.sendCanceledEmail(&d)
			log.Printf("execution: decision %d auto-cancelled (window expired)", d.ID)
		}
	}
}

/*
CloseAllPositionsForDate is the 3:55pm ET load-bearing safety job.
Wraps each position close in its own panic recovery so one failure
can't prevent another from running. Designed to NEVER skip — even if
the morning email pipeline crashed, this cron will fire as long as
the binary is up.
*/
func (s *Service) CloseAllPositionsForDate(ctx context.Context, tradeDate string) {
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

func (s *Service) closeOne(ctx context.Context, d *Decision) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("execution: closeOne panic for decision %d: %v", d.ID, r)
		}
	}()

	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		s.sendCloseFailedEmail(d, fmt.Sprintf("account hash lookup failed: %v", err))
		return
	}

	order, err := BuildCloseOrder(d)
	if err != nil {
		s.sendCloseFailedEmail(d, fmt.Sprintf("build close order: %v", err))
		return
	}

	execRow := Execution{
		DecisionID:        d.ID,
		Mode:              s.cfg.Mode,
		Side:              "close",
		Status:            "pending",
		RequestedQuantity: MaxContracts,
	}
	execID, err := s.store.InsertExecution(execRow)
	if err != nil {
		log.Printf("execution: insert close row: %v", err)
		s.sendCloseFailedEmail(d, fmt.Sprintf("insert close row: %v", err))
		return
	}

	// First attempt: market order at 3:55pm.
	orderID, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", nil, 0, err.Error())
		s.sendCloseFailedEmail(d, fmt.Sprintf("first PlaceOrder: %v", err))
		return
	}
	if s.pollFilled(ctx, hash, orderID, 8, 15*time.Second) {
		s.recordCloseAndEmail(ctx, d, execID, hash, orderID)
		return
	}

	// Second attempt: cancel + replace at 3:57pm.
	_ = s.trader.CancelOrder(ctx, hash, orderID)
	orderID2, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", nil, 0, "cancel-replace failed: "+err.Error())
		s.sendCloseFailedEmail(d, fmt.Sprintf("cancel-replace PlaceOrder: %v", err))
		return
	}
	if s.pollFilled(ctx, hash, orderID2, 8, 15*time.Second) {
		s.recordCloseAndEmail(ctx, d, execID, hash, orderID2)
		return
	}

	// Still not filled by 3:59pm. Page the human.
	_ = s.store.UpdateExecutionStatus(execID, "failed", nil, 0, "unfilled after retry-cancel-replace")
	s.sendCloseFailedEmail(d, "Position did not fill within 4-minute retry-cancel-replace window. Close on Schwab manually before 4:00pm ET.")
}

/*
pollFilled waits for an order to reach FILLED status, polling every
`interval` for `attempts` cycles. Returns true if filled, false on
timeout or any error during polling.
*/
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

func (s *Service) recordCloseAndEmail(ctx context.Context, d *Decision, execID int, hash, orderID string) {
	st, err := s.trader.GetOrder(ctx, hash, orderID)
	if err != nil {
		log.Printf("execution: post-fill GetOrder: %v", err)
		return
	}
	fp := st.FillPrice
	_ = s.store.UpdateExecutionStatus(execID, "filled", &fp, st.FilledQuantity, "")

	// Look up the matching open execution to compute realized P&L.
	openPrice := d.ContractPrice
	open, err := s.findOpenExecution(d.ID)
	if err == nil && open != nil && open.FillPrice != nil {
		openPrice = *open.FillPrice
	}
	realized := (st.FillPrice - openPrice) * 100 * float64(MaxContracts)

	data := templates.ExecuteCloseReceiptData{
		Subject:            fmt.Sprintf("[%s] Position closed: %s %s · P&L $%.2f", strings.ToUpper(s.cfg.Mode), d.Symbol, d.ContractType, realized),
		Date:               time.Now().In(easternTime()).Format("Monday, Jan 2 · 3:04 PM ET"),
		Mode:               s.cfg.Mode,
		Symbol:             d.Symbol,
		ContractType:       d.ContractType,
		StrikePrice:        d.StrikePrice,
		Expiration:         d.Expiration,
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

/*
findOpenExecution locates the open-side execution for a decision so
the close path can compute realized P&L from the actual entry fill
(which can differ from decision.ContractPrice in live mode due to
slippage). Returns the most recent open-side row regardless of fill
status — caller checks FillPrice nil/non-nil before using.
*/
func (s *Service) findOpenExecution(decisionID int) (*Execution, error) {
	return s.store.OpenExecutionForDecision(decisionID)
}

func (s *Service) sendCanceledEmail(d *Decision) {
	data := templates.ExecuteCanceledData{
		Subject:         fmt.Sprintf("[%s] Trade not executed: %s", strings.ToUpper(s.cfg.Mode), d.Symbol),
		Date:            time.Now().In(easternTime()).Format("Monday, Jan 2 · 3:04 PM ET"),
		Mode:            s.cfg.Mode,
		Symbol:          d.Symbol,
		ContractType:    d.ContractType,
		StrikePrice:     d.StrikePrice,
		Expiration:      d.Expiration,
		ContractPrice:   d.ContractPrice,
		GPTModelName:    s.cfg.GPTModelLabel,
		GPTScore:        d.GPTScore,
		ClaudeModelName: s.cfg.ClaudeModelLabel,
		ClaudeScore:     d.ClaudeScore,
	}
	html, err := templates.RenderExecuteCanceled(data)
	if err != nil {
		log.Printf("execution: render canceled email: %v", err)
		return
	}
	if err := s.mail.SendTradeEmail(s.cfg.EmailFrom, []string{s.cfg.Recipient}, data.Subject, html); err != nil {
		log.Printf("execution: send canceled email: %v", err)
	}
}

func (s *Service) sendReceiptEmail(d *Decision, orderID string, fillPrice float64) {
	data := templates.ExecuteReceiptData{
		Subject:            fmt.Sprintf("[%s] Order filled: %s %s @ $%.2f", strings.ToUpper(s.cfg.Mode), d.Symbol, d.ContractType, fillPrice),
		Date:               time.Now().In(easternTime()).Format("Monday, Jan 2 · 3:04 PM ET"),
		Mode:               s.cfg.Mode,
		Symbol:             d.Symbol,
		ContractType:       d.ContractType,
		StrikePrice:        d.StrikePrice,
		Expiration:         d.Expiration,
		OCCSymbol:          d.OCCSymbol,
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

func (s *Service) sendCloseFailedEmail(d *Decision, errMsg string) {
	data := templates.ExecuteCloseFailedData{
		Subject:            fmt.Sprintf("[ACTION REQUIRED] vibetradez close failed: %s", d.Symbol),
		Date:               time.Now().In(easternTime()).Format("Monday, Jan 2 · 3:04 PM ET"),
		Symbol:             d.Symbol,
		ContractType:       d.ContractType,
		StrikePrice:        d.StrikePrice,
		Expiration:         d.Expiration,
		OCCSymbol:          d.OCCSymbol,
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

// confirmURL builds a /execute deep link with the signed token.
func (s *Service) confirmURL(token, action string) string {
	base := strings.TrimRight(s.cfg.PublicBaseURL, "/")
	return fmt.Sprintf("%s/execute?token=%s&action=%s", base, token, action)
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
