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
	"html/template"
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
//
// Section bodies are template.HTML so callers can include inline
// <strong>, <em>, <a href="...">, and <ul><li>... markup. The
// admin/announce endpoint is gated behind X-Admin-Key, so the
// input is trusted.
var launchAnnouncement = templates.AnnouncementData{
	Subject:      "VibeTradez relaunch: dual-model trade picker, head-to-head model comparison, and a new domain",
	Badge:        "Rebrand + Relaunch",
	Headline:     "jaycetrades is now vibetradez.com",
	HeroImageURL: "https://media.licdn.com/dms/image/v2/D5622AQEfCEI8dXczVw/feedshare-shrink_800/feedshare-shrink_800/0/1719904786508?e=2147483647&v=beta&t=jmqaTzPrRC_dl6YvHOPCcqfNGy3YesDYf2PmV1ZqpH8",
	Sections: []templates.AnnouncementSection{
		{
			Title: "TL;DR",
			Body: template.HTML(`<ul style="margin: 0; padding-left: 20px;">
				<li style="margin-bottom: 6px;"><strong>New name and new home.</strong> JayceTrades is now <strong>VibeTradez</strong> at <strong>vibetradez.com</strong>.</li>
				<li style="margin-bottom: 6px;"><strong>Two AIs, not one.</strong> Both OpenAI and Anthropic Claude now pick your trades independently every morning.</li>
				<li style="margin-bottom: 6px;"><strong>Pick a side.</strong> A new filter in the nav lets you see only OpenAI's picks, only Claude's picks, or both.</li>
				<li style="margin-bottom: 6px;"><strong>Head-to-head page.</strong> A brand new <strong>/models</strong> page shows which AI would have made you more money.</li>
				<li style="margin-bottom: 6px;"><strong>Same email, same cadence, same free price.</strong> Your subscription carried over automatically.</li>
				<li style="margin-bottom: 0;"><strong>Open source.</strong> The whole stack is on GitHub if you want to read the code.</li>
			</ul>`),
		},
		{
			Title: "What changed under the hood",
			Body: template.HTML(`The biggest difference: instead of one AI generating your daily picks, <strong>two different AI models now run the exact same workflow independently</strong> every morning at 9:25 AM ET. <strong>OpenAI GPT-5.4</strong> and <strong>Anthropic Claude Opus 4.6</strong> each get the same Reddit sentiment, the same live Schwab market data, the same options chain, the same web search for catalysts — and each one picks its own ranked top 10 trades for the day.<br><br>
			Neither AI sees what the other one picked. When both models happen to pick the same ticker, we call it a <strong>consensus pick</strong> and bubble it to the top of your dashboard. When only one model picked a ticker, you still see it in the list, tagged with whichever AI made the call.`),
		},
		{
			Title: "Filter the dashboard by which AI picked it",
			Body:  template.HTML(`In the nav bar at the top of every page you'll see a new <strong>All / OpenAI / Claude</strong> toggle. Click <strong>OpenAI</strong> to see exactly what GPT picked today. Click <strong>Claude</strong> to see what Claude picked. Click <strong>All</strong> for the merged consensus view. The filter applies to the live dashboard <em>and</em> the historical analytics page, and it remembers your choice across page reloads.`),
		},
		{
			Title: "The new head-to-head page",
			Body: template.HTML(`<a href="https://vibetradez.com/models" style="color: #2563eb; font-weight: 700;">vibetradez.com/models</a> is the new headline feature. Side-by-side OpenAI vs Anthropic backtest with:<br>
			<ul style="margin: 8px 0 0 0; padding-left: 20px;">
				<li><strong>Cumulative P&amp;L curve</strong> for each model</li>
				<li><strong>Win rate, average return, best and worst pick</strong> per side</li>
				<li><strong>Agreement rate</strong> — how often the two models scored within 1 of each other</li>
				<li><strong>Configurable date range</strong> — week, month, year, or all time</li>
			</ul>
			<br>Find out which model would actually have made you more money if you'd only listened to one of them.`),
		},
		{
			Title: "Everywhere else got polish too",
			Body: template.HTML(`<ul style="margin: 0; padding-left: 20px;">
				<li style="margin-bottom: 4px;">Every pick card now shows <strong>both models' scores and rationales</strong> with their brand colors and icons.</li>
				<li style="margin-bottom: 4px;">The history page <strong>equity curve overlays all four Top-N strategies</strong> (Top 1 / 3 / 5 / 10) so you can compare them at a glance.</li>
				<li style="margin-bottom: 4px;">Win rate, agreement rate, and other percentage stats now <strong>shade red → green continuously</strong> instead of snapping to three buckets.</li>
				<li style="margin-bottom: 4px;">The morning email now carries the <strong>dual-model conviction band on every pick</strong>.</li>
				<li style="margin-bottom: 4px;">Mobile layout, animations, and overflow handling were <strong>rebuilt from scratch</strong>.</li>
				<li style="margin-bottom: 0;">A few crashes on the history page and an empty-state bug on the dashboard are <strong>fixed</strong>.</li>
			</ul>`),
		},
		{
			Title: "Read the fine print",
			Body:  template.HTML(`The <strong>FAQ</strong> and <strong>methodology</strong> sections (linked from the footer of every page) are fully rewritten to explain how the two-model architecture works, what consensus picks mean, and how the comparison page math is computed. The Terms of Service has been updated to reflect both LLM providers and the new domain.`),
		},
		{
			Title: "Built in the open",
			Body:  template.HTML(`Everything that runs VibeTradez — the Go API, the Next.js dashboard, the dual-model picker, the Schwab integration, the email templates, the CI/CD pipeline, and the docker compose stack that runs it all — lives in <strong>one public monorepo on GitHub</strong>. Read the code, file an issue, or fork it for your own experiments.`),
		},
		{
			Title: "One thing to remember",
			Body: template.HTML(`<strong>This is still hypothetical P&amp;L.</strong> I do not actually trade these picks with real money. They are an experiment in whether two AI models picking independently from the same raw sentiment data can outperform one — and you get to watch the experiment run live, every market day, for free.<br><br>
			The morning pipeline is at the mercy of Reddit's API, Schwab's API, two LLM provider APIs, and my own bugs. If something breaks I will fix it. Bear with me.<br><br>
			Sincerely,<br>
			your favorite idiot,<br>
			<strong>Jayce</strong>`),
		},
	},
	CTAs: []templates.AnnouncementCTA{
		{Text: "Open the dashboard", URL: "https://vibetradez.com", Style: "primary"},
		{Text: "Compare the models", URL: "https://vibetradez.com/models", Style: "secondary"},
		{Text: "View on GitHub", URL: "https://github.com/JayceBordelon/personal-monorepo", Style: "secondary"},
	},
}
