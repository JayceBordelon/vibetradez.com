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
	nextID           int
	workingPositions []OpenPosition
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
func (f *fakeStore) WorkingOpenPositionsForDate(string) ([]OpenPosition, error) {
	return f.workingPositions, nil
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
	placeID    string
	placeErr   error
	status     OrderStatus
	getErr     error
	availFunds float64
	availErr   error
}

func (f *fakeTrader) AccountHash(context.Context) (string, error) { return "ACCT-HASH", nil }
func (f *fakeTrader) AvailableFunds(context.Context, string) (float64, error) {
	if f.availErr != nil {
		return 0, f.availErr
	}
	if f.availFunds == 0 {
		// Default to comfortably above the basket cap so cash isn't the
		// gating factor for tests that don't care about it.
		return 1e6, nil
	}
	return f.availFunds, nil
}
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

// runSinglePick wraps handleSinglePick with the ACCT-HASH the test
// fakes return so per-pick tests don't have to thread the lookup
// through the full HandleQualifyingPicks basket path. handleSinglePick
// is the workhorse — same code path the basket loop calls.
func runSinglePick(svc *Service, tr *trades.Trade, tradeID int) (string, error) {
	return svc.handleSinglePick(context.Background(), tr, tradeID, "ACCT-HASH")
}

func TestHandleSinglePick_RejectedPersistsReasonAndEmails(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{
		placeID: "ORDER-XYZ",
		status: OrderStatus{
			OrderID:      "ORDER-XYZ",
			RawStatus:    "REJECTED",
			Terminal:     true,
			ErrorMessage: "Buying power insufficient",
		},
	}}
	svc := newTestService(trader, store, mail)

	if _, err := runSinglePick(svc, sampleTrade(), 42); err != nil {
		t.Fatalf("handleSinglePick: %v", err)
	}

	// Expect 2 updates: the orphan-prevention immediate write
	// (working + orderID, set right after PlaceOrder returns) and the
	// terminal rejected update after GetOrder confirms the broker
	// rejection.
	if len(store.updates) != 2 {
		t.Fatalf("expected 2 status updates, got %d: %+v", len(store.updates), store.updates)
	}
	if store.updates[0].status != "working" || store.updates[0].orderID != "ORDER-XYZ" {
		t.Errorf("first update should persist orderID with working status, got %+v", store.updates[0])
	}
	upd := store.updates[1]
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

func TestHandleSinglePick_RejectedFallsBackToRawStatusWhenNoDescription(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{
		placeID: "ORDER-NODESC",
		status: OrderStatus{
			OrderID:   "ORDER-NODESC",
			RawStatus: "EXPIRED",
			Terminal:  true,
		},
	}}
	svc := newTestService(trader, store, mail)

	if _, err := runSinglePick(svc, sampleTrade(), 7); err != nil {
		t.Fatalf("handleSinglePick: %v", err)
	}

	// 2 updates: orphan-prevention immediate working write, then
	// terminal rejected with EXPIRED as the fallback errMessage.
	if len(store.updates) != 2 || store.updates[1].errMessage != "EXPIRED" {
		t.Fatalf("expected 2 updates with EXPIRED in the rejected errMessage, got %+v", store.updates)
	}
}

func TestHandleSinglePick_PlaceErrorEmailsAndPersistsFailed(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{
		placeErr: errors.New("HTTP 401: token expired"),
	}}
	svc := newTestService(trader, store, mail)

	if _, err := runSinglePick(svc, sampleTrade(), 9); err == nil {
		t.Fatal("expected error from handleSinglePick")
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

func TestHandleSinglePick_FilledPersistsOrderIDAndSendsReceipt(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{
		placeID: "ORDER-FILL",
		status: OrderStatus{
			OrderID:        "ORDER-FILL",
			RawStatus:      "FILLED",
			Filled:         true,
			Terminal:       true,
			FillPrice:      1.27,
			FilledQuantity: 1,
		},
	}}
	svc := newTestService(trader, store, mail)

	if _, err := runSinglePick(svc, sampleTrade(), 11); err != nil {
		t.Fatalf("handleSinglePick: %v", err)
	}

	// 2 updates: orphan-prevention working write, then terminal filled.
	if len(store.updates) != 2 {
		t.Fatalf("expected 2 status updates, got %d", len(store.updates))
	}
	if store.updates[0].status != "working" || store.updates[0].orderID != "ORDER-FILL" {
		t.Errorf("first update should persist orderID with working status, got %+v", store.updates[0])
	}
	upd := store.updates[1]
	if upd.status != "filled" || upd.orderID != "ORDER-FILL" {
		t.Errorf("filled update: want status=filled orderID=ORDER-FILL, got %+v", upd)
	}
	if len(mail.sent) != 1 || !strings.Contains(mail.sent[0].subject, "Order filled") {
		t.Errorf("expected receipt email, got %+v", mail.sent)
	}
}

func TestHandleSinglePick_LimitPriceFromLiveAsk(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{
		placeID: "ORDER-LIMIT",
		status:  OrderStatus{OrderID: "ORDER-LIMIT", RawStatus: "FILLED", Filled: true, Terminal: true, FillPrice: 1.20, FilledQuantity: 1},
	}}
	ask := func(_ context.Context, _, _, _ string, _ float64) (float64, error) {
		return 1.20, nil
	}
	svc := newTestServiceWithAsk(trader, store, mail, ask)

	if _, err := runSinglePick(svc, sampleTrade(), 100); err != nil {
		t.Fatalf("handleSinglePick: %v", err)
	}
	if len(trader.placed) != 1 {
		t.Fatalf("expected 1 placed order, got %d", len(trader.placed))
	}
	if trader.placed[0].OrderType != "LIMIT" {
		t.Errorf("OrderType: want LIMIT, got %q", trader.placed[0].OrderType)
	}
	// 1.20 * 1.05 = 1.26
	if trader.placed[0].Price != 1.26 {
		t.Errorf("Price: want 1.26, got %.4f", trader.placed[0].Price)
	}
}

func TestHandleSinglePick_LimitFallsBackToEstimateOnAskError(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{
		placeID: "ORDER-FALLBACK",
		status:  OrderStatus{OrderID: "ORDER-FALLBACK", RawStatus: "WORKING", Working: true},
	}}
	ask := func(_ context.Context, _, _, _ string, _ float64) (float64, error) {
		return 0, errors.New("schwab market data 503")
	}
	svc := newTestServiceWithAsk(trader, store, mail, ask)

	tr := sampleTrade()
	tr.EstimatedPrice = 0.80
	if _, err := runSinglePick(svc, tr, 101); err != nil {
		t.Fatalf("handleSinglePick: %v", err)
	}
	// 0.80 * 1.05 = 0.84
	if len(trader.placed) != 1 || trader.placed[0].Price != 0.84 {
		t.Errorf("Price: want 0.84 (fallback to estimate), got %+v", trader.placed)
	}
}

func TestHandleSinglePick_LimitClampedToMaxContractPremium(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{
		placeID: "ORDER-CLAMP",
		status:  OrderStatus{OrderID: "ORDER-CLAMP", RawStatus: "WORKING", Working: true},
	}}
	ask := func(_ context.Context, _, _, _ string, _ float64) (float64, error) {
		return 4.95, nil // 4.95 * 1.05 = 5.1975 → clamp to 5.00
	}
	svc := newTestServiceWithAsk(trader, store, mail, ask)

	if _, err := runSinglePick(svc, sampleTrade(), 102); err != nil {
		t.Fatalf("handleSinglePick: %v", err)
	}
	if len(trader.placed) != 1 || trader.placed[0].Price != MaxContractPremium {
		t.Errorf("Price: want %.2f (clamped), got %+v", MaxContractPremium, trader.placed)
	}
}

func TestHandleSinglePick_AbortsWhenAskAndEstimateBothZero(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{placeID: "should-not-be-called"}}
	ask := func(_ context.Context, _, _, _ string, _ float64) (float64, error) {
		return 0, errors.New("no quote")
	}
	svc := newTestServiceWithAsk(trader, store, mail, ask)

	tr := sampleTrade()
	tr.EstimatedPrice = 0
	if _, err := runSinglePick(svc, tr, 103); err == nil {
		t.Fatal("expected error when no usable price basis")
	}
	// No order reached the broker AND the operator was alerted.
	if len(trader.placed) != 0 {
		t.Errorf("expected no PlaceOrder call, but trader captured %+v", trader.placed)
	}
	if len(mail.sent) != 1 {
		t.Fatalf("expected 1 alert email, got %d", len(mail.sent))
	}
	if !strings.Contains(mail.sent[0].subject, "Open failed") {
		t.Errorf("subject: want 'Open failed' marker, got %q", mail.sent[0].subject)
	}
}

// ── HandleQualifyingPicks (basket) ────────────────────────────────

// countingTrader counts PlaceOrder invocations and the cost of each so
// the basket tests can assert exactly how many contracts the basket
// submitted.
type countingTrader struct {
	fakeTrader
	placed []Order
}

func (c *countingTrader) PlaceOrder(ctx context.Context, hash string, o Order) (string, error) {
	c.placed = append(c.placed, o)
	return c.fakeTrader.PlaceOrder(ctx, hash, o)
}

func basketSampleTrade(symbol string, rank int, est float64) trades.Trade {
	return trades.Trade{
		ID:             rank * 1000, // each rank gets a deterministic non-zero ID
		Symbol:         symbol,
		ContractType:   "CALL",
		StrikePrice:    100,
		Expiration:     "2026-06-19",
		EstimatedPrice: est,
		Rank:           rank,
	}
}

func TestHandleQualifyingPicks_PlacesEachWhenBasketFits(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{
		placeID: "ORDER-OK",
		status:  OrderStatus{OrderID: "ORDER-OK", RawStatus: "FILLED", Filled: true, Terminal: true, FillPrice: 1.25, FilledQuantity: 1},
	}}
	svc := newTestService(trader, store, mail)
	picks := []trades.Trade{
		basketSampleTrade("AKAM", 1, 1.25),
		basketSampleTrade("RKLB", 2, 1.69),
		basketSampleTrade("IREN", 3, 1.65),
	}

	count, err := svc.HandleQualifyingPicks(context.Background(), picks)
	if err != nil {
		t.Fatalf("HandleQualifyingPicks: %v", err)
	}
	if count != 3 {
		t.Fatalf("submitted: want 3, got %d", count)
	}
	if len(trader.placed) != 3 {
		t.Fatalf("PlaceOrder calls: want 3, got %d", len(trader.placed))
	}
}

func TestHandleQualifyingPicks_StopsWhenSchwabCashIsLow(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{
		availFunds: 200, // only $200 cash → only rank-1 fits, ranks 2 and 3 do not
		placeID:    "ORDER-OK",
		status:     OrderStatus{OrderID: "ORDER-OK", RawStatus: "FILLED", Filled: true, Terminal: true, FillPrice: 1.25, FilledQuantity: 1},
	}}
	svc := newTestService(trader, store, mail)
	// Limit prices: 1.25*1.05=1.31, 1.69*1.05=1.77, 1.65*1.05=1.73 (rounded).
	// Costs (×100): 131, 177, 173. With $200 budget: only 131 fits.
	picks := []trades.Trade{
		basketSampleTrade("AKAM", 1, 1.25),
		basketSampleTrade("RKLB", 2, 1.69),
		basketSampleTrade("IREN", 3, 1.65),
	}

	count, err := svc.HandleQualifyingPicks(context.Background(), picks)
	if err != nil {
		t.Fatalf("HandleQualifyingPicks: %v", err)
	}
	if count != 1 {
		t.Fatalf("submitted: want 1 (cash gate), got %d", count)
	}
	if len(trader.placed) != 1 {
		t.Fatalf("PlaceOrder calls: want 1, got %d", len(trader.placed))
	}
}

func TestHandleQualifyingPicks_AvailableFundsErrorAbortsBasket(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{
		availErr: errors.New("HTTP 401: token expired"),
	}}
	svc := newTestService(trader, store, mail)
	picks := []trades.Trade{
		basketSampleTrade("AKAM", 1, 1.25),
	}

	count, err := svc.HandleQualifyingPicks(context.Background(), picks)
	if err == nil {
		t.Fatal("expected error when AvailableFunds fails")
	}
	if count != 0 {
		t.Errorf("submitted: want 0 on cash-lookup failure, got %d", count)
	}
	if len(trader.placed) != 0 {
		t.Errorf("PlaceOrder must not be called when cash lookup fails, got %d calls", len(trader.placed))
	}
}

func TestHandleQualifyingPicks_SkipsContractsWithoutTradeID(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{
		placeID: "ORDER-OK",
		status:  OrderStatus{OrderID: "ORDER-OK", RawStatus: "FILLED", Filled: true, Terminal: true, FillPrice: 1.25, FilledQuantity: 1},
	}}
	svc := newTestService(trader, store, mail)
	picks := []trades.Trade{
		basketSampleTrade("AKAM", 1, 1.25),
		// Rank-2 missing ID — should be skipped without aborting the basket.
		{Symbol: "RKLB", ContractType: "CALL", StrikePrice: 90, Expiration: "2026-06-19", EstimatedPrice: 1.69, Rank: 2},
		basketSampleTrade("IREN", 3, 1.65),
	}

	count, _ := svc.HandleQualifyingPicks(context.Background(), picks)
	if count != 2 {
		t.Errorf("submitted: want 2 (middle pick skipped for missing ID), got %d", count)
	}
}

func TestHandleQualifyingPicks_EmptyInputIsNoOp(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{}
	svc := newTestService(trader, store, mail)

	count, err := svc.HandleQualifyingPicks(context.Background(), nil)
	if err != nil || count != 0 {
		t.Errorf("nil input: want (0, nil), got (%d, %v)", count, err)
	}
	if len(trader.placed) != 0 {
		t.Errorf("PlaceOrder must not be called for empty basket")
	}
}

func TestHandleSinglePick_WorkingPersistsOrderID(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &countingTrader{fakeTrader: fakeTrader{
		placeID: "ORDER-WORK",
		status: OrderStatus{
			OrderID:   "ORDER-WORK",
			RawStatus: "WORKING",
			Working:   true,
		},
	}}
	svc := newTestService(trader, store, mail)

	if _, err := runSinglePick(svc, sampleTrade(), 5); err != nil {
		t.Fatalf("handleSinglePick: %v", err)
	}
	// 2 updates: orphan-prevention working write, then redundant
	// working write after GetOrder confirms broker still working. The
	// second write is harmless (same row state) and the test asserts
	// both for clarity.
	if len(store.updates) != 2 {
		t.Fatalf("expected 2 status updates, got %d", len(store.updates))
	}
	for i, upd := range store.updates {
		if upd.status != "working" || upd.orderID != "ORDER-WORK" {
			t.Errorf("update[%d]: want status=working orderID=ORDER-WORK, got %+v", i, upd)
		}
	}
	if len(mail.sent) != 0 {
		t.Errorf("expected no email on working state, got %d", len(mail.sent))
	}
}

// ── RepriceWorkingOpens ───────────────────────────────────────────

/*
repriceTrader is a richer fake than fakeTrader, with per-order status
lookup, captured cancels, and a queue of PlaceOrder return ids so a
single test can simulate the cancel-old + place-new round trip.
*/
type repriceTrader struct {
	statusByOrder  map[string]OrderStatus
	statusGetErr   error
	canceled       []string
	cancelErr      error
	placeIDs       []string
	placedOrders   []Order
	placeErr       error
	placeStatusNew OrderStatus
}

func (r *repriceTrader) AccountHash(context.Context) (string, error) { return "ACCT-HASH", nil }
func (r *repriceTrader) AvailableFunds(context.Context, string) (float64, error) {
	return 1e6, nil
}
func (r *repriceTrader) PlaceOrder(_ context.Context, _ string, o Order) (string, error) {
	if r.placeErr != nil {
		return "", r.placeErr
	}
	if len(r.placeIDs) == 0 {
		return "", errors.New("repriceTrader: no placeIDs left")
	}
	id := r.placeIDs[0]
	r.placeIDs = r.placeIDs[1:]
	r.placedOrders = append(r.placedOrders, o)
	if r.statusByOrder == nil {
		r.statusByOrder = map[string]OrderStatus{}
	}
	st := r.placeStatusNew
	st.OrderID = id
	r.statusByOrder[id] = st
	return id, nil
}
func (r *repriceTrader) GetOrder(_ context.Context, _, orderID string) (OrderStatus, error) {
	if r.statusGetErr != nil {
		return OrderStatus{}, r.statusGetErr
	}
	st, ok := r.statusByOrder[orderID]
	if !ok {
		return OrderStatus{}, errors.New("repriceTrader: no status for " + orderID)
	}
	return st, nil
}
func (r *repriceTrader) CancelOrder(_ context.Context, _, orderID string) error {
	r.canceled = append(r.canceled, orderID)
	return r.cancelErr
}

func workingPositionFixture(execID int, tradeID int, orderID string, contractPrice float64) OpenPosition {
	oid := orderID
	return OpenPosition{
		Execution: Execution{
			ID:                execID,
			TradeID:           tradeID,
			Mode:              "live",
			Side:              "open",
			SchwabOrderID:     &oid,
			Status:            "working",
			RequestedQuantity: 1,
		},
		Symbol:        "AAPL",
		ContractType:  "CALL",
		StrikePrice:   150,
		Expiration:    "2026-06-19",
		ContractPrice: contractPrice,
	}
}

func askFn(price float64, err error) func(context.Context, string, string, string, float64) (float64, error) {
	return func(context.Context, string, string, string, float64) (float64, error) {
		return price, err
	}
}

func TestRepriceWorkingOpens_NoOpWhenNewLimitNotHigher(t *testing.T) {
	store := &fakeStore{
		workingPositions: []OpenPosition{workingPositionFixture(7, 42, "ORDER-OLD", 0.92)},
	}
	mail := &fakeMail{}
	trader := &repriceTrader{
		statusByOrder: map[string]OrderStatus{
			"ORDER-OLD": {OrderID: "ORDER-OLD", RawStatus: "WORKING", Working: true, LimitPrice: 0.95},
		},
	}
	// fresh ask × 1.05 = 0.90 × 1.05 = 0.945 → rounds to 0.95, equal to old.
	svc := newTestServiceWithAsk(trader, store, mail, askFn(0.90, nil))

	svc.RepriceWorkingOpens(context.Background(), "2026-05-13")

	if len(trader.canceled) != 0 {
		t.Errorf("expected no cancels on no-op, got %v", trader.canceled)
	}
	if len(trader.placedOrders) != 0 {
		t.Errorf("expected no replacement orders on no-op, got %d", len(trader.placedOrders))
	}
	if len(store.updates) != 0 {
		t.Errorf("expected no DB writes on no-op, got %d (%+v)", len(store.updates), store.updates)
	}
	if len(mail.sent) != 0 {
		t.Errorf("expected no emails on no-op, got %d", len(mail.sent))
	}
}

func TestRepriceWorkingOpens_CancelAndReplaceWhenAskMoved(t *testing.T) {
	store := &fakeStore{
		workingPositions: []OpenPosition{workingPositionFixture(7, 42, "ORDER-OLD", 0.92)},
	}
	mail := &fakeMail{}
	trader := &repriceTrader{
		statusByOrder: map[string]OrderStatus{
			"ORDER-OLD": {OrderID: "ORDER-OLD", RawStatus: "WORKING", Working: true, LimitPrice: 0.92},
		},
		placeIDs:       []string{"ORDER-NEW"},
		placeStatusNew: OrderStatus{RawStatus: "WORKING", Working: true},
	}
	// 2.40 × 1.05 = 2.52 → well above old 0.92 and below the $5 cap.
	svc := newTestServiceWithAsk(trader, store, mail, askFn(2.40, nil))

	svc.RepriceWorkingOpens(context.Background(), "2026-05-13")

	if got := trader.canceled; len(got) != 1 || got[0] != "ORDER-OLD" {
		t.Fatalf("expected cancel of ORDER-OLD, got %v", got)
	}
	if len(trader.placedOrders) != 1 {
		t.Fatalf("expected one replacement order placed, got %d", len(trader.placedOrders))
	}
	if got := trader.placedOrders[0].Price; got != 2.52 {
		t.Errorf("replacement limit: want 2.52, got %.4f", got)
	}
	if got := trader.placedOrders[0].OrderType; got != "LIMIT" {
		t.Errorf("replacement order type: want LIMIT, got %q", got)
	}

	if len(store.inserts) != 1 {
		t.Fatalf("expected one new execution row, got %d", len(store.inserts))
	}
	newExec := store.inserts[0]
	if newExec.TradeID != 42 || newExec.Side != "open" || newExec.Mode != "live" {
		t.Errorf("new execution shape unexpected: %+v", newExec)
	}

	// Expected update sequence:
	//   1. old exec (id=7) → canceled with "repriced post-open ..." reason
	//   2. new exec (id=1) → working with ORDER-NEW
	//   3. new exec (id=1) → working with ORDER-NEW (post-place GetOrder confirmation)
	if len(store.updates) < 2 {
		t.Fatalf("expected at least 2 status updates, got %d (%+v)", len(store.updates), store.updates)
	}
	if store.updates[0].id != 7 || store.updates[0].status != "canceled" || store.updates[0].orderID != "ORDER-OLD" {
		t.Errorf("update[0]: want id=7 canceled ORDER-OLD, got %+v", store.updates[0])
	}
	if !strings.Contains(store.updates[0].errMessage, "repriced post-open") {
		t.Errorf("update[0] error message should mention repriced post-open, got %q", store.updates[0].errMessage)
	}
	if store.updates[1].status != "working" || store.updates[1].orderID != "ORDER-NEW" {
		t.Errorf("update[1]: want working ORDER-NEW, got %+v", store.updates[1])
	}
	if len(mail.sent) != 0 {
		t.Errorf("expected no emails on healthy reprice, got %d", len(mail.sent))
	}
}

func TestRepriceWorkingOpens_CancelWithoutReplaceWhenOverCap(t *testing.T) {
	store := &fakeStore{
		workingPositions: []OpenPosition{workingPositionFixture(7, 42, "ORDER-OLD", 3.08)},
	}
	mail := &fakeMail{}
	trader := &repriceTrader{
		statusByOrder: map[string]OrderStatus{
			"ORDER-OLD": {OrderID: "ORDER-OLD", RawStatus: "WORKING", Working: true, LimitPrice: 3.08},
		},
	}
	// 14.60 × 1.05 = 15.33 → far above the $5 single-contract cap.
	svc := newTestServiceWithAsk(trader, store, mail, askFn(14.60, nil))

	svc.RepriceWorkingOpens(context.Background(), "2026-05-13")

	if got := trader.canceled; len(got) != 1 || got[0] != "ORDER-OLD" {
		t.Fatalf("expected cancel of ORDER-OLD, got %v", got)
	}
	if len(trader.placedOrders) != 0 {
		t.Errorf("expected no replacement order when over cap, got %d", len(trader.placedOrders))
	}
	if len(store.inserts) != 0 {
		t.Errorf("expected no new execution row when over cap, got %d", len(store.inserts))
	}
	if len(store.updates) != 1 {
		t.Fatalf("expected 1 status update, got %d (%+v)", len(store.updates), store.updates)
	}
	upd := store.updates[0]
	if upd.id != 7 || upd.status != "canceled" || upd.orderID != "ORDER-OLD" {
		t.Errorf("update: want id=7 canceled ORDER-OLD, got %+v", upd)
	}
	if !strings.Contains(upd.errMessage, "exceeds single-contract cap") {
		t.Errorf("update reason should mention single-contract cap exceed, got %q", upd.errMessage)
	}
	if len(mail.sent) != 1 {
		t.Fatalf("expected 1 operator email on cap-exceed cancel, got %d", len(mail.sent))
	}
}

func TestRepriceWorkingOpens_LeavesAloneWhenAlreadyFilled(t *testing.T) {
	store := &fakeStore{
		workingPositions: []OpenPosition{workingPositionFixture(7, 42, "ORDER-OLD", 0.92)},
	}
	mail := &fakeMail{}
	trader := &repriceTrader{
		statusByOrder: map[string]OrderStatus{
			"ORDER-OLD": {OrderID: "ORDER-OLD", RawStatus: "FILLED", Filled: true, Terminal: true, LimitPrice: 0.92},
		},
	}
	svc := newTestServiceWithAsk(trader, store, mail, askFn(2.40, nil))

	svc.RepriceWorkingOpens(context.Background(), "2026-05-13")

	if len(trader.canceled) != 0 {
		t.Errorf("expected no cancel on already-filled order, got %v", trader.canceled)
	}
	if len(trader.placedOrders) != 0 {
		t.Errorf("expected no replacement on already-filled order, got %d", len(trader.placedOrders))
	}
	if len(store.updates) != 0 {
		t.Errorf("expected no DB writes (reconcile owns the filled flip), got %d", len(store.updates))
	}
}

func TestRepriceWorkingOpens_LeavesAloneWhenFreshAskMissing(t *testing.T) {
	store := &fakeStore{
		workingPositions: []OpenPosition{workingPositionFixture(7, 42, "ORDER-OLD", 0.92)},
	}
	mail := &fakeMail{}
	trader := &repriceTrader{
		statusByOrder: map[string]OrderStatus{
			"ORDER-OLD": {OrderID: "ORDER-OLD", RawStatus: "WORKING", Working: true, LimitPrice: 0.92},
		},
	}
	svc := newTestServiceWithAsk(trader, store, mail, askFn(0, nil))

	svc.RepriceWorkingOpens(context.Background(), "2026-05-13")

	if len(trader.canceled) != 0 || len(trader.placedOrders) != 0 || len(store.updates) != 0 {
		t.Errorf("expected total no-op when fresh ask is 0: canceled=%v placed=%d updates=%v", trader.canceled, len(trader.placedOrders), store.updates)
	}
}

func TestRepriceWorkingOpens_NoOpWhenNoWorkingPositions(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeMail{}
	trader := &repriceTrader{}
	svc := newTestServiceWithAsk(trader, store, mail, askFn(2.40, nil))

	svc.RepriceWorkingOpens(context.Background(), "2026-05-13")

	if len(trader.canceled) != 0 || len(trader.placedOrders) != 0 || len(store.updates) != 0 {
		t.Errorf("expected total no-op with empty positions")
	}
}

func TestRepriceWorkingOpens_SchwabGetOrderErrorDoesNotPanic(t *testing.T) {
	store := &fakeStore{
		workingPositions: []OpenPosition{workingPositionFixture(7, 42, "ORDER-OLD", 0.92)},
	}
	mail := &fakeMail{}
	trader := &repriceTrader{
		statusGetErr: errors.New("schwab 503"),
	}
	svc := newTestServiceWithAsk(trader, store, mail, askFn(2.40, nil))

	// Must not panic and must not act when broker status is unreadable.
	svc.RepriceWorkingOpens(context.Background(), "2026-05-13")

	if len(trader.canceled) != 0 || len(trader.placedOrders) != 0 || len(store.updates) != 0 {
		t.Errorf("expected no actions on Schwab GetOrder error")
	}
}

func TestRepriceWorkingOpens_NoOpWhenOptionAskNotConfigured(t *testing.T) {
	store := &fakeStore{
		workingPositions: []OpenPosition{workingPositionFixture(7, 42, "ORDER-OLD", 0.92)},
	}
	mail := &fakeMail{}
	trader := &repriceTrader{
		statusByOrder: map[string]OrderStatus{
			"ORDER-OLD": {OrderID: "ORDER-OLD", RawStatus: "WORKING", Working: true, LimitPrice: 0.92},
		},
	}
	// OptionAsk left nil — reprice must short-circuit without touching the broker.
	svc := newTestService(trader, store, mail)

	svc.RepriceWorkingOpens(context.Background(), "2026-05-13")

	if len(trader.canceled) != 0 || len(trader.placedOrders) != 0 || len(store.updates) != 0 {
		t.Errorf("expected no actions when OptionAsk is nil")
	}
}
