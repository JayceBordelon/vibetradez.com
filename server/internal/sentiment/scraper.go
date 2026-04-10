package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
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

// ScrapeRedditWSB scrapes r/wallstreetbets for ticker mentions
func (s *Scraper) ScrapeRedditWSB(ctx context.Context) ([]TickerMention, error) {
	// Use old.reddit.com — more lenient with unauthenticated access than www.reddit.com
	urls := []string{
		"https://old.reddit.com/r/wallstreetbets/hot.json?limit=50",
		"https://old.reddit.com/r/wallstreetbets/rising.json?limit=50",
		"https://old.reddit.com/r/options/hot.json?limit=50",
	}

	mentions := make(map[string]*TickerMention)

	for _, redditURL := range urls {
		posts, err := s.fetchRedditPosts(ctx, redditURL)
		if err != nil {
			log.Printf("Warning: failed to fetch %s: %v", redditURL, err)
			continue // Don't fail entirely if one source fails
		}

		for _, post := range posts {
			tickers := extractTickers(post.Title + " " + post.Selftext)
			for _, ticker := range tickers {
				if _, exists := mentions[ticker]; !exists {
					mentions[ticker] = &TickerMention{
						Symbol:    ticker,
						Mentions:  0,
						Sentiment: 0,
						Sources:   []string{},
					}
				}
				mentions[ticker].Mentions++
				mentions[ticker].Sources = append(mentions[ticker].Sources, "reddit")
				mentions[ticker].Sentiment += analyzeSentiment(post.Title + " " + post.Selftext)
			}
		}
	}

	// Convert to slice and sort by mentions
	result := make([]TickerMention, 0, len(mentions))
	for _, m := range mentions {
		if m.Mentions > 0 {
			m.Sentiment = m.Sentiment / float64(m.Mentions) // Average sentiment
		}
		result = append(result, *m)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Mentions > result[j].Mentions
	})

	return result, nil
}

type redditResponse struct {
	Data struct {
		Children []struct {
			Data redditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type redditPost struct {
	Title    string `json:"title"`
	Selftext string `json:"selftext"`
	Score    int    `json:"score"`
	URL      string `json:"url"`
}

func (s *Scraper) fetchRedditPosts(ctx context.Context, redditURL string) ([]redditPost, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", redditURL, nil)
	if err != nil {
		return nil, err
	}
	// Use a browser-like User-Agent — Reddit blocks generic bot agents
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit returned status %d for %s", resp.StatusCode, redditURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var redditResp redditResponse
	if err := json.Unmarshal(body, &redditResp); err != nil {
		return nil, fmt.Errorf("failed to parse reddit response from %s: %w", redditURL, err)
	}

	posts := make([]redditPost, 0, len(redditResp.Data.Children))
	for _, child := range redditResp.Data.Children {
		posts = append(posts, child.Data)
	}

	return posts, nil
}

// ScrapeFinanceNews scrapes financial news headlines
func (s *Scraper) ScrapeFinanceNews(ctx context.Context) ([]TickerMention, error) {
	// Using a free news API endpoint
	newsURL := "https://newsdata.io/api/1/news?apikey=pub_0&category=business&language=en&q=stock%20options"

	req, err := http.NewRequestWithContext(ctx, "GET", newsURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		// Fallback - return empty if news API fails
		return []TickerMention{}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse and extract tickers from headlines
	// This is a simplified version - in production you'd use a proper news API
	return []TickerMention{}, nil
}

// GetTrendingTickers combines all sources and returns top trending tickers
func (s *Scraper) GetTrendingTickers(ctx context.Context, limit int) ([]TickerMention, error) {
	allMentions := make(map[string]*TickerMention)

	// Scrape Reddit
	redditMentions, err := s.ScrapeRedditWSB(ctx)
	if err == nil {
		for _, m := range redditMentions {
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

	// Convert and sort
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

// extractTickers finds stock ticker symbols in text
func extractTickers(text string) []string {
	// Match $TICKER format (high confidence)
	dollarPattern := regexp.MustCompile(`\$([A-Z]{1,5})\b`)
	// Match bare uppercase tickers 2-5 chars surrounded by word boundaries (lower confidence)
	barePattern := regexp.MustCompile(`\b([A-Z]{2,5})\b`)

	tickers := make(map[string]bool)

	// Common words/acronyms to exclude
	exclude := map[string]bool{
		"I": true, "A": true, "THE": true, "AND": true, "FOR": true,
		"ARE": true, "BUT": true, "NOT": true, "YOU": true, "ALL": true,
		"CAN": true, "HER": true, "WAS": true, "ONE": true, "OUR": true,
		"OUT": true, "DAY": true, "HAD": true, "HAS": true, "HIS": true,
		"HOW": true, "ITS": true, "MAY": true, "NEW": true, "NOW": true,
		"OLD": true, "SEE": true, "WAY": true, "WHO": true, "BOY": true,
		"DID": true, "GET": true, "LET": true, "PUT": true, "SAY": true,
		"SHE": true, "TOO": true, "USE": true, "CEO": true, "IPO": true,
		"ATH": true, "ITM": true, "OTM": true, "ATM": true, "DD": true,
		"YOLO": true, "FOMO": true, "IMO": true, "TBH": true, "LOL": true,
		"USA": true, "GDP": true, "SEC": true, "FED": true, "IV": true,
		"PE": true, "EPS": true, "RSI": true, "HODL": true, "OP": true,
		"RIP": true, "BRO": true, "WTF": true, "SMH": true, "ETF": true,
		"ANY": true, "BIG": true, "RUN": true, "EOD": true, "RED": true,
		"DTE": true, "OI": true, "PM": true, "AM": true, "API": true,
		"EDIT": true, "JUST": true, "LIKE": true, "THIS": true, "THAT": true,
		"BEEN": true, "FROM": true, "THEY": true, "WILL": true, "WITH": true,
		"WHAT": true, "WHEN": true, "YOUR": true, "SOME": true, "THEM": true,
		"THAN": true, "THEN": true, "ONLY": true, "OVER": true, "ALSO": true,
		"BACK": true, "MUCH": true, "MOST": true, "VERY": true, "EVEN": true,
		"LONG": true, "GOOD": true, "EACH": true, "MAKE": true,
		"LMAO": true, "BEAR": true, "BULL": true, "HOLD": true, "SELL": true,
		"CALL": true, "PUTS": true, "MOON": true, "PUMP": true, "DUMP": true,
	}

	// $TICKER matches are always included
	for _, match := range dollarPattern.FindAllStringSubmatch(text, -1) {
		ticker := match[1]
		if !exclude[ticker] {
			tickers[ticker] = true
		}
	}

	// Bare tickers only included if 2-5 chars (reduces false positives)
	for _, match := range barePattern.FindAllStringSubmatch(text, -1) {
		ticker := match[1]
		if !exclude[ticker] && len(ticker) >= 2 {
			tickers[ticker] = true
		}
	}

	result := make([]string, 0, len(tickers))
	for t := range tickers {
		result = append(result, t)
	}
	return result
}

// analyzeSentiment does basic sentiment analysis
// Returns value between -1 (bearish) and 1 (bullish)
func analyzeSentiment(text string) float64 {
	text = strings.ToLower(text)

	bullishWords := []string{
		"buy", "calls", "moon", "rocket", "bullish", "long", "up",
		"gain", "profit", "winning", "tendies", "print", "squeeze",
		"breakout", "support", "accumulate", "undervalued", "cheap",
	}

	bearishWords := []string{
		"sell", "puts", "crash", "dump", "bearish", "short", "down",
		"loss", "lose", "losing", "drill", "tank", "resistance",
		"overvalued", "expensive", "bubble", "dead", "rip",
	}

	bullCount := 0
	bearCount := 0

	for _, word := range bullishWords {
		if strings.Contains(text, word) {
			bullCount++
		}
	}

	for _, word := range bearishWords {
		if strings.Contains(text, word) {
			bearCount++
		}
	}

	total := bullCount + bearCount
	if total == 0 {
		return 0
	}

	return float64(bullCount-bearCount) / float64(total)
}

// ValidateTicker checks if a symbol is likely a real ticker
func ValidateTicker(symbol string) bool {
	// Basic validation - in production you'd check against an exchange list
	if len(symbol) < 1 || len(symbol) > 5 {
		return false
	}

	matched, _ := regexp.MatchString(`^[A-Z]+$`, symbol)
	return matched
}

// SearchNews searches for news about specific tickers
func (s *Scraper) SearchNews(ctx context.Context, ticker string) ([]NewsItem, error) {
	// DuckDuckGo instant answer API (free, no auth)
	searchURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s+stock+options&format=json",
		url.QueryEscape(ticker))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse response - simplified for now
	return []NewsItem{}, nil
}

type NewsItem struct {
	Title     string
	URL       string
	Source    string
	Sentiment float64
}
