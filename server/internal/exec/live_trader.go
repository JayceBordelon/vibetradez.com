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

/*
AvailableFunds fetches the account balance from Schwab's Trader API
and returns the cash available for a new non-marginable position
(buying long options is non-marginable in a typical retail account).
The endpoint returns "currentBalances" with several flavors of buying
power; we prefer the most conservative one available so a margin-type
account doesn't accidentally let the basket eat margin headroom that
would otherwise cover working orders or other positions.

Preference order (first non-zero wins):
  - availableFundsNonMarginableTrade: cash that can fund a long option
    open without locking into margin. This is the right number for our
    cash-secured option workflow.
  - cashAvailableForTrading: surfaced on CASH-type accounts which don't
    expose the non-marginable-trade field at all.
  - availableFunds: the broadest "unrestricted cash" number — last resort.
*/
func (lt *LiveTrader) AvailableFunds(ctx context.Context, accountHash string) (float64, error) {
	url := fmt.Sprintf("%s/accounts/%s", traderBase, accountHash)
	resp, err := lt.c.AuthenticatedDo("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("get account balances: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("account balances HTTP %d: %s", resp.StatusCode, string(body))
	}
	var raw struct {
		SecuritiesAccount struct {
			CurrentBalances struct {
				AvailableFunds                   float64 `json:"availableFunds"`
				AvailableFundsNonMarginableTrade float64 `json:"availableFundsNonMarginableTrade"`
				CashAvailableForTrading          float64 `json:"cashAvailableForTrading"`
			} `json:"currentBalances"`
		} `json:"securitiesAccount"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, fmt.Errorf("decode account balances: %w", err)
	}
	cb := raw.SecuritiesAccount.CurrentBalances
	if cb.AvailableFundsNonMarginableTrade > 0 {
		return cb.AvailableFundsNonMarginableTrade, nil
	}
	if cb.CashAvailableForTrading > 0 {
		return cb.CashAvailableForTrading, nil
	}
	return cb.AvailableFunds, nil
}

/*
GetPositions fetches the account's open positions from Schwab's Trader
API (GET /accounts/{hash}?fields=positions) and normalizes them to
[]BrokerPosition. Net long quantity is longQuantity minus shortQuantity;
the v2 portfolio manager is long-only so short legs should be absent, but
the subtraction is defensive. Underlying keys to the equity ticker
(instrument.underlyingSymbol for options, symbol for equity) so
concentration accounting can group an equity and its options together.
*/
func (lt *LiveTrader) GetPositions(ctx context.Context, accountHash string) ([]BrokerPosition, error) {
	url := fmt.Sprintf("%s/accounts/%s?fields=positions", traderBase, accountHash)
	resp, err := lt.c.AuthenticatedDo("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("get positions: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get positions HTTP %d: %s", resp.StatusCode, string(body))
	}
	var raw struct {
		SecuritiesAccount struct {
			Positions []struct {
				ShortQuantity float64 `json:"shortQuantity"`
				LongQuantity  float64 `json:"longQuantity"`
				AveragePrice  float64 `json:"averagePrice"`
				MarketValue   float64 `json:"marketValue"`
				Instrument    struct {
					AssetType        string `json:"assetType"`
					Symbol           string `json:"symbol"`
					PutCall          string `json:"putCall"`
					UnderlyingSymbol string `json:"underlyingSymbol"`
				} `json:"instrument"`
			} `json:"positions"`
		} `json:"securitiesAccount"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode positions: %w", err)
	}
	out := make([]BrokerPosition, 0, len(raw.SecuritiesAccount.Positions))
	for _, p := range raw.SecuritiesAccount.Positions {
		underlying := p.Instrument.UnderlyingSymbol
		if underlying == "" {
			underlying = p.Instrument.Symbol
		}
		var ctype string
		switch p.Instrument.PutCall {
		case "CALL":
			ctype = "CALL"
		case "PUT":
			ctype = "PUT"
		}
		bp := BrokerPosition{
			Symbol:       p.Instrument.Symbol,
			AssetType:    p.Instrument.AssetType,
			Underlying:   underlying,
			Quantity:     p.LongQuantity - p.ShortQuantity,
			MarketValue:  p.MarketValue,
			AverageCost:  p.AveragePrice,
			ContractType: ctype,
		}
		enrichOptionContract(&bp)
		out = append(out, bp)
	}
	return out, nil
}

/*
enrichOptionContract fills an OPTION position's contract spec (strike,
expiration, and any missing put-call / underlying) by decoding its OCC
symbol. Schwab's positions payload carries no strike or expiration fields,
so without the decode the live book serves options with a zero strike and
no expiry, and the dashboard's holding detail renders "-" for strike,
expiration, and days-to-expiry. A malformed symbol degrades to passthrough.
*/
func enrichOptionContract(bp *BrokerPosition) {
	if bp.AssetType != "OPTION" {
		return
	}
	sym, exp, ctype, strike, err := DecodeOCCSymbol(bp.Symbol)
	if err != nil {
		return
	}
	bp.Strike = strike
	bp.Expiration = exp
	if bp.ContractType == "" {
		bp.ContractType = ctype
	}
	if bp.Underlying == "" || bp.Underlying == bp.Symbol {
		bp.Underlying = sym
	}
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
		OrderID                 int64           `json:"orderId"`
		Status                  string          `json:"status"`
		Quantity                float64         `json:"quantity"`
		FilledQuantity          float64         `json:"filledQuantity"`
		ClosingPrice            float64         `json:"closingPrice"`
		Price                   float64         `json:"price"`
		StatusDescription       string          `json:"statusDescription"`
		EnteredTime             string          `json:"enteredTime"`
		OrderActivityCollection []orderActivity `json:"orderActivityCollection"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return OrderStatus{}, fmt.Errorf("decode order status: %w", err)
	}

	/*
		raw.Price is the LIMIT price the order was placed at, not the
		fill price — for MARKET orders (the 3:55pm close path) it is
		always 0, so a previous version of this code logged closes at
		$0.00 with wildly wrong P&L. The actual fill prices live in
		orderActivityCollection[].executionLegs[]; compute a volume-
		weighted average across all execution legs of EXECUTION /
		FILL activities. Falls back to raw.Price only when the
		activity collection is missing entirely (defensive — Schwab
		populates it on every FILLED order in practice).
	*/
	fillPrice := volumeWeightedFillPrice(raw.OrderActivityCollection)
	if fillPrice == 0 {
		fillPrice = raw.Price
	}

	st := OrderStatus{
		OrderID:        orderID,
		RawStatus:      raw.Status,
		Quantity:       int(raw.Quantity),
		FilledQuantity: int(raw.FilledQuantity),
		FillPrice:      fillPrice,
		LimitPrice:     raw.Price,
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

/*
orderActivity mirrors the Schwab Trader API order-activity entry that
carries fill prices. Each activity holds one or more executionLegs;
each leg has the price + quantity of one partial fill.
*/
type orderActivity struct {
	ActivityType  string              `json:"activityType"`
	ExecutionType string              `json:"executionType"`
	ExecutionLegs []orderExecutionLeg `json:"executionLegs"`
}

type orderExecutionLeg struct {
	LegID    int     `json:"legId"`
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
}

/*
volumeWeightedFillPrice computes the average fill price across an
order's execution legs, weighted by quantity. Schwab returns each
partial fill (and most orders fill in a single execution) inside
orderActivityCollection[].executionLegs[]. Returns 0 when there are
no executions OR when the total quantity sums to 0 — caller treats
0 as "use the limit-price fallback".
*/
func volumeWeightedFillPrice(activities []orderActivity) float64 {
	var totalNotional float64
	var totalQuantity float64
	for _, act := range activities {
		// Schwab uses ActivityType="EXECUTION", ExecutionType="FILL" for
		// real fills. Anything else (cancels, replaces, etc.) shouldn't
		// move the fill-price needle.
		if act.ActivityType != "" && act.ActivityType != "EXECUTION" {
			continue
		}
		if act.ExecutionType != "" && act.ExecutionType != "FILL" {
			continue
		}
		for _, leg := range act.ExecutionLegs {
			if leg.Quantity <= 0 || leg.Price <= 0 {
				continue
			}
			totalNotional += leg.Price * leg.Quantity
			totalQuantity += leg.Quantity
		}
	}
	if totalQuantity <= 0 {
		return 0
	}
	return totalNotional / totalQuantity
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
