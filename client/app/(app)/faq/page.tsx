import { HelpCircle } from "lucide-react";
import type { Metadata } from "next";

import {
	Accordion,
	AccordionContent,
	AccordionItem,
	AccordionTrigger,
} from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";

const OG_IMAGE = "/og";

export const metadata: Metadata = {
	title: "FAQ",
	description:
		"Frequently asked questions about VibeTradez, including how AI trade picks work, data sources, rankings, and performance tracking.",
	openGraph: {
		title: "VibeTradez | FAQ",
		description:
			"Frequently asked questions about VibeTradez, including how AI trade picks work, data sources, rankings, and performance tracking.",
		images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
	},
	twitter: {
		card: "summary_large_image",
		title: "VibeTradez | FAQ",
		images: [OG_IMAGE],
	},
};

const faqs = [
	{
		question: "How are the daily trade picks generated?",
		answer: `Every market day at 9:25 AM ET, the system scrapes trending tickers from Reddit communities like r/wallstreetbets and r/options, capturing sentiment scores and mention counts. The same raw sentiment payload is then sent independently to two large language models, OpenAI GPT-5.4 and Anthropic Claude Opus 4.6, running the identical analysis prompt with the same toolset (live Schwab quotes, full options chain with greeks, and built-in web search). Each model independently produces its own ranked top 10 picks for the day. Neither sees the other's output. The two pick sets are then unioned: when both models picked the same ticker it's a "consensus pick" and the row carries both scores and rationales; when only one model picked it the other side's score is left at zero. Final rank is by combined score with consensus picks tie-breaking ahead of single-model picks. Every price in every pick comes from real market data, and both models are explicitly instructed never to guess.`,
	},
	{
		question: "What does the All / OpenAI / Claude filter in the nav bar do?",
		answer: `It controls which model's picks you're looking at. "All" shows the union of both pick sets ranked by combined score. That's the default consensus view, with up to ~14 trades per day depending on how much the two models overlap. "OpenAI" filters down to only the trades GPT picked, ranked by GPT's score, exactly as if you only listened to OpenAI. "Claude" does the same for Claude. The filter applies globally, so toggling it affects the dashboard, the history page, and every API call. The Models page intentionally ignores the filter because that page exists specifically to compare both sides head to head.`,
	},
	{
		question: "What do the rankings (Top 1, Top 3, Top 5, Top 10) mean?",
		answer: `The Top N filter narrows the visible picks to only the highest-ranked entries on whichever model view you have selected. In the All view it picks from the combined ranking; in the OpenAI or Claude views it picks from that model's individual ranking. Rank 1 is the single best trade idea of the day under the active model. The historical performance page recalculates all metrics (win rate, P&L, Sharpe, expectancy, drawdown) based on your selected Top N and model filter so you can directly compare how each tier and each model has performed over time.`,
	},
	{
		question: "What does the Models page show?",
		answer: `The Models page replays the historical pick data under each model's ranking in isolation and shows a side-by-side cumulative P&L curve so you can see what you would have made by following only OpenAI, only Claude, or the combined consensus over the selected range (week / month / year / all time). It also surfaces an "agreement rate" stat, which is the fraction of dual-scored trades where the two models scored within one point of each other. This serves as a rough gauge of how often the models actually disagree on what looks like a good setup. Best and worst pick per model is shown too.`,
	},
	{
		question: "Is the P&L shown based on real trades?",
		answer: `No. VibeTradez does not execute any trades. All P&L figures are hypothetical. They assume you bought one contract of each suggested trade at the estimated market open price (the option's mark price from Schwab at 9:25 AM) and sold at the closing mark price (captured at 4:05 PM). The calculation is (closing premium − entry premium) × 100 per contract. Real-world results would differ due to bid-ask spreads, slippage, commissions, liquidity, and execution timing. These numbers are meant to track the quality of the picks over time, not to represent actual portfolio returns.`,
	},
	{
		question: "Where does the market data come from?",
		answer: `Stock quotes and option chain data (bid, ask, mark, greeks, open interest, and volume) come from the Schwab Market Data API via an authenticated OAuth connection. Sentiment data is scraped from Reddit's public JSON feeds. Both LLMs run via the official Go SDKs (openai-go and anthropic-sdk-go) with function-calling against a shared Schwab tool surface plus a built-in web search tool for catalyst and news verification. Both models share the same toolset so the comparison between them is about analytical reasoning, not about who has access to better data.`,
	},
	{
		question: "How often are emails sent, and what do they contain?",
		answer: `Subscribers receive up to three emails per market day. The morning email (before 9:30 AM ET) contains every union pick with full contract details, thesis, catalyst, sentiment, risk level, and a dual-model conviction band showing both OpenAI and Claude's scores and rationales (or just one side when only one model picked it). The end-of-day email (after 4:05 PM ET) shows how each pick performed, including entry vs closing price, stock movement, and per-trade P&L, along with an attached leaderboard noting which model's top picks made the most money that day. On Fridays the weekly digest aggregates everything across the week. All emails are free and always will be.`,
	},
];

export default function FAQPage() {
	return (
		<div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
			<div className="mb-8 flex items-start gap-3">
				<div className="rounded-md border bg-card p-2 shadow-sm">
					<HelpCircle className="h-5 w-5 text-primary" />
				</div>
				<div>
					<h1 className="text-2xl font-semibold tracking-tight">
						Frequently Asked Questions
					</h1>
					<div className="mt-1 flex items-center gap-2">
						<p className="text-sm text-muted-foreground">
							How VibeTradez works under the hood.
						</p>
						<Badge variant="secondary" className="text-[11px]">
							{faqs.length} questions
						</Badge>
					</div>
				</div>
			</div>

			<Accordion
				type="single"
				collapsible
				className="rounded-lg border bg-card shadow-sm"
			>
				{faqs.map((faq, i) => (
					<AccordionItem
						key={faq.question}
						value={`item-${i}`}
						className="border-b last:border-b-0"
					>
						<AccordionTrigger className="px-5 text-left text-base font-semibold hover:no-underline">
							{faq.question}
						</AccordionTrigger>
						<AccordionContent className="px-5 text-[15px] leading-relaxed text-muted-foreground">
							{faq.answer}
						</AccordionContent>
					</AccordionItem>
				))}
			</Accordion>
		</div>
	);
}
