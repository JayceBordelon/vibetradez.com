// Package quotes runs the live-quotes streaming hub: a single Schwab
// WebSocket connection multiplexed to N browser SSE clients. The
// dashboard polls /api/market/status on mount; when open, it opens
// /api/quotes/stream and receives a snapshot + per-tick updates as
// individual contracts and their underlying stocks change.
//
// Lifecycle is cron-driven from cmd/scanner/main.go: Start at 9:30 ET
// sharp (today's picks are already in DB by then since the 9:25 picker
// has finished), Stop at 16:00 ET (or 13:00 on half-days). When the
// hub isn't running, the SSE endpoint returns 503 and the client
// renders the closed-page.
package quotes

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/trades"
)

/*
TradeFetcher is the slice of the store the hub depends on: today's
picks, by date. The Hub reads this once at Start to compute its
initial subscription set (one equity ticker + one option contract
per pick).
*/
type TradeFetcher interface {
	GetMorningTrades(date string) ([]trades.Trade, error)
	GetLatestTradeDate() (string, error)
}

// EquityQuote / OptionQuote mirror the wire types the dashboard
// already consumes. See client/types/trade.ts.
type EquityQuote struct {
	LastPrice    float64 `json:"last_price"`
	OpenPrice    float64 `json:"open_price"`
	NetChange    float64 `json:"net_change"`
	NetChangePct float64 `json:"net_change_pct"`
	BidPrice     float64 `json:"bid_price"`
	AskPrice     float64 `json:"ask_price"`
	Volume       int     `json:"volume"`
}

type OptionQuote struct {
	Bid          float64 `json:"bid"`
	Ask          float64 `json:"ask"`
	Last         float64 `json:"last"`
	Mark         float64 `json:"mark"`
	Volume       int     `json:"volume"`
	OpenInterest int     `json:"open_interest"`
	Delta        float64 `json:"delta"`
	Theta        float64 `json:"theta"`
	ImpliedVol   float64 `json:"implied_vol"`
}

/*
Snapshot is the SSE `snapshot` event payload sent on each new client
attach. The maps mirror /api/quotes/live's pre-streaming shape:

  - Quotes is keyed by symbol (e.g. "AAPL")
  - Options is keyed by the internal compound key
    "<SYMBOL>|<CALL|PUT>|<strike .2f>|<expiration>" so the existing
    UI lookups (execution-badge.tsx, morning-layout.tsx) don't change.

Connected reflects Schwab WS status; market_open reflects the
calendar. Both are sent on every snapshot so the client can render
honest state during a transient disconnect.
*/
type Snapshot struct {
	Connected  bool                   `json:"connected"`
	MarketOpen bool                   `json:"market_open"`
	AsOf       string                 `json:"as_of"`
	Quotes     map[string]EquityQuote `json:"quotes"`
	Options    map[string]OptionQuote `json:"options"`
}

/*
QuoteUpdate is the SSE `quote` event payload — sent once per tick.
Exactly one of Quote / Option is populated. The OptionKey when
present is the same compound key used in Snapshot.Options.
*/
type QuoteUpdate struct {
	Symbol    string       `json:"symbol,omitempty"`
	OptionKey string       `json:"option_key,omitempty"`
	Quote     *EquityQuote `json:"quote,omitempty"`
	Option    *OptionQuote `json:"option,omitempty"`
	AsOf      string       `json:"as_of"`
}

// ── Hub ──────────────────────────────────────────────────────────

/*
Hub fans out one Schwab StreamClient's ticks to many SSE clients.
Owns the StreamClient lifecycle, the latest-tick cache (for snapshot
on new attach), and the SSE client registry.

A single Hub instance lives for the lifetime of the trading-server
process. Start/Stop are called from the cron schedule and are
idempotent.
*/
type Hub struct {
	stream   *schwab.StreamClient
	store    TradeFetcher
	occToKey map[string]string // OCC symbol -> internal compound key

	mu        sync.RWMutex
	running   bool
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	quotes    map[string]EquityQuote
	options   map[string]OptionQuote
	asOf      time.Time
	connected bool
	clients   map[*Client]struct{}
}

/*
NewHub builds a hub backed by a (possibly nil) Schwab stream client.
A nil stream client puts the hub in "no live data" mode — the SSE
endpoint will still serve a snapshot with Connected=false so the
client can render gracefully. This matches the existing fallback for
schwab not connected on the REST path.
*/
func NewHub(stream *schwab.StreamClient, store TradeFetcher) *Hub {
	return &Hub{
		stream:   stream,
		store:    store,
		occToKey: make(map[string]string),
		quotes:   make(map[string]EquityQuote),
		options:  make(map[string]OptionQuote),
		clients:  make(map[*Client]struct{}),
	}
}

/*
Start brings the hub up: reads today's picks, computes the
subscription set, starts the Schwab stream client, and begins
fan-out. Idempotent.

If picks can't be read (no trading day in DB, or DB error), Start
still succeeds but the hub serves an empty snapshot — the dashboard
shows the closed-page or "no picks today" state regardless.
*/
func (h *Hub) Start(ctx context.Context) error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = true
	hubCtx, cancel := context.WithCancel(ctx)
	h.cancel = cancel
	h.mu.Unlock()

	picks, _ := h.loadPicks()
	equities, options, occToKey := h.computeSubscriptions(picks)

	h.mu.Lock()
	h.occToKey = occToKey
	h.mu.Unlock()

	if h.stream != nil {
		if err := h.stream.Start(hubCtx); err != nil {
			log.Printf("quotes hub: stream start: %v", err)
		} else {
			h.mu.Lock()
			h.connected = true
			h.mu.Unlock()
		}
		if err := h.stream.Subscribe(equities, options); err != nil {
			log.Printf("quotes hub: initial subscribe: %v", err)
		}
		h.wg.Add(1)
		go h.runReadLoop(hubCtx)
	}

	log.Printf("quotes hub: started with %d equity subscriptions, %d option subscriptions", len(equities), len(options))
	return nil
}

/*
Stop tears the hub down: closes the stream, drops all SSE clients
(which causes their handlers to return), and resets state. Idempotent.
*/
func (h *Hub) Stop() {
	h.mu.Lock()
	if !h.running {
		h.mu.Unlock()
		return
	}
	h.running = false
	cancel := h.cancel
	clients := h.clients
	h.clients = make(map[*Client]struct{})
	h.connected = false
	h.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if h.stream != nil {
		h.stream.Stop()
	}
	for c := range clients {
		c.close()
	}
	h.wg.Wait()
	log.Printf("quotes hub: stopped")
}

// Running reports whether the hub is currently active. Used by the
// SSE handler to decide between 200 (stream) and 503 (closed).
func (h *Hub) Running() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}

// SnapshotNow returns the current per-key cache for a fresh SSE attach.
func (h *Hub) SnapshotNow(marketOpen bool) Snapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	quotes := make(map[string]EquityQuote, len(h.quotes))
	for k, v := range h.quotes {
		quotes[k] = v
	}
	options := make(map[string]OptionQuote, len(h.options))
	for k, v := range h.options {
		options[k] = v
	}
	asOf := h.asOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	return Snapshot{
		Connected:  h.connected,
		MarketOpen: marketOpen,
		AsOf:       asOf.UTC().Format(time.RFC3339),
		Quotes:     quotes,
		Options:    options,
	}
}

// ── Client registration (SSE) ─────────────────────────────────────

/*
Client is one connected SSE consumer. Created by the SSE handler via
Register and closed on disconnect via Unregister. The handler reads
from Updates() and writes SSE-formatted frames to the response writer.
*/
type Client struct {
	updates chan QuoteUpdate
	closed  chan struct{}
	once    sync.Once
}

func newClient() *Client {
	return &Client{
		updates: make(chan QuoteUpdate, 256),
		closed:  make(chan struct{}),
	}
}

func (c *Client) Updates() <-chan QuoteUpdate { return c.updates }
func (c *Client) Done() <-chan struct{}       { return c.closed }

func (c *Client) close() {
	c.once.Do(func() {
		close(c.closed)
	})
}

// Register adds a new SSE client. The caller MUST call Unregister
// when the client disconnects.
func (h *Hub) Register() *Client {
	c := newClient()
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	return c
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	c.close()
}

// ── Internal: tick read loop ─────────────────────────────────────

func (h *Hub) runReadLoop(ctx context.Context) {
	defer h.wg.Done()
	if h.stream == nil {
		return
	}
	events := h.stream.Events()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			h.handleEvent(ev)
		}
	}
}

func (h *Hub) handleEvent(ev schwab.StreamEvent) {
	now := time.Now()
	switch {
	case ev.Equity != nil:
		eq := EquityQuote{
			LastPrice:    ev.Equity.LastPrice,
			OpenPrice:    ev.Equity.OpenPrice,
			NetChange:    ev.Equity.NetChange,
			NetChangePct: ev.Equity.NetChangePct,
			BidPrice:     ev.Equity.BidPrice,
			AskPrice:     ev.Equity.AskPrice,
			Volume:       ev.Equity.Volume,
		}
		h.mu.Lock()
		// Merge: a partial tick (e.g. only bid changed) must not zero
		// out the other fields. Schwab sends sparse updates, so we
		// fold each tick into the prior snapshot.
		prev := h.quotes[ev.Equity.Symbol]
		merged := mergeEquity(prev, eq)
		h.quotes[ev.Equity.Symbol] = merged
		h.asOf = now
		clients := h.clientList()
		h.mu.Unlock()

		update := QuoteUpdate{
			Symbol: ev.Equity.Symbol,
			Quote:  &merged,
			AsOf:   now.UTC().Format(time.RFC3339),
		}
		h.broadcast(clients, update)
	case ev.Option != nil:
		op := OptionQuote{
			Bid:          ev.Option.Bid,
			Ask:          ev.Option.Ask,
			Last:         ev.Option.Last,
			Mark:         ev.Option.Mark,
			Volume:       ev.Option.Volume,
			OpenInterest: ev.Option.OpenInterest,
			Delta:        ev.Option.Delta,
			Theta:        ev.Option.Theta,
			ImpliedVol:   ev.Option.ImpliedVol,
		}
		h.mu.Lock()
		key, ok := h.occToKey[ev.Option.OCCSymbol]
		if !ok {
			h.mu.Unlock()
			return
		}
		prev := h.options[key]
		merged := mergeOption(prev, op)
		h.options[key] = merged
		h.asOf = now
		clients := h.clientList()
		h.mu.Unlock()

		update := QuoteUpdate{
			OptionKey: key,
			Option:    &merged,
			AsOf:      now.UTC().Format(time.RFC3339),
		}
		h.broadcast(clients, update)
	}
}

func (h *Hub) clientList() []*Client {
	out := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		out = append(out, c)
	}
	return out
}

func (h *Hub) broadcast(clients []*Client, u QuoteUpdate) {
	for _, c := range clients {
		select {
		case c.updates <- u:
		default:
			// Slow client. Drop this tick rather than block the read
			// loop. The next tick on the same key supersedes anyway.
		}
	}
}

// ── Internal: subscription set computation ───────────────────────

func (h *Hub) loadPicks() ([]trades.Trade, error) {
	date, err := h.store.GetLatestTradeDate()
	if err != nil {
		return nil, fmt.Errorf("latest trade date: %w", err)
	}
	if date == "" {
		return nil, nil
	}
	return h.store.GetMorningTrades(date)
}

func (h *Hub) computeSubscriptions(picks []trades.Trade) (equities, options []string, occToKey map[string]string) {
	equitySet := make(map[string]struct{})
	occToKey = make(map[string]string)
	for _, t := range picks {
		equitySet[t.Symbol] = struct{}{}
		if !t.ContractResolved() {
			// Pre-resolution picks (executor hasn't run yet at the 9:30
			// ET hub start, or resolution failed) have no contract to
			// stream. Equity stream still subscribes; option stream
			// kicks in once the contract resolves and the hub re-syncs.
			continue
		}
		occ, err := schwab.FormatOCCSymbol(t.Symbol, t.ContractType, t.ExpirationStr(), t.Strike())
		if err != nil {
			log.Printf("quotes hub: skip pick %s (OCC build: %v)", t.Symbol, err)
			continue
		}
		// Internal key matches the existing /api/quotes/live shape so
		// the client lookup (key built from trade fields) finds it.
		internalKey := fmt.Sprintf("%s|%s|%.2f|%s", t.Symbol, t.ContractType, t.Strike(), t.ExpirationStr())
		occToKey[occ] = internalKey
		options = append(options, occ)
	}
	for sym := range equitySet {
		equities = append(equities, sym)
	}
	return equities, options, occToKey
}

// ── Tick merge helpers ───────────────────────────────────────────

func mergeEquity(prev, next EquityQuote) EquityQuote {
	if next.LastPrice == 0 {
		next.LastPrice = prev.LastPrice
	}
	if next.OpenPrice == 0 {
		next.OpenPrice = prev.OpenPrice
	}
	if next.BidPrice == 0 {
		next.BidPrice = prev.BidPrice
	}
	if next.AskPrice == 0 {
		next.AskPrice = prev.AskPrice
	}
	if next.NetChange == 0 {
		next.NetChange = prev.NetChange
	}
	if next.NetChangePct == 0 {
		next.NetChangePct = prev.NetChangePct
	}
	if next.Volume == 0 {
		next.Volume = prev.Volume
	}
	return next
}

func mergeOption(prev, next OptionQuote) OptionQuote {
	if next.Bid == 0 {
		next.Bid = prev.Bid
	}
	if next.Ask == 0 {
		next.Ask = prev.Ask
	}
	if next.Last == 0 {
		next.Last = prev.Last
	}
	if next.Mark == 0 {
		next.Mark = prev.Mark
	}
	if next.Volume == 0 {
		next.Volume = prev.Volume
	}
	if next.OpenInterest == 0 {
		next.OpenInterest = prev.OpenInterest
	}
	if next.Delta == 0 {
		next.Delta = prev.Delta
	}
	if next.Theta == 0 {
		next.Theta = prev.Theta
	}
	if next.ImpliedVol == 0 {
		next.ImpliedVol = prev.ImpliedVol
	}
	return next
}
