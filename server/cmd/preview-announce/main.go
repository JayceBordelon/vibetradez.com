// preview-announce renders the launch announcement email to a local
// HTML file using the same templates package the production server
// uses, so we can preview the rendered email in a browser before
// firing /admin/announce against production.
//
// Usage:
//
//	go run ./cmd/preview-announce > /tmp/announce_preview.html
//	open -a "Google Chrome" /tmp/announce_preview.html
//
// The payload here is the source of truth for the launch email.
// When the preview looks right, the same struct fields are POSTed
// (as JSON) to /admin/announce on production.
package main

import (
	"fmt"
	"os"

	"vibetradez.com/internal/templates"
)

func main() {
	html, err := templates.RenderAnnouncementEmail(launchAnnouncement)
	if err != nil {
		fmt.Fprintf(os.Stderr, "render failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(html)
}

// launchAnnouncement is the full payload for the jaycetrades →
// vibetradez relaunch email. Keep the prose here in sync with what
// gets POSTed to /admin/announce.
var launchAnnouncement = templates.AnnouncementData{
	Subject:      "VibeTradez relaunch: dual-model trade picker, head-to-head model comparison, and a new domain",
	Badge:        "Rebrand + Relaunch",
	Headline:     "jaycetrades is now vibetradez.com — and the entire trade picker just got rebuilt",
	HeroImageURL: "https://media.licdn.com/dms/image/v2/D5622AQEfCEI8dXczVw/feedshare-shrink_800/feedshare-shrink_800/0/1719904786508?e=2147483647&v=beta&t=jmqaTzPrRC_dl6YvHOPCcqfNGy3YesDYf2PmV1ZqpH8",
	Sections: []templates.AnnouncementSection{
		{
			Title: "Same project, new name",
			Body:  "The trading service you subscribed to as JayceTrades is now VibeTradez at https://vibetradez.com. Your subscription carries over automatically — same email list, same morning cadence, same free price, same hypothetical-only P&L. The old jaycetrades.com domain is going away, so update any bookmarks. Everything below describes what changed inside.",
		},
		{
			Title: "Two AI models now pick your trades, not one",
			Body:  "The biggest change. The morning pipeline used to run a single OpenAI model to generate the daily picks. Now both OpenAI GPT-5.4 AND Anthropic Claude Opus 4.6 run the exact same workflow independently — same Reddit sentiment, same Schwab market data API, same options chain with greeks, same live web search for catalysts. Neither model sees the other's output. Each independently produces its own ranked top 10 picks for the day, and the cron unions both pick sets so you see every trade either model thinks is worth taking. When both models picked the same ticker we mark it as a consensus pick and bubble it to the top. Per-pick scores, rationales, and brand-marked attribution from both sides are visible on every card.",
		},
		{
			Title: "Filter the dashboard by which model picked it",
			Body:  "New segmented control in the nav bar with All / OpenAI / Claude. The filter applies globally across the dashboard, the history page, and every API call. Toggle to OpenAI to see exactly what GPT picked today. Toggle to Claude to see what Claude picked. All shows the merged consensus view ranked by combined score. Persists across page reloads so it remembers what you were looking at.",
		},
		{
			Title: "Brand new model comparison page",
			Body:  "https://vibetradez.com/models is the new headline feature. Side-by-side OpenAI vs Anthropic backtest with cumulative P&L curve, agreement rate (how often the two models scored within 1 of each other), best and worst pick per model, and a configurable date range — week, month, year, or all time. Find out which model would actually have made you more money if you'd only listened to one of them.",
		},
		{
			Title: "The dashboard, history, and analytics got a full overhaul",
			Body:  "Every pick card now carries a dual-model conviction band with both models' scores and rationales. The equity curve on the history page shows all four Top-N strategies (Top 1 / 3 / 5 / 10) overlaid on the same chart so you can compare them at a glance. Win rate, agreement rate, ROC, expectancy, profit factor, max drawdown — every percentage stat now shades from red to green continuously instead of snapping to three buckets. Mobile layout, animations, and overflow handling were rebuilt from scratch. The history page also got a smoother daily-breakdown dropdown and a fixed crash that used to hit any week with no historical trades.",
		},
		{
			Title: "Email upgrades",
			Body:  "The morning email now carries the dual-model conviction band per pick — both rationales side by side when consensus, single side when only one model picked it — with each provider's brand color so you can tell at a glance who picked what. End-of-day and weekly digest emails will surface a similar attribution leaderboard.",
		},
		{
			Title: "Read the fine print",
			Body:  "The FAQ and methodology sections (linked from the footer of every page) are fully rewritten to explain how the two-model architecture works, what consensus picks mean, how the combined score is computed, and how the Models page math is calculated. The Terms of Service has been updated to reflect both LLM providers and the new domain.",
		},
		{
			Title: "Honesty time",
			Body:  "This is still hypothetical P&L. I do not actually trade these picks with real money. They are an experiment in whether two AI models picking independently from the same raw sentiment data outperform one — and you get to watch the experiment run live, for free, every market day. The morning pipeline is at the mercy of Reddit's API, Schwab's API, two LLM provider APIs, and my own bugs. If something breaks I will fix it. Bear with me.\n\nSincerely,\nyour favorite idiot,\nJayce",
		},
	},
	CTAText: "Open the new dashboard",
	CTAURL:  "https://vibetradez.com",
}
