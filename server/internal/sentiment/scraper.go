package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type TickerMention struct {
	Symbol    string
	Mentions  int
	Sentiment float64 // -1 to 1
	Sources   []string
}

type Scraper struct {
	httpClient *http.Client
}

func NewScraper() *Scraper {
	return &Scraper{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

/*
GetTrendingTickers combines all market signal sources and returns the top
trending tickers. Each source contributes independently; failures are
logged but do not block other sources from running.
*/
func (s *Scraper) GetTrendingTickers(ctx context.Context, limit int) ([]TickerMention, error) {
	allMentions := make(map[string]*TickerMention)

	merge := func(mentions []TickerMention) {
		for _, m := range mentions {
			if existing, ok := allMentions[m.Symbol]; ok {
				existing.Mentions += m.Mentions
				existing.Sentiment = (existing.Sentiment + m.Sentiment) / 2
				existing.Sources = append(existing.Sources, m.Sources...)
			} else {
				copy := m
				allMentions[m.Symbol] = &copy
			}
		}
	}

	// StockTwits: social momentum + trending scores
	if mentions, err := s.scrapeStockTwitsTrending(ctx); err != nil {
		log.Printf("Warning: StockTwits trending: %v", err)
	} else {
		merge(mentions)
	}

	// Yahoo Finance: trending tickers and market movers
	if mentions, err := s.scrapeYahooTrending(ctx); err != nil {
		log.Printf("Warning: Yahoo trending: %v", err)
	} else {
		merge(mentions)
	}

	// Finviz: unusual volume and most active options
	if mentions, err := s.scrapeFinvizSignals(ctx); err != nil {
		log.Printf("Warning: Finviz signals: %v", err)
	} else {
		merge(mentions)
	}

	// SEC EDGAR: recent filings with catalyst keywords (8-K)
	if mentions, err := s.scrapeEDGARCatalysts(ctx); err != nil {
		log.Printf("Warning: EDGAR catalysts: %v", err)
	} else {
		merge(mentions)
	}

	// Convert and sort by mention count
	result := make([]TickerMention, 0, len(allMentions))
	for _, m := range allMentions {
		result = append(result, *m)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Mentions > result[j].Mentions
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// ---------- StockTwits ----------

type stockTwitsResponse struct {
	Symbols []struct {
		Symbol  string `json:"symbol"`
		Title   string `json:"title"`
		Summary string `json:"trending_summary"`
	} `json:"symbols"`
}

/*
scrapeStockTwitsTrending fetches StockTwits' trending symbols endpoint.
Returns up to 30 symbols with trending scores and human-readable
summaries explaining why each ticker is trending. No auth required.
*/
func (s *Scraper) scrapeStockTwitsTrending(ctx context.Context) ([]TickerMention, error) {
	body, err := s.fetchJSON(ctx, "https://api.stocktwits.com/api/2/trending/symbols.json")
	if err != nil {
		return nil, fmt.Errorf("StockTwits fetch: %w", err)
	}

	var resp stockTwitsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("StockTwits parse: %w", err)
	}

	mentions := make([]TickerMention, 0, len(resp.Symbols))
	for _, sym := range resp.Symbols {
		ticker := strings.TrimSpace(sym.Symbol)
		if !isEquityTicker(ticker) {
			continue
		}
		sentiment := estimateSentimentFromText(sym.Summary)
		mentions = append(mentions, TickerMention{
			Symbol:    ticker,
			Mentions:  2, // weight StockTwits trending higher than a single mention
			Sentiment: sentiment,
			Sources:   []string{"stocktwits-trending"},
		})
	}
	return mentions, nil
}

// ---------- Yahoo Finance ----------

type yahooTrendingResponse struct {
	Finance struct {
		Result []struct {
			Quotes []struct {
				Symbol string `json:"symbol"`
			} `json:"quotes"`
		} `json:"result"`
	} `json:"finance"`
}

type yahooMoversResponse struct {
	Finance struct {
		Result []struct {
			Quotes []struct {
				Symbol                     string `json:"symbol"`
				RegularMarketChangePercent struct {
					Raw float64 `json:"raw"`
				} `json:"regularMarketChangePercent"`
			} `json:"quotes"`
		} `json:"result"`
	} `json:"finance"`
}

/*
scrapeYahooTrending fetches Yahoo Finance's trending tickers and market
movers (gainers + losers). These are public JSON endpoints that do not
require authentication.
*/
func (s *Scraper) scrapeYahooTrending(ctx context.Context) ([]TickerMention, error) {
	urls := []string{
		"https://query2.finance.yahoo.com/v1/finance/trending/US?count=25",
		"https://query2.finance.yahoo.com/v1/finance/screener/predefined/saved?scrIds=day_gainers&count=15",
		"https://query2.finance.yahoo.com/v1/finance/screener/predefined/saved?scrIds=day_losers&count=15",
		"https://query2.finance.yahoo.com/v1/finance/screener/predefined/saved?scrIds=most_actives&count=15",
	}

	mentions := make(map[string]*TickerMention)

	for _, u := range urls {
		body, err := s.fetchJSON(ctx, u)
		if err != nil {
			log.Printf("Warning: Yahoo fetch %s: %v", u, err)
			continue
		}

		/*
			Trending endpoint has a different shape than screener endpoints.
			Try trending first, fall back to screener/movers.
		*/
		var trending yahooTrendingResponse
		if err := json.Unmarshal(body, &trending); err == nil {
			for _, r := range trending.Finance.Result {
				for _, q := range r.Quotes {
					sym := strings.TrimSpace(q.Symbol)
					if sym == "" || !isEquityTicker(sym) {
						continue
					}
					if _, ok := mentions[sym]; !ok {
						mentions[sym] = &TickerMention{Symbol: sym, Sources: []string{}}
					}
					mentions[sym].Mentions++
					mentions[sym].Sources = append(mentions[sym].Sources, "yahoo-trending")
				}
			}
		}

		var movers yahooMoversResponse
		if err := json.Unmarshal(body, &movers); err == nil {
			for _, r := range movers.Finance.Result {
				for _, q := range r.Quotes {
					sym := strings.TrimSpace(q.Symbol)
					if sym == "" || !isEquityTicker(sym) {
						continue
					}
					if _, ok := mentions[sym]; !ok {
						mentions[sym] = &TickerMention{Symbol: sym, Sources: []string{}}
					}
					mentions[sym].Mentions++
					mentions[sym].Sources = append(mentions[sym].Sources, "yahoo-movers")
					// Large move = stronger sentiment signal
					pct := q.RegularMarketChangePercent.Raw
					if pct > 3 {
						mentions[sym].Sentiment += 0.5
					} else if pct < -3 {
						mentions[sym].Sentiment -= 0.5
					}
				}
			}
		}
	}

	result := make([]TickerMention, 0, len(mentions))
	for _, m := range mentions {
		if m.Mentions > 0 {
			m.Sentiment = m.Sentiment / float64(m.Mentions)
		}
		result = append(result, *m)
	}
	return result, nil
}

// ---------- Finviz ----------

/*
scrapeFinvizSignals scrapes Finviz's signal pages for unusual volume and
most active options tickers. Finviz serves HTML but with a consistent
table structure that is straightforward to parse.
*/
func (s *Scraper) scrapeFinvizSignals(ctx context.Context) ([]TickerMention, error) {
	/*
		ta_unusualvolume: stocks trading at significantly higher volume than normal
		ta_mostactive: highest absolute volume
	*/
	urls := []string{
		"https://finviz.com/screener.ashx?v=111&s=ta_unusualvolume",
		"https://finviz.com/screener.ashx?v=111&s=ta_mostactive",
	}

	mentions := make(map[string]*TickerMention)

	for _, u := range urls {
		body, err := s.fetchHTML(ctx, u)
		if err != nil {
			log.Printf("Warning: Finviz fetch %s: %v", u, err)
			continue
		}

		tickers := extractTickersFromFinvizHTML(body)
		source := "finviz-unusual-volume"
		if strings.Contains(u, "mostactive") {
			source = "finviz-most-active"
		}
		for _, sym := range tickers {
			if _, ok := mentions[sym]; !ok {
				mentions[sym] = &TickerMention{Symbol: sym, Sources: []string{}}
			}
			mentions[sym].Mentions++
			mentions[sym].Sources = append(mentions[sym].Sources, source)
		}
	}

	result := make([]TickerMention, 0, len(mentions))
	for _, m := range mentions {
		result = append(result, *m)
	}
	return result, nil
}

/*
extractTickersFromFinvizHTML pulls ticker symbols from Finviz screener
HTML. Finviz renders tickers as links inside the screener results table
with class "screener-link-primary".
*/
func extractTickersFromFinvizHTML(body string) []string {
	tickers := make([]string, 0)
	seen := make(map[string]bool)

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return tickers
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, "screener-link-primary") {
					// The ticker text is the first child text node
					if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
						sym := strings.TrimSpace(n.FirstChild.Data)
						if isEquityTicker(sym) && !seen[sym] {
							tickers = append(tickers, sym)
							seen[sym] = true
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return tickers
}

// ---------- SEC EDGAR ----------

type edgarSearchResponse struct {
	Hits struct {
		Hits []struct {
			Source struct {
				DisplayNames []string `json:"display_names"`
				Form         string   `json:"form_type"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

/*
scrapeEDGARCatalysts queries the SEC EDGAR full-text search API (EFTS)
for recent 8-K filings containing catalyst keywords (mergers,
acquisitions, material agreements). The EFTS API is free, public, and
run by the US government, making it extremely stable.
*/
func (s *Scraper) scrapeEDGARCatalysts(ctx context.Context) ([]TickerMention, error) {
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// Search for 8-K filings with catalyst language
	searchURL := fmt.Sprintf(
		"https://efts.sec.gov/LATEST/search-index?q=%%22material+definitive+agreement%%22+OR+%%22merger%%22+OR+%%22acquisition%%22&forms=8-K&dateRange=custom&startdt=%s&enddt=%s&from=0&size=40",
		yesterday, today,
	)

	body, err := s.fetchEDGAR(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("EDGAR search: %w", err)
	}

	var resp edgarSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("EDGAR parse: %w", err)
	}

	// EDGAR display_names contain "Company Name (TICKER) (CIK ...)"
	tickerPattern := regexp.MustCompile(`\(([A-Z]{1,5})\)`)

	mentions := make(map[string]*TickerMention)
	for _, hit := range resp.Hits.Hits {
		for _, name := range hit.Source.DisplayNames {
			matches := tickerPattern.FindAllStringSubmatch(name, -1)
			for _, m := range matches {
				sym := m[1]
				if !isEquityTicker(sym) || sym == "CIK" {
					continue
				}
				if _, ok := mentions[sym]; !ok {
					mentions[sym] = &TickerMention{
						Symbol:  sym,
						Sources: []string{},
					}
				}
				mentions[sym].Mentions++
				mentions[sym].Sources = append(mentions[sym].Sources, "edgar-8k-catalyst")
			}
		}
	}

	result := make([]TickerMention, 0, len(mentions))
	for _, m := range mentions {
		result = append(result, *m)
	}
	return result, nil
}

/*
fetchEDGAR makes an HTTP request to SEC EDGAR with the required
User-Agent format (SEC blocks requests without a proper contact UA).
*/
func (s *Scraper) fetchEDGAR(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "VibeTradez bordelonjayce@gmail.com")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d for %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
}

// ---------- Helpers ----------

func (s *Scraper) fetchJSON(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "VibeTradez/1.0 (bordelonjayce@gmail.com)")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d for %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
}

func (s *Scraper) fetchHTML(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	// Finviz requires a browser-like UA
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

/*
estimateSentimentFromText does a quick keyword scan on a short text
blurb (like StockTwits' trending_summary) and returns a value between
-1 (bearish) and 1 (bullish).
*/
func estimateSentimentFromText(text string) float64 {
	lower := strings.ToLower(text)

	bullish := []string{
		"buy", "bull", "upgrade", "beat", "surge", "rally", "breakout",
		"squeeze", "calls", "upside", "growth", "profit", "gain",
	}
	bearish := []string{
		"sell", "bear", "downgrade", "miss", "crash", "drop", "puts",
		"downside", "loss", "decline", "warning", "risk", "short",
	}

	bull, bear := 0, 0
	for _, w := range bullish {
		if strings.Contains(lower, w) {
			bull++
		}
	}
	for _, w := range bearish {
		if strings.Contains(lower, w) {
			bear++
		}
	}

	total := bull + bear
	if total == 0 {
		return 0
	}
	return float64(bull-bear) / float64(total)
}

// SourceStatus describes the health of a single market signal source.
type SourceStatus struct {
	Name    string
	OK      bool
	Tickers int
	Err     string
	Latency time.Duration
}

/*
ProbeAll tests every scraping source and returns per-source results.
Used both for startup verification and the /health endpoint.
*/
func (s *Scraper) ProbeAll(ctx context.Context) []SourceStatus {
	type probe struct {
		name string
		fn   func(context.Context) ([]TickerMention, error)
	}

	probes := []probe{
		{"stocktwits", s.scrapeStockTwitsTrending},
		{"yahoo", s.scrapeYahooTrending},
		{"finviz", s.scrapeFinvizSignals},
		{"edgar", s.scrapeEDGARCatalysts},
	}

	results := make([]SourceStatus, len(probes))
	for i, p := range probes {
		start := time.Now()
		mentions, err := p.fn(ctx)
		elapsed := time.Since(start)

		ss := SourceStatus{Name: p.name, Latency: elapsed}
		if err != nil {
			ss.Err = err.Error()
		} else {
			ss.OK = true
			ss.Tickers = len(mentions)
		}
		results[i] = ss
	}
	return results
}

/*
isEquityTicker validates that a string looks like a US equity ticker
(1-5 uppercase letters, no dots or numbers which indicate preferred
shares, warrants, etc.).
*/
func isEquityTicker(sym string) bool {
	if len(sym) < 1 || len(sym) > 5 {
		return false
	}
	matched, _ := regexp.MatchString(`^[A-Z]+$`, sym)
	return matched
}
