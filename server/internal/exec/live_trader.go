package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"vibetradez.com/internal/schwab"
)

// traderBase is the prefix for all Schwab Trader API endpoints.
const traderBase = "https://api.schwabapi.com/trader/v1"

/*
LiveTrader implements exec.TraderClient against the real Schwab Trader
API. Construction does NOT make any network calls; the first call to
AccountHash discovers and caches the account hash. All write
operations (PlaceOrder / CancelOrder) require the user to have
authorized the "Accounts and Trading" Schwab product, which may
require re-running the OAuth flow if the original consent only
covered Market Data.
*/
type LiveTrader struct {
	c *schwab.Client

	mu          sync.Mutex
	cachedHash  string
	cacheExpiry time.Time
}

func NewLiveTrader(c *schwab.Client) *LiveTrader {
	return &LiveTrader{c: c}
}

func (lt *LiveTrader) AccountHash(ctx context.Context) (string, error) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if lt.cachedHash != "" && time.Now().Before(lt.cacheExpiry) {
		return lt.cachedHash, nil
	}

	resp, err := lt.c.AuthenticatedDo("GET", traderBase+"/accounts/accountNumbers", nil)
	if err != nil {
		return "", fmt.Errorf("account numbers: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("account numbers HTTP %d: %s", resp.StatusCode, string(body))
	}
	var rows []struct {
		AccountNumber string `json:"accountNumber"`
		HashValue     string `json:"hashValue"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return "", fmt.Errorf("decode account numbers: %w", err)
	}
	if len(rows) == 0 {
		return "", errors.New("schwab returned no accounts")
	}
	// Cache for 24h — account hashes are stable per Schwab account.
	lt.cachedHash = rows[0].HashValue
	lt.cacheExpiry = time.Now().Add(24 * time.Hour)
	return lt.cachedHash, nil
}

func (lt *LiveTrader) PlaceOrder(ctx context.Context, accountHash string, order Order) (string, error) {
	body, err := json.Marshal(order)
	if err != nil {
		return "", fmt.Errorf("marshal order: %w", err)
	}
	url := fmt.Sprintf("%s/accounts/%s/orders", traderBase, accountHash)
	resp, err := lt.c.AuthenticatedDo("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("place order: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)

	/*
		Schwab returns 201 Created with the new order id in the Location
		header (last path segment). 200 also surfaces as success on some
		account types. Anything else is a broker-side rejection.
	*/
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("place order HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", errors.New("schwab returned no Location header on order create")
	}
	parts := strings.Split(loc, "/")
	id := parts[len(parts)-1]
	if id == "" {
		return "", fmt.Errorf("schwab Location header malformed: %q", loc)
	}
	return id, nil
}

func (lt *LiveTrader) GetOrder(ctx context.Context, accountHash, orderID string) (OrderStatus, error) {
	url := fmt.Sprintf("%s/accounts/%s/orders/%s", traderBase, accountHash, orderID)
	resp, err := lt.c.AuthenticatedDo("GET", url, nil)
	if err != nil {
		return OrderStatus{}, fmt.Errorf("get order: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return OrderStatus{}, fmt.Errorf("get order HTTP %d: %s", resp.StatusCode, string(body))
	}
	var raw struct {
		OrderID           int64   `json:"orderId"`
		Status            string  `json:"status"`
		Quantity          float64 `json:"quantity"`
		FilledQuantity    float64 `json:"filledQuantity"`
		ClosingPrice      float64 `json:"closingPrice"`
		Price             float64 `json:"price"`
		StatusDescription string  `json:"statusDescription"`
		EnteredTime       string  `json:"enteredTime"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return OrderStatus{}, fmt.Errorf("decode order status: %w", err)
	}

	st := OrderStatus{
		OrderID:        orderID,
		RawStatus:      raw.Status,
		Quantity:       int(raw.Quantity),
		FilledQuantity: int(raw.FilledQuantity),
		FillPrice:      raw.Price,
		UpdatedAt:      time.Now(),
		ErrorMessage:   raw.StatusDescription,
	}
	switch raw.Status {
	case "FILLED":
		st.Filled = true
		st.Terminal = true
	case "CANCELED", "EXPIRED", "REJECTED", "REPLACED":
		st.Terminal = true
	case "WORKING", "QUEUED", "ACCEPTED", "PENDING_ACTIVATION", "AWAITING_PARENT_ORDER",
		"AWAITING_CONDITION", "AWAITING_STOP_CONDITION", "AWAITING_MANUAL_REVIEW",
		"AWAITING_UR_OUT", "PENDING_CANCEL", "PENDING_REPLACE":
		st.Working = true
	}
	return st, nil
}

func (lt *LiveTrader) CancelOrder(ctx context.Context, accountHash, orderID string) error {
	url := fmt.Sprintf("%s/accounts/%s/orders/%s", traderBase, accountHash, orderID)
	resp, err := lt.c.AuthenticatedDo("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cancel order HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
