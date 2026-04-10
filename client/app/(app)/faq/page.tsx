import { HelpCircle } from "lucide-react";
import type { Metadata } from "next";

import {
	Accordion,
	AccordionContent,
	AccordionItem,
	AccordionTrigger,
} from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";

export const metadata: Metadata = {
	title: "FAQ",
	description:
		"Frequently asked questions about VibeTradez — how AI trade picks work, data sources, rankings, and performance tracking.",
};

const faqs = [
	{
		question: "How are the daily trade picks generated?",
		answer: `Every market day at 9:25 AM ET, our system scrapes trending tickers from Reddit communities like r/wallstreetbets and r/options, capturing sentiment scores and mention counts. This data — along with real-time stock quotes and live option chain pricing from the Schwab Market Data API — is fed into OpenAI's GPT-5.4 model. GPT-5.4 has proven particularly strong at sentiment analysis and market context synthesis. The model identifies 10 high-conviction short-dated options trades (0–7 DTE, under $200 per contract), ranks them 1–10 by conviction, and returns specific contract details including strike price, expiration, entry price, target, and stop loss. Every price in the pick is sourced from real market data — nothing is guessed. We are planning a migration to Anthropic's Claude for the analysis layer in the near future.`,
	},
	{
		question: "What do the rankings (Top 1, Top 3, Top 5, Top 10) mean?",
		answer: `Each morning, the AI ranks all 10 picks from 1 (highest conviction) to 10 (lowest conviction). Rank 1 is the single best trade idea of the day. The Top N filter lets you narrow the dashboard and historical analytics to only the highest-ranked picks. If you have limited capital or prefer fewer, higher-quality positions, filter to Top 1 or Top 3. If you want broader exposure, view all 10. The historical performance page recalculates all metrics (win rate, P&L, Sharpe ratio, etc.) based on your selected filter, so you can compare how different tiers have performed over time.`,
	},
	{
		question: "Is the P&L shown based on real trades?",
		answer: `No. VibeTradez does not execute any trades. All P&L figures are hypothetical. They assume you bought one contract of each suggested trade at the estimated market open price (the option's mark price from Schwab at 9:25 AM) and sold at the closing mark price (captured at 4:05 PM). The calculation is (closing premium − entry premium) × 100 per contract. Real-world results would differ due to bid-ask spreads, slippage, commissions, liquidity, and execution timing. These numbers are meant to track the quality of the AI's picks over time, not to represent actual portfolio returns.`,
	},
	{
		question: "Where does the market data come from?",
		answer: `Stock quotes and option chain data (bid, ask, mark, greeks, open interest) come from the Schwab Market Data API via an authenticated OAuth connection. Sentiment data is scraped from Reddit's public JSON feeds. The AI analysis is currently powered by OpenAI's GPT-5.4 model using the Responses API with function calling — the model can query Schwab for real-time quotes and option chains, and use web search for news and catalyst context, all within a single analysis session. GPT-5.4 has been the backbone of sentiment analysis here and has delivered solid results, but we will be migrating to Anthropic's Claude in an upcoming release for enhanced reasoning and trade analysis capabilities.`,
	},
	{
		question: "How often are emails sent, and what do they contain?",
		answer: `Subscribers receive up to three emails per week-day. The morning email (before 9:30 AM ET) contains all 10 ranked picks with contract details, thesis, sentiment, catalyst, and risk level. The end-of-day email (after 4:05 PM ET) shows how each pick performed — entry vs. closing price, stock movement, and per-trade P&L. On Fridays, a weekly summary email aggregates the full week's performance with win rate, total P&L, best/worst trades, and a hypothetical buy-at-open/sell-at-close analysis. All emails are free and always will be.`,
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
