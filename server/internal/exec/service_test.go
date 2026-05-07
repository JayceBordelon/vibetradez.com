package exec

import (
	"context"
	"errors"
	"strings"
	"testing"

	"vibetradez.com/internal/trades"
)

type fakeStore struct {
	inserts []Execution
	updates []storeUpdate
	nextID  int
}

type storeUpdate struct {
	id         int
	status     string
	orderID    string
	fillPrice  *float64
	filledQty  int
	errMessage string
}

func (f *fakeStore) InsertExecution(e Execution) (int, error) {
	f.nextID++
	e.ID = f.nextID
	f.inserts = append(f.inserts, e)
	return f.nextID, nil
}
func (f *fakeStore) UpdateExecutionStatus(id int, status, schwabOrderID string, fillPrice *float64, filledQty int, errMsg string) error {
	f.updates = append(f.updates, storeUpdate{id, status, schwabOrderID, fillPrice, filledQty, errMsg})
	return nil
}
func (f *fakeStore) GetExecution(int) (*Execution, error)              { return nil, errors.New("unused") }
func (f *fakeStore) OpenExecutionForTrade(int) (*Execution, error)     { return nil, errors.New("unused") }
func (f *fakeStore) LiveExecutionsForDate(string) ([]Execution, error) { return nil, nil }
func (f *fakeStore) OpenPositionsForDate(string) ([]OpenPosition, error) {
	return nil, nil
}

type fakeMail struct {
	sent []sentEmail
}

type sentEmail struct {
	from    string
	to      []string
	subject string
	html    string
}

func (f *fakeMail) SendTradeEmail(from string, to []string, subject, html string) error {
	f.sent = append(f.sent, sentEmail{from, to, subject, html})
	return nil
}

type fakeTrader struct {
	placeID  string
	placeErr error
	status   OrderStatus
	getErr   error
}

func (f *fakeTrader) AccountHash(context.Context) (string, error) { return "ACCT-HASH", nil }
func (f *fakeTrader) PlaceOrder(context.Context, string, Order) (string, error) {
	return f.placeID, f.placeErr
}
func (f *fakeTrader) GetOrder(context.Context, string, string) (OrderStatus, error) {
	return f.status, f.getErr
}
func (f *fakeTrader) CancelOrder(context.Context, string, string) error { return nil }

func newTestService(trader TraderClient, store DecisionStore, mail MailSender) *Service {
	return newTestServiceWithAsk(trader, store, mail, nil)
}

func newTestServiceWithAsk(trader TraderClient, store DecisionStore, mail MailSender, ask func(context.Context, string, string, string, float64) (float64, error)) *Service {
	return NewService(store, trader, mail, ServiceConfig{
		Mode:      "live",
		Recipient: "ops@example.com",
		EmailFrom: "Vibe <test@example.com>",
		SchwabAccountHash: func(ctx context.Context) (string, error) {
			return trader.AccountHash(ctx)
		},
		OptionAsk: ask,
	})
}

func sampleTrade() *trades.Trade {
	return &trades.Trade{
		Symbol:         "AAPL",
		ContractType:   "CALL",
		StrikePrice:    150,
		Expiration:     "2026-06-19",
		EstimatedPrice: 1.25,
	}
}

func TestHandleQualifyingPick_RejectedPersistsReasonAndEmails(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &fakeTrader{
		placeID: "ORDER-XYZ",
		status: OrderStatus{
			OrderID:      "ORDER-XYZ",
			RawStatus:    "REJECTED",
			Terminal:     true,
			ErrorMessage: "Buying power insufficient",
		},
	}
	svc := newTestService(trader, store, mail)

	if err := svc.HandleQualifyingPick(context.Background(), sampleTrade(), 42); err != nil {
		t.Fatalf("HandleQualifyingPick: %v", err)
	}

	if len(store.updates) != 1 {
		t.Fatalf("expected 1 status update, got %d: %+v", len(store.updates), store.updates)
	}
	upd := store.updates[0]
	if upd.status != "rejected" {
		t.Errorf("status: want rejected, got %q", upd.status)
	}
	if upd.orderID != "ORDER-XYZ" {
		t.Errorf("orderID: want ORDER-XYZ, got %q", upd.orderID)
	}
	if upd.errMessage != "Buying power insufficient" {
		t.Errorf("errMessage: want 'Buying power insufficient', got %q", upd.errMessage)
	}

	if len(mail.sent) != 1 {
		t.Fatalf("expected 1 email, got %d", len(mail.sent))
	}
	got := mail.sent[0]
	if len(got.to) != 1 || got.to[0] != "ops@example.com" {
		t.Errorf("recipient: want [ops@example.com], got %v", got.to)
	}
	if !strings.Contains(got.subject, "Open failed") {
		t.Errorf("subject: want 'Open failed' marker, got %q", got.subject)
	}
	if !strings.Contains(got.html, "Buying power insufficient") {
		t.Errorf("html missing reason")
	}
	if !strings.Contains(got.html, "ORDER-XYZ") {
		t.Errorf("html missing order id")
	}
}

func TestHandleQualifyingPick_RejectedFallsBackToRawStatusWhenNoDescription(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &fakeTrader{
		placeID: "ORDER-NODESC",
		status: OrderStatus{
			OrderID:   "ORDER-NODESC",
			RawStatus: "EXPIRED",
			Terminal:  true,
		},
	}
	svc := newTestService(trader, store, mail)

	if err := svc.HandleQualifyingPick(context.Background(), sampleTrade(), 7); err != nil {
		t.Fatalf("HandleQualifyingPick: %v", err)
	}

	if len(store.updates) != 1 || store.updates[0].errMessage != "EXPIRED" {
		t.Fatalf("expected fallback to RawStatus EXPIRED in errMessage, got %+v", store.updates)
	}
}

func TestHandleQualifyingPick_PlaceErrorEmailsAndPersistsFailed(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &fakeTrader{
		placeErr: errors.New("HTTP 401: token expired"),
	}
	svc := newTestService(trader, store, mail)

	err := svc.HandleQualifyingPick(context.Background(), sampleTrade(), 9)
	if err == nil {
		t.Fatal("expected error from HandleQualifyingPick")
	}

	if len(store.updates) != 1 || store.updates[0].status != "failed" {
		t.Fatalf("expected one 'failed' update, got %+v", store.updates)
	}
	if !strings.Contains(store.updates[0].errMessage, "token expired") {
		t.Errorf("errMessage: want 'token expired', got %q", store.updates[0].errMessage)
	}
	if len(mail.sent) != 1 {
		t.Fatalf("expected 1 alert email, got %d", len(mail.sent))
	}
	if len(mail.sent[0].to) != 1 || mail.sent[0].to[0] != "ops@example.com" {
		t.Errorf("recipient: want [ops@example.com], got %v", mail.sent[0].to)
	}
}

func TestHandleQualifyingPick_FilledPersistsOrderIDAndSendsReceipt(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &fakeTrader{
		placeID: "ORDER-FILL",
		status: OrderStatus{
			OrderID:        "ORDER-FILL",
			RawStatus:      "FILLED",
			Filled:         true,
			Terminal:       true,
			FillPrice:      1.27,
			FilledQuantity: 1,
		},
	}
	svc := newTestService(trader, store, mail)

	if err := svc.HandleQualifyingPick(context.Background(), sampleTrade(), 11); err != nil {
		t.Fatalf("HandleQualifyingPick: %v", err)
	}

	if len(store.updates) != 1 {
		t.Fatalf("expected 1 status update, got %d", len(store.updates))
	}
	upd := store.updates[0]
	if upd.status != "filled" || upd.orderID != "ORDER-FILL" {
		t.Errorf("filled update: want status=filled orderID=ORDER-FILL, got %+v", upd)
	}
	if len(mail.sent) != 1 || !strings.Contains(mail.sent[0].subject, "Order filled") {
		t.Errorf("expected receipt email, got %+v", mail.sent)
	}
}

// capturingTrader records the Order it was given so tests can assert
// the LIMIT price, instruction, and shape.
type capturingTrader struct {
	fakeTrader
	captured Order
}

func (c *capturingTrader) PlaceOrder(ctx context.Context, hash string, o Order) (string, error) {
	c.captured = o
	return c.fakeTrader.PlaceOrder(ctx, hash, o)
}

func TestHandleQualifyingPick_LimitPriceFromLiveAsk(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &capturingTrader{fakeTrader: fakeTrader{
		placeID: "ORDER-LIMIT",
		status:  OrderStatus{OrderID: "ORDER-LIMIT", RawStatus: "FILLED", Filled: true, Terminal: true, FillPrice: 1.20, FilledQuantity: 1},
	}}
	ask := func(_ context.Context, _, _, _ string, _ float64) (float64, error) {
		return 1.20, nil
	}
	svc := newTestServiceWithAsk(trader, store, mail, ask)

	if err := svc.HandleQualifyingPick(context.Background(), sampleTrade(), 100); err != nil {
		t.Fatalf("HandleQualifyingPick: %v", err)
	}
	if trader.captured.OrderType != "LIMIT" {
		t.Errorf("OrderType: want LIMIT, got %q", trader.captured.OrderType)
	}
	// 1.20 * 1.05 = 1.26
	if trader.captured.Price != 1.26 {
		t.Errorf("Price: want 1.26, got %.4f", trader.captured.Price)
	}
}

func TestHandleQualifyingPick_LimitFallsBackToEstimateOnAskError(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &capturingTrader{fakeTrader: fakeTrader{
		placeID: "ORDER-FALLBACK",
		status:  OrderStatus{OrderID: "ORDER-FALLBACK", RawStatus: "WORKING", Working: true},
	}}
	ask := func(_ context.Context, _, _, _ string, _ float64) (float64, error) {
		return 0, errors.New("schwab market data 503")
	}
	svc := newTestServiceWithAsk(trader, store, mail, ask)

	tr := sampleTrade()
	tr.EstimatedPrice = 0.80
	if err := svc.HandleQualifyingPick(context.Background(), tr, 101); err != nil {
		t.Fatalf("HandleQualifyingPick: %v", err)
	}
	// 0.80 * 1.05 = 0.84
	if trader.captured.Price != 0.84 {
		t.Errorf("Price: want 0.84 (fallback to estimate), got %.4f", trader.captured.Price)
	}
}

func TestHandleQualifyingPick_LimitClampedToMaxContractPremium(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &capturingTrader{fakeTrader: fakeTrader{
		placeID: "ORDER-CLAMP",
		status:  OrderStatus{OrderID: "ORDER-CLAMP", RawStatus: "WORKING", Working: true},
	}}
	ask := func(_ context.Context, _, _, _ string, _ float64) (float64, error) {
		return 4.95, nil // 4.95 * 1.05 = 5.1975 → clamp to 5.00
	}
	svc := newTestServiceWithAsk(trader, store, mail, ask)

	if err := svc.HandleQualifyingPick(context.Background(), sampleTrade(), 102); err != nil {
		t.Fatalf("HandleQualifyingPick: %v", err)
	}
	if trader.captured.Price != MaxContractPremium {
		t.Errorf("Price: want %.2f (clamped), got %.4f", MaxContractPremium, trader.captured.Price)
	}
}

func TestHandleQualifyingPick_AbortsWhenAskAndEstimateBothZero(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &capturingTrader{fakeTrader: fakeTrader{placeID: "should-not-be-called"}}
	ask := func(_ context.Context, _, _, _ string, _ float64) (float64, error) {
		return 0, errors.New("no quote")
	}
	svc := newTestServiceWithAsk(trader, store, mail, ask)

	tr := sampleTrade()
	tr.EstimatedPrice = 0
	err := svc.HandleQualifyingPick(context.Background(), tr, 103)
	if err == nil {
		t.Fatal("expected error when no usable price basis")
	}
	// Insert may or may not have happened depending on order; the
	// hard contract is: no order was placed AND an alert was sent.
	if trader.captured.OrderType != "" {
		t.Errorf("expected no PlaceOrder call, but trader captured %+v", trader.captured)
	}
	if len(mail.sent) != 1 {
		t.Fatalf("expected 1 alert email, got %d", len(mail.sent))
	}
	if !strings.Contains(mail.sent[0].subject, "Open failed") {
		t.Errorf("subject: want 'Open failed' marker, got %q", mail.sent[0].subject)
	}
}

func TestHandleQualifyingPick_WorkingPersistsOrderID(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &fakeTrader{
		placeID: "ORDER-WORK",
		status: OrderStatus{
			OrderID:   "ORDER-WORK",
			RawStatus: "WORKING",
			Working:   true,
		},
	}
	svc := newTestService(trader, store, mail)

	if err := svc.HandleQualifyingPick(context.Background(), sampleTrade(), 5); err != nil {
		t.Fatalf("HandleQualifyingPick: %v", err)
	}
	if len(store.updates) != 1 {
		t.Fatalf("expected 1 status update, got %d", len(store.updates))
	}
	upd := store.updates[0]
	if upd.status != "working" || upd.orderID != "ORDER-WORK" {
		t.Errorf("working update: want status=working orderID=ORDER-WORK, got %+v", upd)
	}
	if len(mail.sent) != 0 {
		t.Errorf("expected no email on working state, got %d", len(mail.sent))
	}
}
