package quotes

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"vibetradez.com/internal/schwab"
)

/*
Hub fans the Schwab streaming WebSocket out to any number of SSE
subscribers, so every open dashboard re-prices the book tick by tick
instead of waiting on the 60-second poll.

Lifecycle: the underlying stream starts when the FIRST subscriber attaches
and stops a grace period after the LAST one leaves, so the Schwab
connection only exists while someone is actually watching. The
subscription set is the held book's symbols (underlying tickers plus OCC
option symbols) plus SPY for the benchmark readout, resynced periodically
so a newly opened position starts ticking without a reconnect.

Slow subscribers are dropped-from, never blocked-on: a tick that cannot be
buffered for a subscriber is discarded for that subscriber only.
*/

// TickSource abstracts schwab.StreamClient so tests can inject a fake
// stream and drive the fan-out deterministically.
type TickSource interface {
	Start(ctx context.Context) error
	Stop()
	Subscribe(equities []string, options []string) error
	Events() <-chan schwab.StreamEvent
}

// SymbolSet is the current set of symbols worth streaming: the held
// book's underlyings and OCC option symbols. Implemented by the caller
// (wired to the live broker positions).
type SymbolSet func(ctx context.Context) (equities []string, options []string)

const (
	// subscriberBuf is each SSE subscriber's tick buffer; a full buffer
	// drops ticks for that subscriber rather than stalling the hub.
	subscriberBuf = 256
	// idleGrace keeps the upstream stream alive briefly after the last
	// subscriber leaves, so a page refresh doesn't bounce the Schwab
	// connection.
	idleGrace = 60 * time.Second
	// resyncEvery re-reads the symbol set so newly opened positions get
	// subscribed mid-session.
	resyncEvery = 60 * time.Second
)

type Hub struct {
	newSource func() TickSource
	symbols   SymbolSet

	mu          sync.Mutex
	subscribers map[chan []byte]struct{}
	source      TickSource
	cancel      context.CancelFunc
	idleTimer   *time.Timer
}

// New builds a hub. newSource is called each time the stream needs to
// (re)start, so a stopped session gets a fresh client.
func New(newSource func() TickSource, symbols SymbolSet) *Hub {
	return &Hub{
		newSource:   newSource,
		symbols:     symbols,
		subscribers: make(map[chan []byte]struct{}),
	}
}

// tickPayload is one SSE event: the per-unit price for a symbol. Mark is
// the option mark or the equity last, whichever the instrument trades on.
type tickPayload struct {
	Type   string  `json:"type"` // "equity" | "option"
	Symbol string  `json:"symbol"`
	Mark   float64 `json:"mark"`
	Bid    float64 `json:"bid,omitempty"`
	Ask    float64 `json:"ask,omitempty"`
}

/*
Subscribe attaches a new SSE consumer and returns its event channel plus
a detach func. The first subscriber starts the upstream stream.
*/
func (h *Hub) Subscribe(ctx context.Context) (<-chan []byte, func()) {
	ch := make(chan []byte, subscriberBuf)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	if h.idleTimer != nil {
		h.idleTimer.Stop()
		h.idleTimer = nil
	}
	if h.source == nil {
		h.startLocked()
	}
	h.mu.Unlock()

	var once sync.Once
	detach := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subscribers, ch)
			if len(h.subscribers) == 0 && h.source != nil {
				// Last watcher left: stop after the grace period unless
				// someone comes back.
				h.idleTimer = time.AfterFunc(idleGrace, h.stopIfIdle)
			}
			h.mu.Unlock()
		})
	}
	// Detach automatically when the request context ends.
	go func() {
		<-ctx.Done()
		detach()
	}()
	return ch, detach
}

func (h *Hub) startLocked() {
	src := h.newSource()
	if src == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.source = src
	h.cancel = cancel
	if err := src.Start(ctx); err != nil {
		log.Printf("quotes hub: stream start: %v", err)
		h.source = nil
		h.cancel = nil
		cancel()
		return
	}
	eq, opt := h.symbols(ctx)
	if err := src.Subscribe(append(eq, "SPY"), opt); err != nil {
		log.Printf("quotes hub: subscribe: %v", err)
	}
	go h.fanOut(ctx, src)
	go h.resyncLoop(ctx, src)
}

func (h *Hub) stopIfIdle() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.subscribers) > 0 || h.source == nil {
		return
	}
	h.cancel()
	h.source.Stop()
	h.source = nil
	h.cancel = nil
	h.idleTimer = nil
}

func (h *Hub) fanOut(ctx context.Context, src TickSource) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-src.Events():
			if !ok {
				return
			}
			payload, valid := marshalTick(ev)
			if !valid {
				continue
			}
			h.mu.Lock()
			for ch := range h.subscribers {
				select {
				case ch <- payload:
				default: // slow subscriber: drop this tick for them
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) resyncLoop(ctx context.Context, src TickSource) {
	t := time.NewTicker(resyncEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			eq, opt := h.symbols(ctx)
			if err := src.Subscribe(append(eq, "SPY"), opt); err != nil {
				log.Printf("quotes hub: resync subscribe: %v", err)
			}
		}
	}
}

// marshalTick converts a stream event into the SSE payload, skipping
// events that carry no usable price.
func marshalTick(ev schwab.StreamEvent) ([]byte, bool) {
	var p tickPayload
	switch {
	case ev.Equity != nil:
		price := ev.Equity.LastPrice
		if price <= 0 {
			price = mid(ev.Equity.BidPrice, ev.Equity.AskPrice)
		}
		if price <= 0 {
			return nil, false
		}
		p = tickPayload{Type: "equity", Symbol: ev.Equity.Symbol, Mark: price, Bid: ev.Equity.BidPrice, Ask: ev.Equity.AskPrice}
	case ev.Option != nil:
		price := ev.Option.Mark
		if price <= 0 {
			price = ev.Option.Last
		}
		if price <= 0 {
			price = mid(ev.Option.Bid, ev.Option.Ask)
		}
		if price <= 0 {
			return nil, false
		}
		p = tickPayload{Type: "option", Symbol: ev.Option.OCCSymbol, Mark: price, Bid: ev.Option.Bid, Ask: ev.Option.Ask}
	default:
		return nil, false
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, false
	}
	return out, true
}

func mid(bid, ask float64) float64 {
	if bid > 0 && ask > 0 {
		return (bid + ask) / 2
	}
	return 0
}
