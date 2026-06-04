package portfolio

import (
	"context"
	"encoding/json"
	"testing"

	"vibetradez.com/internal/schwab"
)

/*
fakeReader / fakeExec stub the two interfaces. fakeExec records how many
broker calls actually happened so a test can assert that a cap rejection
NEVER reaches the broker (the whole point of the tool layer being the
security boundary).
*/
type fakeReader struct {
	liq LiquidityCtx
}

func (f *fakeReader) Snapshot(context.Context) (Snapshot, error) { return Snapshot{}, nil }
func (f *fakeReader) GetQuotes([]string) (map[string]schwab.StockQuote, error) {
	return map[string]schwab.StockQuote{}, nil
}
func (f *fakeReader) GetOptionChain(string, string, string, string, float64) (*schwab.OptionChain, error) {
	return &schwab.OptionChain{}, nil
}
func (f *fakeReader) Liquidity(context.Context, Move) (LiquidityCtx, error) { return f.liq, nil }
func (f *fakeReader) GetDailyPriceHistory(string) (*schwab.PriceHistory, error) {
	return &schwab.PriceHistory{Candles: []schwab.Candle{{Close: 100, High: 100, Low: 100}}}, nil
}
func (f *fakeReader) GetFundamentals(string) (*schwab.Fundamentals, error) {
	return &schwab.Fundamentals{}, nil
}
func (f *fakeReader) RecentDecisions(int) ([]HistoryEntry, error) { return nil, nil }
func (f *fakeReader) RecentStances(int) ([]StanceEntry, error)    { return nil, nil }
func (f *fakeReader) OrderStatus(context.Context, string) (OrderStatus, error) {
	return OrderStatus{}, nil
}
func (f *fakeReader) PriorSession() (string, string, bool, error) { return "", "", false, nil }

type fakeExec struct {
	buyEquityCalls int
}

func (f *fakeExec) BuyEquity(context.Context, string, float64, float64) (string, int, error) {
	f.buyEquityCalls++
	return "ord-1", 101, nil
}
func (f *fakeExec) SellEquity(context.Context, string, float64, float64) (string, int, error) {
	return "ord-s", 102, nil
}
func (f *fakeExec) BuyOption(context.Context, string, string, string, float64, string, int, float64) (string, int, error) {
	return "ord-o", 103, nil
}
func (f *fakeExec) SellOption(context.Context, string, int, float64) (string, int, error) {
	return "ord-so", 104, nil
}
func (f *fakeExec) CancelOrder(context.Context, string) error { return nil }

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func isErrResult(s string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return false
	}
	_, ok := m["error"]
	return ok
}

// A buy that violates a cap must be refused at the tool layer and must NOT
// reach the broker.
func TestDispatch_CapRejectionNeverHitsBroker(t *testing.T) {
	reader := &fakeReader{liq: goodLiquidity()}
	ex := &fakeExec{}
	snap := baseSnapshot() // $6k, per-order cap $1,800
	d := NewToolDispatcher(reader, ex, DefaultCaps(), snap)

	// $2,000 order > $1,800 per-order cap.
	res := d.Dispatch(context.Background(), "buy_equity", mustJSON(t, map[string]any{
		"symbol": "MDT", "quantity": 40.0, "limit_price": 50.0, "rationale": "test",
	}))
	if !isErrResult(res) {
		t.Fatalf("expected a refusal, got: %s", res)
	}
	if ex.buyEquityCalls != 0 {
		t.Fatalf("cap-rejected order must not reach the broker, got %d calls", ex.buyEquityCalls)
	}
	if len(d.Decisions()) != 0 {
		t.Fatalf("rejected order must not record a decision, got %d", len(d.Decisions()))
	}
}

// Successful buys update the working snapshot so a later buy is gated
// against cumulative deployment, not the pristine session state.
func TestDispatch_CumulativeDeploymentAcrossMoves(t *testing.T) {
	reader := &fakeReader{liq: goodLiquidity()}
	ex := &fakeExec{}
	snap := baseSnapshot() // deployment budget $3,000
	d := NewToolDispatcher(reader, ex, DefaultCaps(), snap)

	// First $1,500 buy: allowed.
	r1 := d.Dispatch(context.Background(), "buy_equity", mustJSON(t, map[string]any{
		"symbol": "AAA", "quantity": 100.0, "limit_price": 15.0, "rationale": "first",
	}))
	if isErrResult(r1) {
		t.Fatalf("first buy should pass, got: %s", r1)
	}
	// Second $1,600 buy: would push deployment to $3,100 > $3,000 budget.
	r2 := d.Dispatch(context.Background(), "buy_equity", mustJSON(t, map[string]any{
		"symbol": "BBB", "quantity": 100.0, "limit_price": 16.0, "rationale": "second",
	}))
	if !isErrResult(r2) {
		t.Fatalf("second buy should be refused on cumulative deployment, got: %s", r2)
	}
	if ex.buyEquityCalls != 1 {
		t.Fatalf("exactly one buy should have reached the broker, got %d", ex.buyEquityCalls)
	}
	decs := d.Decisions()
	if len(decs) != 1 || decs[0].Symbol != "AAA" {
		t.Fatalf("expected one recorded decision for AAA, got %+v", decs)
	}
}

func TestDispatch_CapHeadroom(t *testing.T) {
	snap := baseSnapshot() // $6k equity, $3k deployment budget
	snap.Positions = []Position{{Symbol: "MDT", Underlying: "MDT", AssetType: AssetEquity, MarkValue: 1500, Quantity: 30}}
	d := NewToolDispatcher(&fakeReader{liq: goodLiquidity()}, &fakeExec{}, DefaultCaps(), snap)

	res := d.Dispatch(context.Background(), "get_cap_headroom", mustJSON(t, map[string]any{}))
	var out map[string]any
	if err := json.Unmarshal([]byte(res), &out); err != nil {
		t.Fatalf("bad json: %s", res)
	}
	// Per-order cap = 30% of $6,000 = $1,800.
	if out["per_order_cap"].(float64) != 1800 {
		t.Fatalf("expected per_order_cap 1800, got %v", out["per_order_cap"])
	}
	// Per-name cap = 40% of $6,000 = $2,400; MDT exposure $1,500, remaining $900.
	names := out["held_name_exposure"].([]any)
	if len(names) != 1 {
		t.Fatalf("expected 1 held name, got %v", names)
	}
	mdt := names[0].(map[string]any)
	if mdt["remaining"].(float64) != 900 {
		t.Fatalf("expected MDT remaining 900, got %v", mdt["remaining"])
	}
}

func TestPriceSummary(t *testing.T) {
	// Strictly rising series so the 1-month return is positive and the last
	// close sits at the 52-week high (0% from high).
	candles := make([]schwab.Candle, 60)
	for i := range candles {
		px := 100 + float64(i)
		candles[i] = schwab.Candle{Open: px, High: px, Low: px, Close: px}
	}
	s := priceSummary("TEST", &schwab.PriceHistory{Candles: candles})
	if s["last_close"].(float64) != 159 {
		t.Fatalf("expected last_close 159, got %v", s["last_close"])
	}
	if s["pct_from_52w_high"].(float64) != 0 {
		t.Fatalf("expected 0%% from high at the top, got %v", s["pct_from_52w_high"])
	}
	if s["return_1m_pct"].(float64) <= 0 {
		t.Fatalf("expected positive 1m return on a rising series, got %v", s["return_1m_pct"])
	}
}

// hold records a continuation decision (by symbol) and never touches the broker.
func TestDispatch_Hold(t *testing.T) {
	d := NewToolDispatcher(&fakeReader{liq: goodLiquidity()}, &fakeExec{}, DefaultCaps(), baseSnapshot())
	res := d.Dispatch(context.Background(), "hold", mustJSON(t, map[string]any{"symbol": "MDT", "rationale": "continuing to hold, thesis intact"}))
	if isErrResult(res) {
		t.Fatalf("hold should succeed, got: %s", res)
	}
	decs := d.Decisions()
	if len(decs) != 1 || decs[0].Action != ActionHold || decs[0].Symbol != "MDT" {
		t.Fatalf("expected one MDT hold decision, got %+v", decs)
	}
	// hold without a symbol is rejected (it's only for continuing a held name).
	if !isErrResult(d.Dispatch(context.Background(), "hold", mustJSON(t, map[string]any{"rationale": "x"}))) {
		t.Fatal("hold without symbol should error")
	}
}
