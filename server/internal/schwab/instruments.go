package schwab

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"
	"sync"
)

/*
marketCapMillionsToUSD scales Schwab's instruments-fundamental marketCap
to absolute USD. Schwab reports marketCap in MILLIONS of dollars in this
projection, so the floor comparison (which is in absolute USD) needs the
×1e6. The scale is a named constant so it's a one-line flip if a live
response shows the field is already absolute. Erring toward this scaling is
the safe direction: if the value were already absolute, scaling makes the
floor lenient (nothing falsely blocked) rather than rejecting real names.
*/
const marketCapMillionsToUSD = 1e6

var marketCapUnitsLogOnce sync.Once

/*
MarketCap returns the underlying's market capitalization in absolute USD.
The /quotes fundamental block does NOT carry market cap, so this hits the
separate /instruments?projection=fundamental endpoint (whose richer
fundamental object includes marketCap + sharesOutstanding). Cached via the
shared quote cache. Returns 0 with an error when the symbol or its market
cap is unavailable; the caller decides whether to fail open or closed.

The first successful call logs the raw + scaled value once so the
millions-vs-absolute units assumption (see marketCapMillionsToUSD) can be
verified against a real response without grepping logs per call.
*/
func (c *Client) MarketCap(symbol string) (float64, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0, fmt.Errorf("empty symbol")
	}
	cacheKey := "mktcap:" + sym
	if cached, ok := cacheGet(cacheKey); ok {
		return cached.(float64), nil
	}

	u := fmt.Sprintf("%s/instruments?symbol=%s&projection=fundamental", marketDataBase, url.QueryEscape(sym))
	resp, err := c.AuthenticatedGet(u)
	if err != nil {
		return 0, fmt.Errorf("get instruments: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("instruments API returned %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Instruments []struct {
			Fundamental struct {
				MarketCap float64 `json:"marketCap"`
			} `json:"fundamental"`
		} `json:"instruments"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, fmt.Errorf("decode instruments: %w", err)
	}
	if len(raw.Instruments) == 0 {
		return 0, fmt.Errorf("no instrument returned for %s", sym)
	}
	rawCap := raw.Instruments[0].Fundamental.MarketCap
	if rawCap <= 0 {
		return 0, fmt.Errorf("no market cap available for %s", sym)
	}
	usd := rawCap * marketCapMillionsToUSD
	marketCapUnitsLogOnce.Do(func() {
		log.Printf("schwab: MarketCap units check, %s raw=%.4g scaled=$%.0f (assuming Schwab reports millions; flip marketCapMillionsToUSD if a real response shows otherwise)", sym, rawCap, usd)
	})
	cacheSet(cacheKey, usd)
	return usd, nil
}

/*
Fundamentals is the subset of Schwab's instruments-fundamental object the
portfolio agent reasons over: valuation, profitability, the 52-week range,
volatility (beta), dividend, and the next earnings date. MarketCap is
scaled to absolute USD (see marketCapMillionsToUSD); the rest are passed
through as Schwab reports them.
*/
type Fundamentals struct {
	Symbol           string  `json:"symbol"`
	MarketCap        float64 `json:"market_cap"`
	PERatio          float64 `json:"pe_ratio"`
	EPS              float64 `json:"eps_ttm"`
	High52           float64 `json:"high_52"`
	Low52            float64 `json:"low_52"`
	Beta             float64 `json:"beta"`
	DividendYield    float64 `json:"dividend_yield"`
	DividendAmount   float64 `json:"dividend_amount"`
	NextEarningsDate string  `json:"next_earnings_date"`
	Avg10DayVolume   float64 `json:"avg_10_day_volume"`
}

/*
GetFundamentals returns the richer fundamental block for a symbol from the
Schwab /instruments fundamental projection (the /quotes endpoint does not
carry these). Cached via the shared quote cache. Returns an error when the
symbol or its fundamental block is unavailable.
*/
func (c *Client) GetFundamentals(symbol string) (*Fundamentals, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil, fmt.Errorf("empty symbol")
	}
	cacheKey := "fundamentals:" + sym
	if cached, ok := cacheGet(cacheKey); ok {
		return cached.(*Fundamentals), nil
	}

	u := fmt.Sprintf("%s/instruments?symbol=%s&projection=fundamental", marketDataBase, url.QueryEscape(sym))
	resp, err := c.AuthenticatedGet(u)
	if err != nil {
		return nil, fmt.Errorf("get instruments: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("instruments API returned %d: %s", resp.StatusCode, string(body))
	}
	var raw struct {
		Instruments []struct {
			Fundamental struct {
				Symbol           string  `json:"symbol"`
				MarketCap        float64 `json:"marketCap"`
				PeRatio          float64 `json:"peRatio"`
				EpsTTM           float64 `json:"epsTTM"`
				High52           float64 `json:"high52"`
				Low52            float64 `json:"low52"`
				Beta             float64 `json:"beta"`
				DivYield         float64 `json:"dividendYield"`
				DivAmount        float64 `json:"dividendAmount"`
				NextEarningsDate string  `json:"nextEarningsDate"`
				Avg10DaysVolume  float64 `json:"avg10DaysVolume"`
			} `json:"fundamental"`
		} `json:"instruments"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode instruments: %w", err)
	}
	if len(raw.Instruments) == 0 {
		return nil, fmt.Errorf("no instrument returned for %s", sym)
	}
	f := raw.Instruments[0].Fundamental
	out := &Fundamentals{
		Symbol:           sym,
		MarketCap:        f.MarketCap * marketCapMillionsToUSD,
		PERatio:          f.PeRatio,
		EPS:              f.EpsTTM,
		High52:           f.High52,
		Low52:            f.Low52,
		Beta:             f.Beta,
		DividendYield:    f.DivYield,
		DividendAmount:   f.DivAmount,
		NextEarningsDate: f.NextEarningsDate,
		Avg10DayVolume:   f.Avg10DaysVolume,
	}
	cacheSet(cacheKey, out)
	return out, nil
}
