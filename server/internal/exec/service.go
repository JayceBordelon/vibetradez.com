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
verbatim per the task spec, do not parameterize.
*/
const schwabPositionsURL = "https://client.schwab.com/app/accounts/positions/#/"

/*
DecisionStore is the slice of *store.Store that exec.Service needs.
Defined as an interface so tests don't need a real Postgres.
*/
type DecisionStore interface {
	InsertExecution(e Execution) (int, error)
	UpdateExecutionStatus(id int, status string, fillPrice *float64, filledQty int, errMsg string) error
	GetExecution(id int) (*Execution, error)
	OpenExecutionForTrade(tradeID int) (*Execution, error)
	LiveExecutionsForDate(tradeDate string) ([]Execution, error)
	OpenPositionsForDate(tradeDate string) ([]OpenPosition, error)
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
(paper, trading scope isn't load-bearing in paper mode).
*/
func (s *Service) Mode() string { return s.cfg.Mode }

/*
HandleQualifyingPick is fired by the morning cron when QualifyingPick
returns a rank-1 contract under the cap. There's no user confirmation
step, the order goes straight to the broker (paper or live depending
on Mode) and a receipt email follows the fill.

t.ID must be set (the cron passes the saved trade row from the DB)
so the execution row can reference back. Errors do NOT block the
morning email pipeline, they're logged and the day moves on.
*/
func (s *Service) HandleQualifyingPick(ctx context.Context, t *trades.Trade, tradeID int) error {
	if s.cfg.Recipient == "" {
		return errors.New("execution recipient not configured")
	}
	if tradeID == 0 {
		return errors.New("tradeID must be set for auto-execution")
	}

	occ, err := OCCSymbol(t.Symbol, t.Expiration, t.ContractType, t.StrikePrice)
	if err != nil {
		return fmt.Errorf("build OCC symbol: %w", err)
	}

	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		return fmt.Errorf("account hash: %w", err)
	}

	order, err := BuildOpenOrderForTrade(t, occ)
	if err != nil {
		return fmt.Errorf("build open order: %w", err)
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
		return fmt.Errorf("insert open execution: %w", err)
	}

	orderID, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", nil, 0, err.Error())
		return fmt.Errorf("place open order: %w", err)
	}

	st, err := s.trader.GetOrder(ctx, hash, orderID)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", nil, 0, err.Error())
		return fmt.Errorf("get open order status: %w", err)
	}

	if st.Filled {
		fp := st.FillPrice
		_ = s.store.UpdateExecutionStatus(execID, "filled", &fp, st.FilledQuantity, "")
		s.sendReceiptEmail(t, occ, orderID, st.FillPrice)
		log.Printf("execution: open filled (trade_id=%d, mode=%s, fill=%.2f, order=%s)", tradeID, s.cfg.Mode, st.FillPrice, orderID)
	} else {
		_ = s.store.UpdateExecutionStatus(execID, "working", nil, 0, "")
		log.Printf("execution: open order working (trade_id=%d, order=%s, status=%s)", tradeID, orderID, st.RawStatus)
	}
	return nil
}

/*
CloseAllPositionsForDate is called by the 3:55pm ET load-bearing
safety job. Wraps each position close in its own panic recovery so
one failure can't prevent another from running. Designed to NEVER
skip, even if the morning open path crashed, this cron will fire as
long as the binary is up.
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
		_ = s.store.UpdateExecutionStatus(execID, "failed", nil, 0, err.Error())
		s.sendCloseFailedEmail(p, fmt.Sprintf("first PlaceOrder: %v", err))
		return
	}
	if s.pollFilled(ctx, hash, orderID, 8, 15*time.Second) {
		s.recordCloseAndEmail(ctx, p, execID, hash, orderID)
		return
	}

	_ = s.trader.CancelOrder(ctx, hash, orderID)
	orderID2, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		_ = s.store.UpdateExecutionStatus(execID, "failed", nil, 0, "cancel-replace failed: "+err.Error())
		s.sendCloseFailedEmail(p, fmt.Sprintf("cancel-replace PlaceOrder: %v", err))
		return
	}
	if s.pollFilled(ctx, hash, orderID2, 8, 15*time.Second) {
		s.recordCloseAndEmail(ctx, p, execID, hash, orderID2)
		return
	}

	_ = s.store.UpdateExecutionStatus(execID, "failed", nil, 0, "unfilled after retry-cancel-replace")
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
	_ = s.store.UpdateExecutionStatus(execID, "filled", &fp, st.FilledQuantity, "")

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
