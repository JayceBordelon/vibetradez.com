// Standalone helper that renders the morning email with synthetic data
// so you can eyeball the layout in a browser without booting the full
// stack. Not used in production. Run with:
//
//	go run ./cmd/preview-email > /tmp/email-preview.html && open /tmp/email-preview.html
package main

import (
	"fmt"
	"os"

	"vibetradez.com/internal/templates"
)

func main() {
	trades := []templates.Trade{
		{
			Symbol: "NVDA", ContractType: "CALL", StrikePrice: 950,
			Expiration: "2026-05-02", DTE: 5, EstimatedPrice: 4.20,
			Thesis:         "AI keynote scheduled for tomorrow with hyperscaler capex commentary expected; vol-of-vol elevated.",
			SentimentScore: 0.45, CurrentPrice: 945, TargetPrice: 8.40, StopLoss: 2.10,
			RiskLevel: "MEDIUM",
			Catalyst:  "Jensen keynote post-close", MentionCount: 320,
			Rank:     1,
			GPTScore: 9, ClaudeScore: 8, CombinedScore: 8.5,
			PickedByOpenAI: true, PickedByClaude: true,
			GPTRationale:    "Setup screams pre-event vol expansion. Spot is pinned within 0.6% of the 950 strike on a 5-DTE clock; an AI keynote is the cleanest catalyst we've had in weeks. Premium is below our mark-price filter and the chain shows healthy interest at the strike. Risk is the move already being priced in.",
			ClaudeRationale: "Bull case is real but the chain shows a fairly priced 5-DTE call rather than a steal. I like the catalyst alignment but I'd want a stop tighter than the suggested 2.10 — anything below the 943 pivot from last Friday and the thesis breaks.",
			GPTVerdict:      "Agree on direction. I'd take the conviction down a half point because IV is already lifted into the keynote.",
			ClaudeVerdict:   "Concur. The catalyst is clean and the strike picks itself. My one nit: the thesis ignores that AAPL prints the same week and could pull tape attention.",
		},
		{
			Symbol: "AMD", ContractType: "PUT", StrikePrice: 168,
			Expiration: "2026-05-02", DTE: 5, EstimatedPrice: 1.85,
			Thesis:         "AMD trading rich vs peer multiples; bearish flow detected on AI-comparable names.",
			SentimentScore: -0.2, CurrentPrice: 170, TargetPrice: 3.50, StopLoss: 0.90,
			RiskLevel: "HIGH",
			Catalyst:  "Bearish dark-pool prints", MentionCount: 78,
			Rank:     2,
			GPTScore: 7, ClaudeScore: 9, CombinedScore: 8.0,
			PickedByOpenAI: true, PickedByClaude: true,
			GPTRationale:    "I'm in but cautiously. Setup looks clean on the chart but momentum names cutting hard into earnings is exactly when shorts get squeezed by a single sympathy bid.",
			ClaudeRationale: "This is the cleanest contrarian setup on the screen. Dark-pool prints into a name trading 35x forward earnings during a sentiment-driven rally is textbook reversion. Strike is right at the gamma flip and DTE gives one full session for any post-keynote AI tape softness to spread.",
			GPTVerdict:      "Convicted setup. I downgraded the score for path risk only — a positive AI tape Monday morning rips this name back to 175 fast.",
			ClaudeVerdict:   "Solid analysis. The risk callout is right, but the dark-pool flow asymmetry is the tiebreaker for me.",
		},
	}

	yesterday := &templates.YesterdayRecap{
		Date:        "Apr 24",
		TotalPnL:    412.50,
		Winners:     6,
		Losers:      4,
		TotalTrades: 10,
		BestSymbol:  "TSLA",
		BestPnL:     185.00,
		WorstSymbol: "META",
		WorstPnL:    -78.00,
	}

	html, err := templates.RenderEmail(trades, "ChatGPT", "Claude", yesterday)
	if err != nil {
		fmt.Fprintln(os.Stderr, "render error:", err)
		os.Exit(1)
	}
	fmt.Print(html)
}
