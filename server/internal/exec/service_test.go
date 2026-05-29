package exec

import (
	"context"
	"errors"
	"strings"
	"testing"

	"vibetradez.com/internal/trades"
)

type fakeStore struct {
	inserts          []Execution
	updates          []storeUpdate
	resolves         []resolveCall
	nextID           int
	workingPositions []OpenPosition
	closeSummaryRows []CloseSummaryRow
}

type storeUpdate struct {
	id         int
	status     string
	orderID    string
	fillPrice  *float64
	filledQty  int
	errMessage string
}

type resolveCall struct {
	tradeID    int
	strike     float64
	expiration string
	dte        int
	entry      float64
	stop       float64
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
func (f *fakeStore) WorkingOpenPositionsForDate(string) ([]OpenPosition, error) {
	return f.workingPositions, nil
}
func (f *fakeStore) ResolveTradeContract(tradeID int, strike float64, expiration string, dte int, entry, stop float64) error {
	f.resolves = append(f.resolves, resolveCall{tradeID, strike, expiration, dte, entry, stop})
	return nil
}
func (f *fakeStore) CloseSummaryForDate(string) ([]CloseSummaryRow, error) {
	return f.closeSummaryRows, nil
}
func (f *fakeStore) BasketSummaryForDate(string) ([]BasketSummaryRow, error) {
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

/*
fakeTrader captures PlaceOrder calls + can be tuned to return synthetic
fill / reject / error outcomes. AvailableFunds defaults to "comfortably
above any test cap" so cash isn't the gating factor unless a test
explicitly sets availFunds.
*/
type fakeTrader struct {
	placeID    string
	placeErr   error
	status     OrderStatus
	getErr     error
	availFunds float64
	availErr   error
	placed     []Order
}

func (f *fakeTrader) AccountHash(context.Context) (string, error) { return "ACCT-HASH", nil }
func (f *fakeTrader) AvailableFunds(context.Context, string) (float64, error) {
	if f.availErr != nil {
		return 0, f.availErr
	}
	if f.availFunds == 0 {
		return 1e6, nil
	}
	return f.availFunds, nil
}
func (f *fakeTrader) PlaceOrder(_ context.Context, _ string, o Order) (string, error) {
	f.placed = append(f.placed, o)
	return f.placeID, f.placeErr
}
func (f *fakeTrader) GetOrder(context.Context, string, string) (OrderStatus, error) {
	return f.status, f.getErr
}
func (f *fakeTrader) CancelOrder(context.Context, string, string) error { return nil }

func newTestService(trader TraderClient, store DecisionStore, mail MailSender) *Service {
	return NewService(store, trader, mail, ServiceConfig{
		Mode:      "live",
		Recipient: "ops@example.com",
		EmailFrom: "Vibe <test@example.com>",
		SchwabAccountHash: func(ctx context.Context) (string, error) {
			return trader.AccountHash(ctx)
		},
	})
}

func sampleTrade() *trades.Trade {
	return &trades.Trade{ID: 42, Symbol: "AAPL", ContractType: "CALL"}
}

// ── PlaceBuyToOpenAgent ────────────────────────────────────────────

func TestPlaceBuyToOpenAgent_HappyPath_PersistsResolutionAndOrder(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &fakeTrader{placeID: "ORDER-1"}
	svc := newTestService(trader, store, mail)

	orderID, execID, err := svc.PlaceBuyToOpenAgent(context.Background(), sampleTrade(), 150, "2099-01-15", 5, 2.30, 1)
	if err != nil {
		t.Fatalf("PlaceBuyToOpenAgent: %v", err)
	}
	if orderID != "ORDER-1" || execID == 0 {
		t.Errorf("expected orderID + execID, got %q / %d", orderID, execID)
	}
	if len(store.resolves) != 1 || store.resolves[0].strike != 150 {
		t.Errorf("expected one ResolveTradeContract call with strike=150, got %+v", store.resolves)
	}
	if len(store.inserts) != 1 || store.inserts[0].Status != "pending" || store.inserts[0].RequestedQuantity != 1 {
		t.Errorf("expected one pending insert with qty 1, got %+v", store.inserts)
	}
	// Two updates: working with orderID, then nothing else (no GetOrder
	// post-place — agent path is fire-and-forget).
	if len(store.updates) != 1 || store.updates[0].status != "working" || store.updates[0].orderID != "ORDER-1" {
		t.Errorf("expected one 'working' update with orderID, got %+v", store.updates)
	}
	if len(trader.placed) != 1 {
		t.Errorf("expected one PlaceOrder call, got %d", len(trader.placed))
	}
}

func TestPlaceBuyToOpenAgent_MultiQuantity_ThreadsThroughOrderAndExecution(t *testing.T) {
	store := &fakeStore{}
	trader := &fakeTrader{placeID: "ORDER-1"}
	svc := newTestService(trader, store, &fakeMail{})

	// 3 contracts at $2.00 = $600 total cost, within the $1000 per-order cap.
	if _, _, err := svc.PlaceBuyToOpenAgent(context.Background(), sampleTrade(), 150, "2099-01-15", 5, 2.00, 3); err != nil {
		t.Fatalf("PlaceBuyToOpenAgent qty 3: %v", err)
	}
	if len(store.inserts) != 1 || store.inserts[0].RequestedQuantity != 3 {
		t.Errorf("expected insert RequestedQuantity 3, got %+v", store.inserts)
	}
	if len(trader.placed) != 1 || trader.placed[0].OrderLegCollection[0].Quantity != 3 {
		t.Errorf("expected broker order leg quantity 3, got %+v", trader.placed)
	}
}

func TestPlaceBuyToOpenAgent_RefusesOverPerOrderCost(t *testing.T) {
	svc := newTestService(&fakeTrader{}, &fakeStore{}, &fakeMail{})
	// 6 contracts at $2.00 = $1200, over MaxOrderCost ($1000), even though
	// the $2.00/share limit is well under the per-share cap.
	if _, _, err := svc.PlaceBuyToOpenAgent(context.Background(), sampleTrade(), 150, "2099-01-15", 5, 2.00, 6); err == nil {
		t.Fatal("expected per-order cost refusal")
	}
}

func TestPlaceBuyToOpenAgent_RejectsNonPositiveQuantity(t *testing.T) {
	svc := newTestService(&fakeTrader{}, &fakeStore{}, &fakeMail{})
	for _, q := range []int{0, -1} {
		if _, _, err := svc.PlaceBuyToOpenAgent(context.Background(), sampleTrade(), 150, "2099-01-15", 5, 2.30, q); err == nil {
			t.Errorf("quantity=%d: expected refusal", q)
		}
	}
}

func TestPlaceBuyToOpenAgent_RefusesOverCapLimitPrice(t *testing.T) {
	svc := newTestService(&fakeTrader{}, &fakeStore{}, &fakeMail{})
	if _, _, err := svc.PlaceBuyToOpenAgent(context.Background(), sampleTrade(), 150, "2099-01-15", 5, 10.01, 1); err == nil {
		t.Fatal("expected over-cap refusal")
	}
}

func TestPlaceBuyToOpenAgent_RefusesNonPositiveLimit(t *testing.T) {
	svc := newTestService(&fakeTrader{}, &fakeStore{}, &fakeMail{})
	for _, lp := range []float64{0, -1.50} {
		if _, _, err := svc.PlaceBuyToOpenAgent(context.Background(), sampleTrade(), 150, "2099-01-15", 5, lp, 1); err == nil {
			t.Errorf("limit_price=%.2f: expected refusal", lp)
		}
	}
}

func TestPlaceBuyToOpenAgent_PersistsFailedOnPlaceError(t *testing.T) {
	store := &fakeStore{}
	trader := &fakeTrader{placeErr: errors.New("broker rejected: insufficient buying power")}
	svc := newTestService(trader, store, &fakeMail{})

	_, _, err := svc.PlaceBuyToOpenAgent(context.Background(), sampleTrade(), 150, "2099-01-15", 5, 2.30, 1)
	if err == nil {
		t.Fatal("expected error when PlaceOrder fails")
	}
	// Pending insert + failed update.
	if len(store.inserts) != 1 {
		t.Errorf("expected one execution insert, got %d", len(store.inserts))
	}
	if len(store.updates) != 1 || store.updates[0].status != "failed" {
		t.Errorf("expected a 'failed' status update, got %+v", store.updates)
	}
}

func TestPlaceBuyToOpenAgent_RejectsMissingTradeID(t *testing.T) {
	svc := newTestService(&fakeTrader{}, &fakeStore{}, &fakeMail{})
	if _, _, err := svc.PlaceBuyToOpenAgent(context.Background(), &trades.Trade{Symbol: "AAPL", ContractType: "CALL"}, 150, "2099-01-15", 5, 2.30, 1); err == nil {
		t.Fatal("expected error on missing trade ID")
	}
}

// ── InsertSkippedExecutionAgent ────────────────────────────────────

func TestInsertSkippedExecutionAgent_PersistsRowWithReason(t *testing.T) {
	store := &fakeStore{}
	svc := newTestService(&fakeTrader{}, store, &fakeMail{})

	id, err := svc.InsertSkippedExecutionAgent(42, "Stock gapped down 8% on overnight news")
	if err != nil {
		t.Fatalf("InsertSkippedExecutionAgent: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero execution id")
	}
	if len(store.inserts) != 1 {
		t.Fatalf("expected one insert, got %d", len(store.inserts))
	}
	row := store.inserts[0]
	if row.Status != "skipped" || row.RequestedQuantity != 0 || row.Side != "open" {
		t.Errorf("unexpected skipped row shape: %+v", row)
	}
	if row.ErrorMessage != "Stock gapped down 8% on overnight news" {
		t.Errorf("reason not on row: %q", row.ErrorMessage)
	}
}

func TestInsertSkippedExecutionAgent_RejectsMissingTradeID(t *testing.T) {
	svc := newTestService(&fakeTrader{}, &fakeStore{}, &fakeMail{})
	if _, err := svc.InsertSkippedExecutionAgent(0, "some reason"); err == nil {
		t.Fatal("expected error on missing trade ID")
	}
}

// ── AvailableFundsAgent ────────────────────────────────────────────

func TestAvailableFundsAgent_DispatchesToTrader(t *testing.T) {
	trader := &fakeTrader{availFunds: 1250.50}
	svc := newTestService(trader, &fakeStore{}, &fakeMail{})

	usd, err := svc.AvailableFundsAgent(context.Background())
	if err != nil {
		t.Fatalf("AvailableFundsAgent: %v", err)
	}
	if usd != 1250.50 {
		t.Errorf("expected 1250.50, got %.2f", usd)
	}
}

func TestAvailableFundsAgent_PropagatesError(t *testing.T) {
	trader := &fakeTrader{availErr: errors.New("HTTP 401: token expired")}
	svc := newTestService(trader, &fakeStore{}, &fakeMail{})

	if _, err := svc.AvailableFundsAgent(context.Background()); err == nil {
		t.Fatal("expected error to propagate")
	}
}

// ── SendCloseSummary (consolidated daily close email) ──────────────

func TestSendCloseSummary_SendsConsolidatedDailyEmail(t *testing.T) {
	store := &fakeStore{
		closeSummaryRows: []CloseSummaryRow{
			{Rank: 1, Symbol: "AAPL", ContractType: "CALL", StrikePrice: 200, Mode: "live", Status: "filled", OpenPrice: 1.50, ClosePrice: 2.30, RealizedPnL: 80, Quantity: 1},
			{Rank: 2, Symbol: "NVDA", ContractType: "CALL", StrikePrice: 950, Mode: "live", Status: "filled", OpenPrice: 4.20, ClosePrice: 3.10, RealizedPnL: -110, Quantity: 1},
			{Rank: 3, Symbol: "TSLA", ContractType: "PUT", StrikePrice: 250, Mode: "live", Status: "failed", OpenPrice: 1.85, ClosePrice: 0, RealizedPnL: 0, Quantity: 1, ErrorMessage: "Position did not fill within 4-minute retry-cancel-replace window"},
		},
	}
	mail := &fakeMail{}
	svc := newTestService(&fakeTrader{}, store, mail)

	svc.SendCloseSummary(context.Background(), "2026-05-21")

	if len(mail.sent) != 1 {
		t.Fatalf("expected exactly 1 consolidated email, got %d", len(mail.sent))
	}
	sent := mail.sent[0]
	if !strings.Contains(sent.subject, "2/3 closed") {
		t.Errorf("subject should mention 2/3 closed, got %q", sent.subject)
	}
	if !strings.Contains(sent.subject, "$-30.00") && !strings.Contains(sent.subject, "$-30") {
		t.Errorf("subject should mention net P&L, got %q", sent.subject)
	}
	for _, want := range []string{"AAPL", "NVDA", "TSLA", "Position did not fill"} {
		if !strings.Contains(sent.html, want) {
			t.Errorf("email body missing %q", want)
		}
	}
	if !strings.Contains(sent.html, "Action required") {
		t.Error("email body missing action-required callout for failed close")
	}
}

func TestSendCloseSummary_EmptyDayStillSendsNotice(t *testing.T) {
	store := &fakeStore{closeSummaryRows: nil}
	mail := &fakeMail{}
	svc := newTestService(&fakeTrader{}, store, mail)

	svc.SendCloseSummary(context.Background(), "2026-05-21")

	if len(mail.sent) != 1 {
		t.Fatalf("expected 1 email even on empty day, got %d", len(mail.sent))
	}
	if !strings.Contains(mail.sent[0].html, "No close-side executions") {
		t.Errorf("empty-day email should explain the absence, body=%q", mail.sent[0].html)
	}
}
