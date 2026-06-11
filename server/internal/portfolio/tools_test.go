package portfolio

import (
	"context"
	"encoding/json"
	"strings"
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
}

func (f *fakeReader) Snapshot(context.Context) (Snapshot, error) { return Snapshot{}, nil }
func (f *fakeReader) GetQuotes([]string) (map[string]schwab.StockQuote, error) {
	return map[string]schwab.StockQuote{}, nil
}
func (f *fakeReader) GetOptionChain(string, string, string, string, float64) (*schwab.OptionChain, error) {
	return &schwab.OptionChain{}, nil
}
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
func (f *fakeReader) TrackRecord(int) (TrackRecord, error)        { return TrackRecord{}, nil }

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
	reader := &fakeReader{}
	ex := &fakeExec{}
	snap := baseSnapshot() // $6k equity, equity sleeve $3,000
	d := NewToolDispatcher(reader, ex, DefaultCaps(), snap)

	// $3,100 stock order > the $3,000 equity sleeve.
	res := d.Dispatch(context.Background(), "buy_equity", mustJSON(t, map[string]any{
		"symbol": "MDT", "quantity": 62.0, "limit_price": 50.0, "rationale": "test",
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

// Successful buys update the working snapshot, so cumulative stock buys
// are gated against the live equity sleeve, not the pristine session
// state. With pacing gone, the sleeve itself is the only cumulative gate.
func TestDispatch_CumulativeBuysRunToTheSleeve(t *testing.T) {
	reader := &fakeReader{}
	ex := &fakeExec{}
	snap := baseSnapshot() // $6,000 equity: equity sleeve $3,000
	d := NewToolDispatcher(reader, ex, DefaultCaps(), snap)

	// $1,500 + $1,400 + $100 lands exactly ON the $3,000 sleeve: all pass
	// (the old per-order/concentration/pacing caps would have refused long
	// before this point).
	for i, buy := range []map[string]any{
		{"symbol": "AAA", "quantity": 100.0, "limit_price": 15.0, "rationale": "first"},
		{"symbol": "BBB", "quantity": 100.0, "limit_price": 14.0, "rationale": "second"},
		{"symbol": "CCC", "quantity": 10.0, "limit_price": 10.0, "rationale": "third"},
	} {
		if r := d.Dispatch(context.Background(), "buy_equity", mustJSON(t, buy)); isErrResult(r) {
			t.Fatalf("buy %d should pass, got: %s", i+1, r)
		}
	}
	// One more dollar of stock must be refused on the equity sleeve.
	r := d.Dispatch(context.Background(), "buy_equity", mustJSON(t, map[string]any{
		"symbol": "DDD", "quantity": 1.0, "limit_price": 10.0, "rationale": "fourth",
	}))
	if !isErrResult(r) || !strings.Contains(r, "equity-sleeve") {
		t.Fatalf("fourth buy should be refused on the equity sleeve, got: %s", r)
	}
	if ex.buyEquityCalls != 3 {
		t.Fatalf("exactly three buys should have reached the broker, got %d", ex.buyEquityCalls)
	}
}

func TestDispatch_CapHeadroom(t *testing.T) {
	snap := baseSnapshot() // $6k equity
	snap.Positions = []Position{{Symbol: "MDT", Underlying: "MDT", AssetType: AssetEquity, MarkValue: 1500, Quantity: 30}}
	d := NewToolDispatcher(&fakeReader{}, &fakeExec{}, DefaultCaps(), snap)

	res := d.Dispatch(context.Background(), "get_cap_headroom", mustJSON(t, map[string]any{}))
	var out map[string]any
	if err := json.Unmarshal([]byte(res), &out); err != nil {
		t.Fatalf("bad json: %s", res)
	}
	// Equity sleeve = 50% of $6,000 = $3,000; $1,500 of stock held leaves $1,500.
	if out["equity_sleeve_remaining"].(float64) != 1500 {
		t.Fatalf("expected equity_sleeve_remaining 1500, got %v", out["equity_sleeve_remaining"])
	}
	// Options sleeve untouched: full $3,000 remaining.
	if out["options_sleeve_remaining"].(float64) != 3000 {
		t.Fatalf("expected options_sleeve_remaining 3000, got %v", out["options_sleeve_remaining"])
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
	d := NewToolDispatcher(&fakeReader{}, &fakeExec{}, DefaultCaps(), baseSnapshot())
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
