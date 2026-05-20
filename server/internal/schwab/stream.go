package schwab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

/*
Schwab streaming protocol notes (synthesized from schwab-py reference
implementation; Schwab's own docs are paywalled behind the developer
portal). All frames are JSON. Requests are wrapped in {"requests":[...]}
arrays; responses arrive as {"response":[...]}, {"data":[...]}, or
{"notify":[...]}.

LOGIN frame shape:
  {
    "requests": [{
      "service": "ADMIN",
      "requestid": "0",
      "command": "LOGIN",
      "SchwabClientCustomerId": "<id>",
      "SchwabClientCorrelId": "<id>",
      "parameters": {
        "Authorization": "<oauth_access_token>",
        "SchwabClientChannel": "<from streamerInfo>",
        "SchwabClientFunctionId": "<from streamerInfo>"
      }
    }]
  }

SUBS / UNSUBS / ADD frame shape:
  {
    "requests": [{
      "service": "LEVELONE_EQUITIES" | "LEVELONE_OPTIONS",
      "requestid": "<seq>",
      "command": "SUBS" | "ADD" | "UNSUBS",
      "SchwabClientCustomerId": "<id>",
      "SchwabClientCorrelId": "<id>",
      "parameters": {
        "keys":   "AAPL,MSFT" | "AAPL  240119C00150000",
        "fields": "0,1,2,3,..."
      }
    }]
  }

Note SUBS replaces the entire subscription set for that service; ADD
appends. We use SUBS once on initial connect and ADD/UNSUBS thereafter
when picks change.

Data frame shape:
  {
    "data": [{
      "service": "LEVELONE_EQUITIES",
      "timestamp": 1715908546054,
      "command": "SUBS",
      "content": [
        {"key": "AAPL", "1": 220.45, "2": 220.50, ...}
      ]
    }]
  }

Numeric keys in `content` map to field codes (see equityFields and
optionFields below). Schwab sends partial updates — only the changed
fields are present on each tick, so consumers must merge into a
per-key snapshot to get a complete state.

Heartbeats arrive as:
  {"notify": [{"heartbeat": "1715908546054"}]}

Server-initiated disconnects (token expiry, server restart, idle) come
as a response frame with code != 0; the read loop returns and the
caller reconnects.
*/

// ── Field codes ───────────────────────────────────────────────────

const (
	// LEVELONE_EQUITIES
	eqFieldBid          = "1"
	eqFieldAsk          = "2"
	eqFieldLast         = "3"
	eqFieldOpen         = "15" // OPEN_PRICE
	eqFieldTotalVolume  = "8"
	eqFieldNetChange    = "18"
	eqFieldNetChangePct = "42"
	// Subscribe to a fixed superset so partial updates fold cleanly into
	// the per-key snapshot.
	equityFieldsRequest = "0,1,2,3,8,15,18,42"

	// LEVELONE_OPTIONS
	opFieldBid          = "2"
	opFieldAsk          = "3"
	opFieldLast         = "4"
	opFieldTotalVolume  = "8"
	opFieldOpenInterest = "9"
	opFieldVolatility   = "10"
	opFieldDelta        = "28"
	opFieldTheta        = "30"
	opFieldMark         = "37"
	optionFieldsRequest = "0,2,3,4,8,9,10,28,30,37"
)

// ── Tick types (consumer-facing) ──────────────────────────────────

// EquityTick is the streamed Level 1 equity quote — same shape as the
// REST liveQuoteEntry the dashboard already consumes.
type EquityTick struct {
	Symbol       string
	LastPrice    float64
	OpenPrice    float64
	NetChange    float64
	NetChangePct float64
	BidPrice     float64
	AskPrice     float64
	Volume       int
}

// OptionTick is the streamed Level 1 option quote — same shape as the
// REST liveOptionEntry.
type OptionTick struct {
	OCCSymbol    string
	Bid          float64
	Ask          float64
	Last         float64
	Mark         float64
	Volume       int
	OpenInterest int
	Delta        float64
	Theta        float64
	ImpliedVol   float64
}

// StreamEvent is the per-tick event emitted by StreamClient.Events().
// Exactly one of Equity / Option will be non-nil.
type StreamEvent struct {
	Equity *EquityTick
	Option *OptionTick
}

// ── StreamClient ─────────────────────────────────────────────────

/*
StreamClient owns one Schwab streaming WebSocket connection. It handles
LOGIN, SUBS, reconnect with backoff, and re-LOGIN+re-SUBSCRIBE on
access-token rotation. Consumers call Subscribe / Unsubscribe to
manage the subscription set and read ticks from Events().

The client is intended to live for the duration of a market session
(9:30 ET to 16:00 ET) and is recreated by the Hub on each session.
Start/Stop are idempotent.
*/
type StreamClient struct {
	client *Client

	mu        sync.Mutex
	conn      *websocket.Conn
	info      *StreamerInfo
	running   bool
	reqSeq    int
	subEquity map[string]struct{}
	subOption map[string]struct{}
	stopCh    chan struct{}
	stoppedCh chan struct{}
	loggedIn  bool
	eventsCh  chan StreamEvent
	resyncCh  chan struct{} // signal that we need to re-SUBSCRIBE after a reconnect

	// dialer overridable for tests
	dialer *websocket.Dialer
}

// NewStreamClient wires a stream client onto an existing OAuth-aware
// Schwab client. The Schwab client must already be authorized
// (IsConnected()==true) before Start is called.
func NewStreamClient(c *Client) *StreamClient {
	return &StreamClient{
		client:    c,
		subEquity: make(map[string]struct{}),
		subOption: make(map[string]struct{}),
		eventsCh:  make(chan StreamEvent, 1024),
		dialer:    websocket.DefaultDialer,
	}
}

// Events returns the channel of incoming ticks. Consumers should drain
// it promptly — a slow consumer will see drops as the buffer fills.
func (s *StreamClient) Events() <-chan StreamEvent { return s.eventsCh }

/*
Start connects, authenticates, and runs the read loop until Stop is
called or an unrecoverable error occurs. On a recoverable error
(disconnect, transient network failure) Start reconnects with
exponential backoff and restores the subscription set. Returns when
the read loop terminates after Stop is called.

Start is idempotent: calling it while already running is a no-op.
*/
func (s *StreamClient) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.stoppedCh = make(chan struct{})
	s.resyncCh = make(chan struct{}, 1)
	s.mu.Unlock()

	go s.runLoop(ctx)
	return nil
}

/*
Stop closes the WebSocket and ends the read loop. Blocks until the
loop has exited. Idempotent — calling Stop on a not-started or
already-stopped client returns immediately.
*/
func (s *StreamClient) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	close(s.stopCh)
	conn := s.conn
	s.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	<-s.stoppedCh
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

// ── Subscription management ──────────────────────────────────────

/*
Subscribe adds tickers + OCC option symbols to the active set and
sends an ADD frame on the live connection. Safe to call before Start
(the keys will be SUBS'd on first connect). Safe to call concurrently.
*/
func (s *StreamClient) Subscribe(equities []string, options []string) error {
	s.mu.Lock()
	newEq := make([]string, 0, len(equities))
	for _, k := range equities {
		k = strings.ToUpper(strings.TrimSpace(k))
		if k == "" {
			continue
		}
		if _, ok := s.subEquity[k]; !ok {
			s.subEquity[k] = struct{}{}
			newEq = append(newEq, k)
		}
	}
	newOpt := make([]string, 0, len(options))
	for _, k := range options {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := s.subOption[k]; !ok {
			s.subOption[k] = struct{}{}
			newOpt = append(newOpt, k)
		}
	}
	conn := s.conn
	loggedIn := s.loggedIn
	info := s.info
	s.mu.Unlock()

	if conn == nil || !loggedIn || info == nil {
		return nil // queued for next connect
	}

	if len(newEq) > 0 {
		if err := s.send(conn, info, "LEVELONE_EQUITIES", "ADD", newEq, equityFieldsRequest); err != nil {
			return fmt.Errorf("subscribe equities: %w", err)
		}
	}
	if len(newOpt) > 0 {
		if err := s.send(conn, info, "LEVELONE_OPTIONS", "ADD", newOpt, optionFieldsRequest); err != nil {
			return fmt.Errorf("subscribe options: %w", err)
		}
	}
	return nil
}

/*
Unsubscribe removes tickers / OCC option symbols from the active set
and sends an UNSUBS frame on the live connection.
*/
func (s *StreamClient) Unsubscribe(equities []string, options []string) error {
	s.mu.Lock()
	dropEq := make([]string, 0, len(equities))
	for _, k := range equities {
		k = strings.ToUpper(strings.TrimSpace(k))
		if _, ok := s.subEquity[k]; ok {
			delete(s.subEquity, k)
			dropEq = append(dropEq, k)
		}
	}
	dropOpt := make([]string, 0, len(options))
	for _, k := range options {
		k = strings.TrimSpace(k)
		if _, ok := s.subOption[k]; ok {
			delete(s.subOption, k)
			dropOpt = append(dropOpt, k)
		}
	}
	conn := s.conn
	loggedIn := s.loggedIn
	info := s.info
	s.mu.Unlock()

	if conn == nil || !loggedIn || info == nil {
		return nil
	}
	if len(dropEq) > 0 {
		if err := s.send(conn, info, "LEVELONE_EQUITIES", "UNSUBS", dropEq, ""); err != nil {
			return fmt.Errorf("unsubscribe equities: %w", err)
		}
	}
	if len(dropOpt) > 0 {
		if err := s.send(conn, info, "LEVELONE_OPTIONS", "UNSUBS", dropOpt, ""); err != nil {
			return fmt.Errorf("unsubscribe options: %w", err)
		}
	}
	return nil
}

// ── Internal: connect / loop / read ───────────────────────────────

func (s *StreamClient) runLoop(ctx context.Context) {
	defer close(s.stoppedCh)

	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		err := s.connectAndRead(ctx)
		if err == nil {
			// Clean shutdown.
			return
		}
		if errors.Is(err, context.Canceled) {
			return
		}

		log.Printf("schwab stream: read loop ended: %v; reconnecting in %v", err, backoff)
		select {
		case <-time.After(backoff):
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (s *StreamClient) connectAndRead(ctx context.Context) error {
	info, err := s.client.GetStreamerInfo()
	if err != nil {
		return fmt.Errorf("get streamer info: %w", err)
	}
	tok, err := s.client.ValidToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, _, err := s.dialer.DialContext(dialCtx, info.StreamerSocketURL, http.Header{})
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	s.mu.Lock()
	s.conn = conn
	s.info = info
	s.loggedIn = false
	s.mu.Unlock()
	defer func() {
		_ = conn.Close()
		s.mu.Lock()
		s.conn = nil
		s.loggedIn = false
		s.mu.Unlock()
	}()

	// LOGIN.
	if err := s.sendLogin(conn, info, tok); err != nil {
		return fmt.Errorf("send login: %w", err)
	}

	// Reset backoff on the next iteration is implicit — we only
	// successfully reach the read loop after a successful LOGIN ack,
	// which is verified in handleFrame below. To keep the contract
	// simple, we leave the backoff as is; a flaky server that LOGINs
	// then drops still backs off.

	for {
		select {
		case <-s.stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		if err := s.handleFrame(raw); err != nil {
			log.Printf("schwab stream: handle frame: %v", err)
		}
	}
}

func (s *StreamClient) sendLogin(conn *websocket.Conn, info *StreamerInfo, token string) error {
	frame := map[string]interface{}{
		"requests": []map[string]interface{}{{
			"service":                "ADMIN",
			"requestid":              s.nextSeq(),
			"command":                "LOGIN",
			"SchwabClientCustomerId": info.SchwabClientCustomerId,
			"SchwabClientCorrelId":   info.SchwabClientCorrelId,
			"parameters": map[string]interface{}{
				"Authorization":          token,
				"SchwabClientChannel":    info.SchwabClientChannel,
				"SchwabClientFunctionId": info.SchwabClientFunctionId,
			},
		}},
	}
	return conn.WriteJSON(frame)
}

func (s *StreamClient) send(conn *websocket.Conn, info *StreamerInfo, service, command string, keys []string, fields string) error {
	params := map[string]interface{}{
		"keys": strings.Join(keys, ","),
	}
	if fields != "" {
		params["fields"] = fields
	}
	frame := map[string]interface{}{
		"requests": []map[string]interface{}{{
			"service":                service,
			"requestid":              s.nextSeq(),
			"command":                command,
			"SchwabClientCustomerId": info.SchwabClientCustomerId,
			"SchwabClientCorrelId":   info.SchwabClientCorrelId,
			"parameters":             params,
		}},
	}
	return conn.WriteJSON(frame)
}

func (s *StreamClient) nextSeq() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqSeq++
	return strconv.Itoa(s.reqSeq)
}

// ── Frame parsing ────────────────────────────────────────────────

type incomingFrame struct {
	Response []responseEntry  `json:"response,omitempty"`
	Data     []dataEntry      `json:"data,omitempty"`
	Notify   []map[string]any `json:"notify,omitempty"`
}

type responseEntry struct {
	Service string          `json:"service"`
	Command string          `json:"command"`
	Content responseContent `json:"content"`
}

type responseContent struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type dataEntry struct {
	Service string                   `json:"service"`
	Command string                   `json:"command"`
	Content []map[string]interface{} `json:"content"`
}

func (s *StreamClient) handleFrame(raw []byte) error {
	var f incomingFrame
	if err := json.Unmarshal(raw, &f); err != nil {
		return fmt.Errorf("unmarshal: %w (raw=%s)", err, truncate(raw, 200))
	}

	for _, r := range f.Response {
		if r.Service == "ADMIN" && r.Command == "LOGIN" {
			if r.Content.Code != 0 {
				return fmt.Errorf("LOGIN rejected: code=%d msg=%q", r.Content.Code, r.Content.Msg)
			}
			s.onLoginSuccess()
		}
	}

	for _, d := range f.Data {
		switch d.Service {
		case "LEVELONE_EQUITIES":
			for _, item := range d.Content {
				ev := parseEquityTick(item)
				if ev != nil {
					s.emit(StreamEvent{Equity: ev})
				}
			}
		case "LEVELONE_OPTIONS":
			for _, item := range d.Content {
				ev := parseOptionTick(item)
				if ev != nil {
					s.emit(StreamEvent{Option: ev})
				}
			}
		}
	}

	// Notify frames (heartbeats, etc) ignored — no action needed.
	_ = f.Notify
	return nil
}

func (s *StreamClient) onLoginSuccess() {
	s.mu.Lock()
	s.loggedIn = true
	conn := s.conn
	info := s.info
	eq := keys(s.subEquity)
	op := keys(s.subOption)
	s.mu.Unlock()

	if conn == nil || info == nil {
		return
	}
	if len(eq) > 0 {
		if err := s.send(conn, info, "LEVELONE_EQUITIES", "SUBS", eq, equityFieldsRequest); err != nil {
			log.Printf("schwab stream: SUBS equities after login: %v", err)
		}
	}
	if len(op) > 0 {
		if err := s.send(conn, info, "LEVELONE_OPTIONS", "SUBS", op, optionFieldsRequest); err != nil {
			log.Printf("schwab stream: SUBS options after login: %v", err)
		}
	}
}

func (s *StreamClient) emit(ev StreamEvent) {
	select {
	case s.eventsCh <- ev:
	default:
		// Slow consumer — drop. The Hub re-broadcasts the snapshot to
		// new SSE clients, so a dropped tick just means N+1 vs N
		// updates on a fast-moving symbol. Logging here would flood
		// during heavy ticks; sample-log if needed.
	}
}

// ── Tick parsers ─────────────────────────────────────────────────

func parseEquityTick(c map[string]interface{}) *EquityTick {
	keyRaw, ok := c["key"]
	if !ok {
		return nil
	}
	key, _ := keyRaw.(string)
	if key == "" {
		return nil
	}
	return &EquityTick{
		Symbol:       key,
		LastPrice:    asFloat(c[eqFieldLast]),
		OpenPrice:    asFloat(c[eqFieldOpen]),
		NetChange:    asFloat(c[eqFieldNetChange]),
		NetChangePct: asFloat(c[eqFieldNetChangePct]),
		BidPrice:     asFloat(c[eqFieldBid]),
		AskPrice:     asFloat(c[eqFieldAsk]),
		Volume:       int(asFloat(c[eqFieldTotalVolume])),
	}
}

func parseOptionTick(c map[string]interface{}) *OptionTick {
	keyRaw, ok := c["key"]
	if !ok {
		return nil
	}
	key, _ := keyRaw.(string)
	if key == "" {
		return nil
	}
	return &OptionTick{
		OCCSymbol:    key,
		Bid:          asFloat(c[opFieldBid]),
		Ask:          asFloat(c[opFieldAsk]),
		Last:         asFloat(c[opFieldLast]),
		Mark:         asFloat(c[opFieldMark]),
		Volume:       int(asFloat(c[opFieldTotalVolume])),
		OpenInterest: int(asFloat(c[opFieldOpenInterest])),
		Delta:        asFloat(c[opFieldDelta]),
		Theta:        asFloat(c[opFieldTheta]),
		ImpliedVol:   asFloat(c[opFieldVolatility]),
	}
}

func asFloat(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

// ── OCC option symbol formatter ─────────────────────────────────

/*
FormatOCCSymbol returns the OSI-compatible OCC option symbol Schwab's
streamer expects in the SUBS `keys` field. Format (21 chars):

	[SYMBOL_left_aligned_padded_to_6][YY][MM][DD][C|P][strike_x_1000_padded_to_8]

Example: AAPL Jan 19 2024 $150 Call -> "AAPL  240119C00150000"

expiration must be "YYYY-MM-DD". contractType must be "CALL" or "PUT".
*/
func FormatOCCSymbol(symbol, contractType, expiration string, strike float64) (string, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" || len(sym) > 6 {
		return "", fmt.Errorf("invalid symbol %q (must be 1-6 chars)", symbol)
	}
	paddedSym := sym + strings.Repeat(" ", 6-len(sym))

	t, err := time.Parse("2006-01-02", expiration)
	if err != nil {
		return "", fmt.Errorf("invalid expiration %q: %w", expiration, err)
	}
	dt := t.Format("060102")

	cp := strings.ToUpper(strings.TrimSpace(contractType))
	var cpCh string
	switch cp {
	case "CALL":
		cpCh = "C"
	case "PUT":
		cpCh = "P"
	default:
		return "", fmt.Errorf("invalid contract type %q (must be CALL or PUT)", contractType)
	}

	strikeInt := int(strike * 1000)
	if strikeInt <= 0 {
		return "", fmt.Errorf("invalid strike %v", strike)
	}
	strikeStr := fmt.Sprintf("%08d", strikeInt)

	return paddedSym + dt + cpCh + strikeStr, nil
}
