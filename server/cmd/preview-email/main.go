/*
Standalone helper that renders the morning email with synthetic data
so you can eyeball the layout in a browser without booting the full
stack. Not used in production. Run with:

	go run ./cmd/preview-email > /tmp/email-preview.html && open /tmp/email-preview.html
*/
package main

import (
	"fmt"
	"os"

	"vibetradez.com/internal/templates"
)

func main() {
	// Top pick is rendered pre-resolution (the morning email goes out at
	// 9:25 ET, before the 9:30:00 executor snaps contracts to the live
	// chain) to exercise the "intent" branch of the template. Other picks
	// can be either; flip ContractResolved to preview the resolved layout.
	trades := []templates.Trade{
		{
			Symbol: "NVDA", ContractType: "CALL",
			Thesis:       "AI keynote scheduled for tomorrow with hyperscaler capex commentary expected; vol-of-vol elevated.",
			CurrentPrice: 945, TargetPrice: 970,
			RiskLevel: "MEDIUM",
			Catalyst:  "Jensen keynote post-close", MentionCount: 320,
			Rank:             1,
			Score:            9,
			Rationale:        "Setup screams pre-event vol expansion. Spot is pinned within 0.6 percent of the round-number strike on a 5-DTE clock; an AI keynote is the cleanest catalyst we've had in weeks. Risk is the move already being priced in.",
			TargetOTMPct:     0.5,
			MinDTE:           3,
			ContractResolved: false,
		},
		{
			Symbol: "AMD", ContractType: "PUT", StrikePrice: 168,
			Expiration: "2026-05-02", DTE: 5, EstimatedPrice: 1.85,
			Thesis:       "AMD trading rich vs peer multiples; bearish flow detected on AI-comparable names.",
			CurrentPrice: 170, TargetPrice: 165, StopLoss: 0.90,
			RiskLevel: "HIGH",
			Catalyst:  "Bearish dark-pool prints", MentionCount: 78,
			Rank:             2,
			Score:            8,
			Rationale:        "Cleanest contrarian setup on the screen. Dark-pool prints into a name trading 35x forward earnings during a momentum-driven rally is textbook reversion. Strike is right at the gamma flip and DTE gives one full session for any post-keynote AI tape softness to spread.",
			TargetOTMPct:     1.2,
			MinDTE:           3,
			ContractResolved: true,
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

	html, err := templates.RenderEmail(trades, "Claude", yesterday)
	if err != nil {
		fmt.Fprintln(os.Stderr, "render error:", err)
		os.Exit(1)
	}
	fmt.Print(html)
}
