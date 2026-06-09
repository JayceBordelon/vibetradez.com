package schwab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

/*
Candle is one daily OHLCV bar from the Schwab price-history endpoint.
Datetime is epoch milliseconds (Schwab's native format).
*/
type Candle struct {
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	Volume   int64   `json:"volume"`
	Datetime int64   `json:"datetime"`
}

// PriceHistory is the Schwab price-history response for a symbol.
type PriceHistory struct {
	Symbol  string   `json:"symbol"`
	Empty   bool     `json:"empty"`
	Candles []Candle `json:"candles"`
}

/*
GetDailyPriceHistory returns up to one year of daily candles for a symbol,
used by the portfolio agent's get_price_history tool to reason about trend,
the 52-week range, and recent volatility. Cached via the shared quote cache
(daily bars don't move intraday, so the 45s TTL is plenty). Returns an
error on a non-200 or an empty result.
*/
func (c *Client) GetDailyPriceHistory(symbol string) (*PriceHistory, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil, fmt.Errorf("empty symbol")
	}
	cacheKey := "pricehistory:" + sym
	if cached, ok := cacheGet(cacheKey); ok {
		return cached.(*PriceHistory), nil
	}

	q := url.Values{}
	q.Set("symbol", sym)
	q.Set("periodType", "year")
	q.Set("period", "1")
	q.Set("frequencyType", "daily")
	q.Set("frequency", "1")
	u := fmt.Sprintf("%s/pricehistory?%s", marketDataBase, q.Encode())

	resp, err := c.AuthenticatedGet(u)
	if err != nil {
		return nil, fmt.Errorf("get price history: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("price history HTTP %d: %s", resp.StatusCode, string(body))
	}
	var ph PriceHistory
	if err := json.Unmarshal(body, &ph); err != nil {
		return nil, fmt.Errorf("decode price history: %w", err)
	}
	if ph.Empty || len(ph.Candles) == 0 {
		return nil, fmt.Errorf("no price history for %s", sym)
	}
	cacheSet(cacheKey, &ph)
	return &ph, nil
}
