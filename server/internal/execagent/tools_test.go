package execagent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/trades"
)

/*
Tool-layer tests. These exercise the dispatcher directly without any
Claude tool loop, because the hard caps + allowlist enforcement live
HERE and must hold regardless of model behavior. Every refusal path
gets a direct test.

The dispatcher is the security boundary; any change that loosens a
gate must come with a test demonstrating the new behavior and an
updated rationale comment on the rule.
*/

func mkCandidate(rank int, symbol, kind string) Candidate {
	return Candidate{Trade: trades.Trade{
		ID:           rank, // tests use rank as a stable ID stand-in
		Rank:         rank,
		Symbol:       symbol,
		ContractType: kind,
		Score:        9,
	}}
}

type stubChain struct {
	quoteCalls int
	chainCalls int
	quoteErr   error
	chainErr   error
}

func (s *stubChain) GetQuotes(symbols []string) (map[string]schwab.StockQuote, error) {
	s.quoteCalls++
	if s.quoteErr != nil {
		return nil, s.quoteErr
	}
	out := make(map[string]schwab.StockQuote, len(symbols))
	for _, sym := range symbols {
		out[sym] = schwab.StockQuote{LastPrice: 100.0, BidPrice: 99.5, AskPrice: 100.5}
	}
	return out, nil
}

func (s *stubChain) GetOptionChain(symbol, contractType, fromDate, toDate string, strike float64) (*schwab.OptionChain, error) {
	s.chainCalls++
	if s.chainErr != nil {
		return nil, s.chainErr
	}
	return &schwab.OptionChain{Symbol: symbol, Status: "SUCCESS"}, nil
}

type stubExec struct {
	mu          sync.Mutex
	placeCalls  []placeArgs
	skipCalls   []skipArgs
	fundsCalls  int
	fundsValue  float64
	placeErr    error
	skipErr     error
	fundsErr    error
	nextOrderID int
}

type placeArgs struct {
	TradeID    int
	Strike     float64
	Expiration string
	DTE        int
	LimitPrice float64
	Quantity   int
}

type skipArgs struct {
	TradeID int
	Reason  string
}

func (s *stubExec) PlaceBuyToOpenAgent(_ context.Context, t *trades.Trade, strike float64, expiration string, dte int, limitPrice float64, quantity int) (string, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.placeErr != nil {
		return "", 0, s.placeErr
	}
	s.placeCalls = append(s.placeCalls, placeArgs{TradeID: t.ID, Strike: strike, Expiration: expiration, DTE: dte, LimitPrice: limitPrice, Quantity: quantity})
	s.nextOrderID++
	return "order-" + itoa(s.nextOrderID), 100 + s.nextOrderID, nil
}

func (s *stubExec) InsertSkippedExecutionAgent(tradeID int, reason string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.skipErr != nil {
		return 0, s.skipErr
	}
	s.skipCalls = append(s.skipCalls, skipArgs{TradeID: tradeID, Reason: reason})
	return 900 + len(s.skipCalls), nil
}

func (s *stubExec) AvailableFundsAgent(_ context.Context) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fundsCalls++
	if s.fundsErr != nil {
		return 0, s.fundsErr
	}
	return s.fundsValue, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b strings.Builder
	for n > 0 {
		b.WriteByte(byte('0' + n%10))
		n /= 10
	}
	r := []byte(b.String())
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// mustExtractErr asserts the tool response is a JSON {"error": ...}
// and returns the error string. Fails the test if the response isn't
// shaped that way — which is itself a regression signal.
func mustExtractErr(t *testing.T, raw string) string {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("tool response is not JSON: %v\nraw=%s", err, raw)
	}
	v, ok := parsed["error"]
	if !ok {
		t.Fatalf("tool response is not an error: %s", raw)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("tool error is not a string: %v", v)
	}
	return s
}

func mustExtractOK(t *testing.T, raw string) map[string]any {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("tool response is not JSON: %v\nraw=%s", err, raw)
	}
	if _, isErr := parsed["error"]; isErr {
		t.Fatalf("expected ok response, got error: %s", raw)
	}
	return parsed
}

func TestPlaceOptionsOrder_HappyPath(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL"), mkCandidate(2, "TSLA", "PUT"), mkCandidate(3, "MSFT", "CALL")}
	exec := &stubExec{}
	d := NewToolDispatcher(&stubChain{}, exec, candidates)

	input := []byte(`{"rank":1,"symbol":"AAPL","contract_type":"CALL","strike":150,"expiration":"2099-01-01","limit_price":2.30}`)
	out := d.Dispatch(context.Background(), "place_options_order", input)
	parsed := mustExtractOK(t, out)

	if parsed["order_id"] != "order-1" {
		t.Errorf("expected order-1, got %v", parsed["order_id"])
	}
	if parsed["orders_left"].(float64) != 2 {
		t.Errorf("expected 2 orders left, got %v", parsed["orders_left"])
	}
	// Omitted quantity defaults to 1; exposure = 2.30 × 100 × 1 = 230.
	if parsed["quantity"].(float64) != 1 {
		t.Errorf("expected quantity 1 default, got %v", parsed["quantity"])
	}
	if got := parsed["budget_remaining"].(float64); got != MaxDailyExposure-230 {
		t.Errorf("expected budget_remaining %.2f, got %v", MaxDailyExposure-230, got)
	}
	if len(exec.placeCalls) != 1 || exec.placeCalls[0].Strike != 150 || exec.placeCalls[0].Quantity != 1 {
		t.Fatalf("place call not recorded as expected: %+v", exec.placeCalls)
	}
}

func TestPlaceOptionsOrder_RefusesQuantityAboveOne(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL")}
	exec := &stubExec{}
	d := NewToolDispatcher(&stubChain{}, exec, candidates)

	// Sizing up is gone: any quantity above 1 is refused at the tool layer.
	input := []byte(`{"rank":1,"symbol":"AAPL","contract_type":"CALL","strike":150,"expiration":"2099-01-01","limit_price":2.00,"quantity":3}`)
	errStr := mustExtractErr(t, d.Dispatch(context.Background(), "place_options_order", input))
	if !strings.Contains(errStr, "exceeds the cap of 1 contract") {
		t.Errorf("expected per-pick quantity-cap refusal, got %q", errStr)
	}
	if len(exec.placeCalls) != 0 {
		t.Fatalf("a quantity>1 order must not reach the broker, got %+v", exec.placeCalls)
	}
}

func TestPlaceOptionsOrder_SingleContractExposureAndBudget(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL")}
	exec := &stubExec{}
	d := NewToolDispatcher(&stubChain{}, exec, candidates)

	// One contract at $2.00 = $200 exposure; quantity defaults to 1.
	input := []byte(`{"rank":1,"symbol":"AAPL","contract_type":"CALL","strike":150,"expiration":"2099-01-01","limit_price":2.00}`)
	parsed := mustExtractOK(t, d.Dispatch(context.Background(), "place_options_order", input))

	if parsed["quantity"].(float64) != 1 {
		t.Errorf("expected quantity 1, got %v", parsed["quantity"])
	}
	if parsed["order_exposure"].(float64) != 200 {
		t.Errorf("expected order_exposure 200, got %v", parsed["order_exposure"])
	}
	if got := parsed["budget_remaining"].(float64); got != MaxDailyExposure-200 {
		t.Errorf("expected budget_remaining %.2f, got %v", MaxDailyExposure-200, got)
	}
	if len(exec.placeCalls) != 1 || exec.placeCalls[0].Quantity != 1 {
		t.Fatalf("expected exactly one single-contract broker call, got %+v", exec.placeCalls)
	}
}

func TestPlaceOptionsOrder_BudgetAccumulatesAcrossRanks(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL"), mkCandidate(2, "TSLA", "PUT")}
	exec := &stubExec{}
	d := NewToolDispatcher(&stubChain{}, exec, candidates)

	// Rank 1: one contract at $5.00 = $500 committed.
	first := []byte(`{"rank":1,"symbol":"AAPL","contract_type":"CALL","strike":150,"expiration":"2099-01-01","limit_price":5.00}`)
	p1 := mustExtractOK(t, d.Dispatch(context.Background(), "place_options_order", first))
	if got := p1["budget_remaining"].(float64); got != MaxDailyExposure-500 {
		t.Fatalf("after first order expected %.2f remaining, got %v", MaxDailyExposure-500, got)
	}

	// Rank 2: one contract at $3.00 = $300; the basket's combined price never
	// blocks taking a pick — both single contracts go through.
	second := []byte(`{"rank":2,"symbol":"TSLA","contract_type":"PUT","strike":250,"expiration":"2099-01-01","limit_price":3.00}`)
	p2 := mustExtractOK(t, d.Dispatch(context.Background(), "place_options_order", second))
	if got := p2["budget_remaining"].(float64); got != MaxDailyExposure-800 {
		t.Errorf("after second order expected %.2f remaining, got %v", MaxDailyExposure-800, got)
	}
	if len(exec.placeCalls) != 2 {
		t.Fatalf("expected exactly 2 successful broker calls, got %d", len(exec.placeCalls))
	}
}

func TestPlaceOptionsOrder_RefusesOffListSymbol(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL")}
	d := NewToolDispatcher(&stubChain{}, &stubExec{}, candidates)

	input := []byte(`{"rank":1,"symbol":"GME","contract_type":"CALL","strike":50,"expiration":"2099-01-01","limit_price":1.00}`)
	out := d.Dispatch(context.Background(), "place_options_order", input)
	msg := mustExtractErr(t, out)
	if !strings.Contains(msg, "does not match candidate") {
		t.Errorf("expected symbol-mismatch error, got: %s", msg)
	}
}

func TestPlaceOptionsOrder_RefusesContractTypeMismatch(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL")}
	d := NewToolDispatcher(&stubChain{}, &stubExec{}, candidates)

	input := []byte(`{"rank":1,"symbol":"AAPL","contract_type":"PUT","strike":150,"expiration":"2099-01-01","limit_price":2.30}`)
	out := d.Dispatch(context.Background(), "place_options_order", input)
	msg := mustExtractErr(t, out)
	if !strings.Contains(msg, "contract_type") {
		t.Errorf("expected contract_type mismatch error, got: %s", msg)
	}
}

func TestPlaceOptionsOrder_RefusesDuplicateRank(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL")}
	d := NewToolDispatcher(&stubChain{}, &stubExec{}, candidates)

	first := []byte(`{"rank":1,"symbol":"AAPL","contract_type":"CALL","strike":150,"expiration":"2099-01-01","limit_price":1.00}`)
	_ = d.Dispatch(context.Background(), "place_options_order", first)

	second := []byte(`{"rank":1,"symbol":"AAPL","contract_type":"CALL","strike":155,"expiration":"2099-01-01","limit_price":1.00}`)
	out := d.Dispatch(context.Background(), "place_options_order", second)
	msg := mustExtractErr(t, out)
	if !strings.Contains(msg, "already placed") {
		t.Errorf("expected already-placed error, got: %s", msg)
	}
}

func TestPlaceOptionsOrder_RefusesFourthOrder(t *testing.T) {
	// Construct a synthetic 4-rank input even though MaxOrdersPerRun=3,
	// to prove the count gate catches the 4th attempt independent of
	// the per-rank dedup gate.
	candidates := []Candidate{
		mkCandidate(1, "AAPL", "CALL"),
		mkCandidate(2, "TSLA", "PUT"),
		mkCandidate(3, "MSFT", "CALL"),
	}
	d := NewToolDispatcher(&stubChain{}, &stubExec{}, candidates)

	for i, sym := range []string{"AAPL", "TSLA", "MSFT"} {
		args := []byte(`{"rank":` + itoa(i+1) + `,"symbol":"` + sym + `","contract_type":"` + map[string]string{"AAPL": "CALL", "TSLA": "PUT", "MSFT": "CALL"}[sym] + `","strike":100,"expiration":"2099-01-01","limit_price":1.00}`)
		out := d.Dispatch(context.Background(), "place_options_order", args)
		mustExtractOK(t, out)
	}

	// Fourth call — invent a phantom rank 4 (allowed-rank check will
	// fail before the count check, but the count check is the backstop
	// for any future relaxation).
	args := []byte(`{"rank":4,"symbol":"AAPL","contract_type":"CALL","strike":100,"expiration":"2099-01-01","limit_price":1.00}`)
	out := d.Dispatch(context.Background(), "place_options_order", args)
	msg := mustExtractErr(t, out)
	if !strings.Contains(msg, "outside") && !strings.Contains(msg, "already placed") && !strings.Contains(msg, "max") {
		t.Errorf("expected a refusal of some kind on the 4th order, got: %s", msg)
	}
}

func TestPlaceOptionsOrder_RefusesOverCapLimitPrice(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL")}
	d := NewToolDispatcher(&stubChain{}, &stubExec{}, candidates)

	input := []byte(`{"rank":1,"symbol":"AAPL","contract_type":"CALL","strike":150,"expiration":"2099-01-01","limit_price":10.01}`)
	out := d.Dispatch(context.Background(), "place_options_order", input)
	msg := mustExtractErr(t, out)
	if !strings.Contains(msg, "cap") {
		t.Errorf("expected over-cap error, got: %s", msg)
	}
}

func TestPlaceOptionsOrder_RefusesNonPositiveLimit(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL")}
	d := NewToolDispatcher(&stubChain{}, &stubExec{}, candidates)

	for _, lp := range []string{"0", "-1.50"} {
		input := []byte(`{"rank":1,"symbol":"AAPL","contract_type":"CALL","strike":150,"expiration":"2099-01-01","limit_price":` + lp + `}`)
		out := d.Dispatch(context.Background(), "place_options_order", input)
		msg := mustExtractErr(t, out)
		if !strings.Contains(msg, "limit_price") {
			t.Errorf("limit_price=%s: expected limit_price error, got: %s", lp, msg)
		}
	}
}

func TestPlaceOptionsOrder_RefusesBadExpiration(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL")}
	d := NewToolDispatcher(&stubChain{}, &stubExec{}, candidates)

	for _, exp := range []string{"", "2099/01/01", "tomorrow"} {
		input := []byte(`{"rank":1,"symbol":"AAPL","contract_type":"CALL","strike":150,"expiration":"` + exp + `","limit_price":1.00}`)
		out := d.Dispatch(context.Background(), "place_options_order", input)
		_ = mustExtractErr(t, out)
	}
}

func TestPlaceOptionsOrder_PropagatesUnderlyingError(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL")}
	exec := &stubExec{placeErr: errors.New("broker rejected: insufficient buying power")}
	d := NewToolDispatcher(&stubChain{}, exec, candidates)

	input := []byte(`{"rank":1,"symbol":"AAPL","contract_type":"CALL","strike":150,"expiration":"2099-01-01","limit_price":2.30}`)
	out := d.Dispatch(context.Background(), "place_options_order", input)
	msg := mustExtractErr(t, out)
	if !strings.Contains(msg, "insufficient buying power") {
		t.Errorf("expected broker error to be surfaced, got: %s", msg)
	}
}

func TestGetOptionChain_RefusesOffListSymbol(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL")}
	chain := &stubChain{}
	d := NewToolDispatcher(chain, &stubExec{}, candidates)

	input := []byte(`{"symbol":"GME","contract_type":"CALL"}`)
	out := d.Dispatch(context.Background(), "get_option_chain", input)
	msg := mustExtractErr(t, out)
	if !strings.Contains(msg, "not a candidate") {
		t.Errorf("expected off-list-symbol error, got: %s", msg)
	}
	if chain.chainCalls != 0 {
		t.Errorf("underlying chain fetcher should not have been called, got %d", chain.chainCalls)
	}
}

func TestGetStockQuotes_AllowsAnySymbol(t *testing.T) {
	// Equity quotes are unconstrained — Claude may want to read a peer
	// (e.g., NVDA) when evaluating an AMD candidate, even though NVDA
	// isn't on the allowlist. Only the EXECUTION tools enforce the
	// allowlist; analysis tools don't.
	candidates := []Candidate{mkCandidate(1, "AMD", "CALL")}
	d := NewToolDispatcher(&stubChain{}, &stubExec{}, candidates)

	input := []byte(`{"symbols":"NVDA,AMD"}`)
	out := d.Dispatch(context.Background(), "get_stock_quotes", input)
	if _, isErr := tryParseError(out); isErr {
		t.Errorf("expected ok, got error: %s", out)
	}
}

func tryParseError(raw string) (string, bool) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", false
	}
	v, ok := parsed["error"]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func TestUnknownToolName(t *testing.T) {
	d := NewToolDispatcher(&stubChain{}, &stubExec{}, nil)
	out := d.Dispatch(context.Background(), "drop_table_executions", []byte(`{}`))
	msg := mustExtractErr(t, out)
	if !strings.Contains(msg, "unknown tool") {
		t.Errorf("expected unknown-tool error, got: %s", msg)
	}
}

func TestRecordSkip_IdempotentPerRank(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL")}
	exec := &stubExec{}
	d := NewToolDispatcher(&stubChain{}, exec, candidates)

	id1, err := d.RecordSkip(1, "thesis broke")
	if err != nil {
		t.Fatalf("first skip: %v", err)
	}
	id2, err := d.RecordSkip(1, "still broke")
	if err != nil {
		t.Fatalf("second skip: %v", err)
	}
	if id1 != id2 {
		t.Errorf("expected idempotent skip, got %d vs %d", id1, id2)
	}
	if len(exec.skipCalls) != 1 {
		t.Errorf("expected one underlying skip persist, got %d", len(exec.skipCalls))
	}
}

func TestRecordSkip_EmptyReasonSubstituted(t *testing.T) {
	candidates := []Candidate{mkCandidate(1, "AAPL", "CALL")}
	exec := &stubExec{}
	d := NewToolDispatcher(&stubChain{}, exec, candidates)

	_, err := d.RecordSkip(1, "   ")
	if err != nil {
		t.Fatalf("skip with empty reason: %v", err)
	}
	if len(exec.skipCalls) != 1 || exec.skipCalls[0].Reason != "(no reason provided)" {
		t.Errorf("expected synthetic reason, got: %+v", exec.skipCalls)
	}
}

func TestGetAccountFunds_DispatchesUnderlying(t *testing.T) {
	exec := &stubExec{fundsValue: 4321.50}
	d := NewToolDispatcher(&stubChain{}, exec, []Candidate{mkCandidate(1, "AAPL", "CALL")})

	out := d.Dispatch(context.Background(), "get_account_funds", []byte(`{}`))
	parsed := mustExtractOK(t, out)
	if parsed["available_usd"].(float64) != 4321.50 {
		t.Errorf("expected 4321.50, got %v", parsed["available_usd"])
	}
}
