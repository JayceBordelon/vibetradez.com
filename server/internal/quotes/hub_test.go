package quotes

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"vibetradez.com/internal/schwab"
)

// fakeSource is an injectable tick source: tests push events and observe
// lifecycle calls.
type fakeSource struct {
	mu         sync.Mutex
	events     chan schwab.StreamEvent
	started    bool
	stopped    bool
	subEquity  []string
	subOptions []string
	subCalls   int
}

func newFakeSource() *fakeSource {
	return &fakeSource{events: make(chan schwab.StreamEvent, 64)}
}

func (f *fakeSource) Start(context.Context) error {
	f.mu.Lock()
	f.started = true
	f.mu.Unlock()
	return nil
}
func (f *fakeSource) Stop()                             { f.mu.Lock(); f.stopped = true; f.mu.Unlock() }
func (f *fakeSource) Events() <-chan schwab.StreamEvent { return f.events }
func (f *fakeSource) Subscribe(eq, opt []string) error {
	f.mu.Lock()
	f.subEquity, f.subOptions = eq, opt
	f.subCalls++
	f.mu.Unlock()
	return nil
}

func testSymbols(context.Context) ([]string, []string) {
	return []string{"NVDA", "GOOGL"}, []string{"NVDA  260918C00215000"}
}

func recv(t *testing.T, ch <-chan []byte) tickPayload {
	t.Helper()
	select {
	case raw := <-ch:
		var p tickPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("bad payload %s: %v", raw, err)
		}
		return p
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tick")
		return tickPayload{}
	}
}

func TestHub_StartsOnFirstSubscriberAndFansOut(t *testing.T) {
	src := newFakeSource()
	h := New(func() TickSource { return src }, testSymbols)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch1, detach1 := h.Subscribe(ctx)
	defer detach1()
	ch2, detach2 := h.Subscribe(ctx)
	defer detach2()

	src.mu.Lock()
	started, eq, opt := src.started, src.subEquity, src.subOptions
	src.mu.Unlock()
	if !started {
		t.Fatal("first subscriber must start the stream")
	}
	if len(eq) != 3 || eq[2] != "SPY" {
		t.Fatalf("expected held equities + SPY, got %v", eq)
	}
	if len(opt) != 1 {
		t.Fatalf("expected the held OCC option subscribed, got %v", opt)
	}

	// One equity tick reaches BOTH subscribers with the last price as mark.
	src.events <- schwab.StreamEvent{Equity: &schwab.EquityTick{Symbol: "NVDA", LastPrice: 210.55, BidPrice: 210.5, AskPrice: 210.6}}
	for _, ch := range []<-chan []byte{ch1, ch2} {
		p := recv(t, ch)
		if p.Type != "equity" || p.Symbol != "NVDA" || p.Mark != 210.55 {
			t.Fatalf("unexpected payload %+v", p)
		}
	}

	// An option tick prefers Mark, falls back to Last.
	src.events <- schwab.StreamEvent{Option: &schwab.OptionTick{OCCSymbol: "NVDA  260918C00215000", Mark: 18.4}}
	if p := recv(t, ch1); p.Type != "option" || p.Mark != 18.4 {
		t.Fatalf("unexpected option payload %+v", p)
	}

	// A tick with no usable price is swallowed, not fanned out.
	src.events <- schwab.StreamEvent{Equity: &schwab.EquityTick{Symbol: "GOOGL"}}
	src.events <- schwab.StreamEvent{Equity: &schwab.EquityTick{Symbol: "GOOGL", LastPrice: 371.2}}
	if p := recv(t, ch1); p.Symbol != "GOOGL" || p.Mark != 371.2 {
		t.Fatalf("zero-price tick must be skipped; got %+v", p)
	}
}

func TestHub_DetachIsIdempotentAndKeepsOthersStreaming(t *testing.T) {
	src := newFakeSource()
	h := New(func() TickSource { return src }, testSymbols)
	ctx := context.Background()

	ch1, detach1 := h.Subscribe(ctx)
	ch2, detach2 := h.Subscribe(ctx)
	defer detach2()

	detach1()
	detach1() // double-detach must be safe

	src.events <- schwab.StreamEvent{Equity: &schwab.EquityTick{Symbol: "NVDA", LastPrice: 211}}
	if p := recv(t, ch2); p.Mark != 211 {
		t.Fatalf("remaining subscriber must keep receiving, got %+v", p)
	}
	select {
	case raw := <-ch1:
		// A buffered pre-detach tick is fine; a post-detach one is not.
		var p tickPayload
		_ = json.Unmarshal(raw, &p)
		if p.Mark == 211 {
			t.Fatal("detached subscriber must not receive new ticks")
		}
	default:
	}

	src.mu.Lock()
	stopped := src.stopped
	src.mu.Unlock()
	if stopped {
		t.Fatal("stream must keep running while a subscriber remains")
	}
}
