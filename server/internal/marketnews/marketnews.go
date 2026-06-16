// Package marketnews provides free, no-account market-news and retail-hype
// signals for the portfolio agent: per-ticker headlines (Yahoo Finance and
// Google News RSS) and social hype (StockTwits trending symbols + per-symbol
// bull/bear sentiment). Every source here is a public, keyless endpoint, so
// there are no credentials to manage.
//
// StockTwits sentiment is returned as AGGREGATE counts only (never raw
// message text), which keeps the public session transcript clear of
// republished third-party content while still giving the agent a hype read.
package marketnews

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// A descriptive User-Agent is sent on every request: politeness for the RSS
// feeds and a hard requirement for SEC-style endpoints we may add later.
const defaultUserAgent = "VibeTradez/1.0 (+https://vibetradez.com; contact bordelonjayce@gmail.com)"

// Client fetches free market-news + hype signals over HTTP. Stateless apart
// from the shared *http.Client, so it is safe for concurrent use.
type Client struct {
	http *http.Client
	ua   string
}

// NewClient returns a Client with a bounded per-request timeout.
func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 8 * time.Second},
		ua:   defaultUserAgent,
	}
}

// NewsItem is one headline for a ticker.
type NewsItem struct {
	Symbol    string    `json:"symbol"`
	Headline  string    `json:"headline"`
	Source    string    `json:"source"`
	URL       string    `json:"url"`
	Published time.Time `json:"published,omitempty"`
}

// TrendingTicker is one symbol from the StockTwits trending list, in rank
// order (rank 1 is hottest).
type TrendingTicker struct {
	Rank           int    `json:"rank"`
	Symbol         string `json:"symbol"`
	Name           string `json:"name,omitempty"`
	WatchlistCount int    `json:"watchlist_count,omitempty"`
}

// Sentiment is the aggregate retail mood for a ticker, derived from the
// bull/bear tags on a sample of recent StockTwits messages. Raw messages are
// never surfaced — only the counts.
type Sentiment struct {
	Symbol       string  `json:"symbol"`
	MessageCount int     `json:"messages_sampled"`
	BullishCount int     `json:"bullish"`
	BearishCount int     `json:"bearish"`
	BullishPct   float64 `json:"bullish_pct_of_tagged"`
}

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json, application/rss+xml, text/xml, */*")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", rawURL, resp.StatusCode)
	}
	return body, nil
}

/*
TickerNews returns recent headlines for each symbol, merged from Yahoo
Finance and Google News RSS, deduped by headline, newest first, capped at
perSymbol (default 5) per ticker. A single feed or symbol failing is
non-fatal: whatever was gathered is returned. An error comes back only when
nothing could be fetched at all.
*/
func (c *Client) TickerNews(symbols []string, perSymbol int) ([]NewsItem, error) {
	if perSymbol <= 0 {
		perSymbol = 5
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	var all []NewsItem
	var lastErr error
	attempted := 0
	for _, sym := range symbols {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			continue
		}
		attempted++
		items, err := c.tickerNewsOne(ctx, sym, perSymbol)
		if err != nil {
			lastErr = err
		}
		all = append(all, items...)
	}
	if len(all) == 0 && lastErr != nil && attempted > 0 {
		return nil, lastErr
	}
	return all, nil
}

func (c *Client) tickerNewsOne(ctx context.Context, sym string, limit int) ([]NewsItem, error) {
	var items []NewsItem
	var lastErr error

	yahoo := "https://feeds.finance.yahoo.com/rss/2.0/headline?s=" + url.QueryEscape(sym) + "&region=US&lang=en-US"
	if body, err := c.get(ctx, yahoo); err == nil {
		items = append(items, parseRSS(body, sym, "Yahoo Finance")...)
	} else {
		lastErr = err
	}

	google := "https://news.google.com/rss/search?q=" + url.QueryEscape(sym+" stock") + "&hl=en-US&gl=US&ceid=US:en"
	if body, err := c.get(ctx, google); err == nil {
		items = append(items, parseRSS(body, sym, "Google News")...)
	} else {
		lastErr = err
	}

	items = dedupNews(items)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Published.After(items[j].Published) })
	if len(items) > limit {
		items = items[:limit]
	}
	if len(items) == 0 {
		return nil, lastErr
	}
	return items, nil
}

type rssFeed struct {
	Items []rssItem `xml:"channel>item"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
	Source  string `xml:"source"`
}

// parseRSS turns an RSS 2.0 body into NewsItems tagged with the symbol and a
// fallback source label. Malformed XML yields no items rather than an error.
func parseRSS(body []byte, symbol, source string) []NewsItem {
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil
	}
	out := make([]NewsItem, 0, len(feed.Items))
	for _, it := range feed.Items {
		title := strings.TrimSpace(it.Title)
		if title == "" {
			continue
		}
		src := source
		if s := strings.TrimSpace(it.Source); s != "" {
			src = s
		}
		out = append(out, NewsItem{
			Symbol:    symbol,
			Headline:  title,
			Source:    src,
			URL:       strings.TrimSpace(it.Link),
			Published: parsePubDate(it.PubDate),
		})
	}
	return out
}

func parsePubDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, "Mon, 2 Jan 2006 15:04:05 -0700", time.RFC822Z, time.RFC822} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// dedupNews drops repeat headlines (case-insensitive, first 80 chars) so the
// same story syndicated to both Yahoo and Google appears once.
func dedupNews(items []NewsItem) []NewsItem {
	seen := make(map[string]bool, len(items))
	out := items[:0]
	for _, it := range items {
		key := strings.ToLower(strings.TrimSpace(it.Headline))
		if len(key) > 80 {
			key = key[:80]
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, it)
	}
	return out
}

/*
TrendingTickers returns the StockTwits trending symbols (rank order, hottest
first), capped at limit (default 15). Use it to discover names retail is
piling into that the agent is not already watching.
*/
func (c *Client) TrendingTickers(limit int) ([]TrendingTicker, error) {
	if limit <= 0 {
		limit = 15
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	body, err := c.get(ctx, "https://api.stocktwits.com/api/2/trending/symbols.json")
	if err != nil {
		return nil, err
	}
	return parseTrending(body, limit)
}

type stTrendingResp struct {
	Symbols []struct {
		Symbol         string `json:"symbol"`
		Title          string `json:"title"`
		WatchlistCount int    `json:"watchlist_count"`
	} `json:"symbols"`
}

func parseTrending(body []byte, limit int) ([]TrendingTicker, error) {
	var r stTrendingResp
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse stocktwits trending: %w", err)
	}
	out := make([]TrendingTicker, 0, len(r.Symbols))
	for i, s := range r.Symbols {
		if i >= limit {
			break
		}
		out = append(out, TrendingTicker{
			Rank:           i + 1,
			Symbol:         strings.ToUpper(strings.TrimSpace(s.Symbol)),
			Name:           strings.TrimSpace(s.Title),
			WatchlistCount: s.WatchlistCount,
		})
	}
	return out, nil
}

/*
SocialSentiment samples recent StockTwits messages for one symbol and
returns the aggregate bull/bear breakdown. Only counts are returned, never
message text. Tagged means messages the author marked Bullish or Bearish;
BullishPct is over the tagged subset.
*/
func (c *Client) SocialSentiment(symbol string) (Sentiment, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return Sentiment{}, fmt.Errorf("social sentiment: empty symbol")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	body, err := c.get(ctx, "https://api.stocktwits.com/api/2/streams/symbol/"+url.PathEscape(symbol)+".json")
	if err != nil {
		return Sentiment{}, err
	}
	return parseSentiment(body, symbol)
}

type stStreamResp struct {
	Messages []struct {
		Entities struct {
			Sentiment *struct {
				Basic string `json:"basic"`
			} `json:"sentiment"`
		} `json:"entities"`
	} `json:"messages"`
}

func parseSentiment(body []byte, symbol string) (Sentiment, error) {
	var r stStreamResp
	if err := json.Unmarshal(body, &r); err != nil {
		return Sentiment{}, fmt.Errorf("parse stocktwits stream: %w", err)
	}
	s := Sentiment{Symbol: symbol, MessageCount: len(r.Messages)}
	for _, m := range r.Messages {
		if m.Entities.Sentiment == nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(m.Entities.Sentiment.Basic)) {
		case "bullish":
			s.BullishCount++
		case "bearish":
			s.BearishCount++
		}
	}
	if tagged := s.BullishCount + s.BearishCount; tagged > 0 {
		s.BullishPct = math.Round(float64(s.BullishCount)/float64(tagged)*1000) / 10
	}
	return s, nil
}
