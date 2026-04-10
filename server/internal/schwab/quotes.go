package schwab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	marketDataBase = baseURL + "/marketdata/v1"
	cacheTTL       = 15 * time.Second // shared cache across all sessions
)

// ── Stock Quotes ──

type StockQuote struct {
	LastPrice         float64 `json:"lastPrice"`
	OpenPrice         float64 `json:"openPrice"`
	HighPrice         float64 `json:"highPrice"`
	LowPrice          float64 `json:"lowPrice"`
	ClosePrice        float64 `json:"closePrice"`
	BidPrice          float64 `json:"bidPrice"`
	AskPrice          float64 `json:"askPrice"`
	Mark              float64 `json:"mark"`
	MarkChange        float64 `json:"markChange"`
	MarkPercentChange float64 `json:"markPercentChange"`
	NetChange         float64 `json:"netChange"`
	NetPercentChange  float64 `json:"netPercentChange"`
	TotalVolume       int64   `json:"totalVolume"`
}

type quoteAPIEntry struct {
	AssetMainType string     `json:"assetMainType"`
	Symbol        string     `json:"symbol"`
	Quote         StockQuote `json:"quote"`
}

// ── Option Chain ──

type OptionContract struct {
	PutCall          string  `json:"putCall"`
	Symbol           string  `json:"symbol"`
	Description      string  `json:"description"`
	Bid              float64 `json:"bid"`
	Ask              float64 `json:"ask"`
	Last             float64 `json:"last"`
	Mark             float64 `json:"mark"`
	TotalVolume      int     `json:"totalVolume"`
	OpenInterest     int     `json:"openInterest"`
	Volatility       float64 `json:"volatility"`
	Delta            float64 `json:"delta"`
	Gamma            float64 `json:"gamma"`
	Theta            float64 `json:"theta"`
	Vega             float64 `json:"vega"`
	InTheMoney       bool    `json:"inTheMoney"`
	StrikePrice      float64 `json:"strikePrice"`
	ExpirationDate   string  `json:"expirationDate"`
	DaysToExpiration int     `json:"daysToExpiration"`
}

type OptionChain struct {
	Symbol          string           `json:"symbol"`
	Status          string           `json:"status"`
	UnderlyingPrice float64          `json:"underlyingPrice"`
	Volatility      float64          `json:"volatility"`
	Calls           []OptionContract `json:"calls"`
	Puts            []OptionContract `json:"puts"`
}

type optionChainRaw struct {
	Symbol          string                                 `json:"symbol"`
	Status          string                                 `json:"status"`
	UnderlyingPrice float64                                `json:"underlyingPrice"`
	Volatility      float64                                `json:"volatility"`
	CallExpDateMap  map[string]map[string][]OptionContract `json:"callExpDateMap"`
	PutExpDateMap   map[string]map[string][]OptionContract `json:"putExpDateMap"`
}

// ── Cache ──

type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

var (
	cacheMu sync.RWMutex
	cache   = make(map[string]cacheEntry)
)

func cacheGet(key string) (interface{}, bool) {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	entry, ok := cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func cacheSet(key string, data interface{}) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cache[key] = cacheEntry{data: data, expiresAt: time.Now().Add(cacheTTL)}
}

// ── Price History (OHLCV candles) ──

type Candle struct {
	Time   int64   `json:"time"` // epoch milliseconds from Schwab, converted to seconds for lightweight-charts
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

type priceHistoryRaw struct {
	Candles []struct {
		Datetime int64   `json:"datetime"`
		Open     float64 `json:"open"`
		High     float64 `json:"high"`
		Low      float64 `json:"low"`
		Close    float64 `json:"close"`
		Volume   int64   `json:"volume"`
	} `json:"candles"`
	Empty bool `json:"empty"`
}

// ── Public Methods ──

// GetQuotes fetches real-time quotes for the given symbols. Results are cached.
func (c *Client) GetQuotes(symbols []string) (map[string]StockQuote, error) {
	if len(symbols) == 0 {
		return nil, nil
	}

	symbolStr := strings.Join(symbols, ",")
	cacheKey := "quotes:" + symbolStr
	if cached, ok := cacheGet(cacheKey); ok {
		return cached.(map[string]StockQuote), nil
	}

	u := fmt.Sprintf("%s/quotes?symbols=%s&fields=quote", marketDataBase, url.QueryEscape(symbolStr))
	resp, err := c.AuthenticatedGet(u)
	if err != nil {
		return nil, fmt.Errorf("get quotes: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("quotes API returned %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]quoteAPIEntry
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode quotes: %w", err)
	}

	result := make(map[string]StockQuote, len(raw))
	for sym, entry := range raw {
		result[sym] = entry.Quote
	}

	cacheSet(cacheKey, result)
	return result, nil
}

// GetOptionChain fetches the option chain for a symbol with optional filters.
func (c *Client) GetOptionChain(symbol, contractType, fromDate, toDate string, strike float64) (*OptionChain, error) {
	cacheKey := fmt.Sprintf("chain:%s:%s:%.2f:%s:%s", symbol, contractType, strike, fromDate, toDate)
	if cached, ok := cacheGet(cacheKey); ok {
		return cached.(*OptionChain), nil
	}

	params := url.Values{"symbol": {symbol}}
	if contractType != "" {
		params.Set("contractType", contractType)
	}
	if fromDate != "" {
		params.Set("fromDate", fromDate)
	}
	if toDate != "" {
		params.Set("toDate", toDate)
	}
	if strike > 0 {
		params.Set("strike", fmt.Sprintf("%.2f", strike))
	}
	params.Set("includeUnderlyingQuote", "true")

	u := fmt.Sprintf("%s/chains?%s", marketDataBase, params.Encode())
	resp, err := c.AuthenticatedGet(u)
	if err != nil {
		return nil, fmt.Errorf("get option chain: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("chains API returned %d: %s", resp.StatusCode, string(body))
	}

	var raw optionChainRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode option chain: %w", err)
	}

	chain := &OptionChain{
		Symbol:          raw.Symbol,
		Status:          raw.Status,
		UnderlyingPrice: raw.UnderlyingPrice,
		Volatility:      raw.Volatility,
	}

	for _, strikes := range raw.CallExpDateMap {
		for _, contracts := range strikes {
			chain.Calls = append(chain.Calls, contracts...)
		}
	}
	for _, strikes := range raw.PutExpDateMap {
		for _, contracts := range strikes {
			chain.Puts = append(chain.Puts, contracts...)
		}
	}

	cacheSet(cacheKey, chain)
	return chain, nil
}

// GetPriceHistory fetches OHLCV candle data for a symbol.
// periodType: "day","month","year","ytd". frequencyType: "minute","daily","weekly","monthly".
func (c *Client) GetPriceHistory(symbol, periodType string, period int, frequencyType string, frequency int) ([]Candle, error) {
	cacheKey := fmt.Sprintf("history:%s:%s:%d:%s:%d", symbol, periodType, period, frequencyType, frequency)
	if cached, ok := cacheGet(cacheKey); ok {
		return cached.([]Candle), nil
	}

	params := url.Values{
		"symbol":        {symbol},
		"periodType":    {periodType},
		"period":        {fmt.Sprintf("%d", period)},
		"frequencyType": {frequencyType},
		"frequency":     {fmt.Sprintf("%d", frequency)},
	}

	u := fmt.Sprintf("%s/pricehistory?%s", marketDataBase, params.Encode())
	resp, err := c.AuthenticatedGet(u)
	if err != nil {
		return nil, fmt.Errorf("get price history: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("pricehistory API returned %d: %s", resp.StatusCode, string(body))
	}

	var raw priceHistoryRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode price history: %w", err)
	}

	if raw.Empty || len(raw.Candles) == 0 {
		return nil, nil
	}

	candles := make([]Candle, len(raw.Candles))
	for i, c := range raw.Candles {
		candles[i] = Candle{
			Time:   c.Datetime / 1000, // Schwab returns epoch ms, lightweight-charts wants seconds
			Open:   c.Open,
			High:   c.High,
			Low:    c.Low,
			Close:  c.Close,
			Volume: c.Volume,
		}
	}

	cacheSet(cacheKey, candles)
	return candles, nil
}

// FindContract looks up a specific option contract from the chain response.
func FindContract(chain *OptionChain, contractType string, strike float64, expiration string) *OptionContract {
	var list []OptionContract
	if strings.EqualFold(contractType, "CALL") {
		list = chain.Calls
	} else {
		list = chain.Puts
	}

	for i := range list {
		c := &list[i]
		strikeDiff := c.StrikePrice - strike
		if strikeDiff < 0 {
			strikeDiff = -strikeDiff
		}
		if strikeDiff < 0.01 && strings.HasPrefix(c.ExpirationDate, expiration) {
			return c
		}
	}
	return nil
}
