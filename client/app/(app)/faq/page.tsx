import { HelpCircle } from "lucide-react";
import type { Metadata } from "next";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";

const OG_IMAGE = "/og";

export const metadata: Metadata = {
  title: "FAQ",
  description: "Frequently asked questions about VibeTradez, including how AI trade picks work, data sources, rankings, and performance tracking.",
  openGraph: {
    title: "VibeTradez | FAQ",
    description: "Frequently asked questions about VibeTradez, including how AI trade picks work, data sources, rankings, and performance tracking.",
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
    answer: `Every market day at 9:25 AM ET the system aggregates trending tickers from StockTwits, Yahoo Finance, Finviz, and SEC EDGAR. That same payload is handed to ChatGPT and Claude in parallel, each running the identical prompt with live Schwab quotes, full options chains, and web search. Each model independently produces 10 ranked picks without seeing the other's work. Once both lists are locked, each model writes a one-sentence verdict on every one of the other's trades. The two pick sets are then unioned: trades both models picked rank ahead of solo picks, ties broken by combined conviction.`,
  },
  {
    question: "What do the rankings (Top 1, Top 3, Top 5, Top 10) mean?",
    answer: `The Top N filter narrows the visible picks to only the highest-ranked entries from the combined daily list. Rank 1 is the single highest-conviction trade of the day. The historical performance page recalculates all metrics (win rate, P&L, Sharpe, expectancy, drawdown) based on your selected Top N so you can directly compare how each tier has performed over time.`,
  },
  {
    question: "What does the Models page show?",
    answer: `The Models page replays the historical pick data under each model's individual ranking in isolation and shows a side-by-side cumulative P&L curve so you can see what you would have made by following only ChatGPT, only Claude, or the combined consensus over the selected range (week, month, year, or all time). It also surfaces an "agreement rate" stat, the fraction of trades where both models picked the same ticker. Best and worst pick per model is shown too, along with each model's verdict on the other's calls so you can see who was right when they disagreed.`,
  },
  {
    question: "Is the P&L shown based on real trades?",
    answer: `No. VibeTradez does not execute any trades. All P&L figures are hypothetical. They assume you bought one contract of each suggested trade at the estimated market open price (the option's mark price from Schwab at 9:25 AM) and sold at the closing mark price (captured at 4:05 PM). The calculation is (closing premium minus entry premium) times 100 per contract. Real-world results would differ due to bid-ask spreads, slippage, commissions, liquidity, and execution timing. These numbers are meant to track the quality of the picks over time, not to represent actual portfolio returns.`,
  },
  {
    question: "Where does the market data come from?",
    answer: `Stock quotes and option chain data (bid, ask, mark, greeks, open interest, volume) come from the Schwab Market Data API via OAuth. Market signals are aggregated from StockTwits, Yahoo Finance, Finviz, and SEC EDGAR. Both models call into the same Schwab + web search tool surface via function-calling so the only variable between them is reasoning.`,
  },
  {
    question: "How often are emails sent, and what do they contain?",
    answer: `Subscribers receive up to three emails per market day. The morning email (before 9:30 AM ET) contains every union pick with full contract details, thesis, catalyst, sentiment, risk level, both models' conviction scores and rationales, and the cross-examination verdict each model wrote on the other's pick. The end-of-day email (after 4:05 PM ET) shows how each pick performed: entry vs closing price, stock movement, per-trade P&L, and a leaderboard noting which model's top picks made the most money that day. On Fridays the weekly digest aggregates everything across the week. All emails are free and always will be.`,
  },
  {
    question: "How do I sign up?",
    answer: `Click Sign in in the nav bar and continue with Google. That creates your account and signs you up for the daily picks email in one step. You can sign out or unsubscribe at any time.`,
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
          <h1 className="text-2xl font-semibold tracking-tight">Frequently Asked Questions</h1>
          <div className="mt-1 flex items-center gap-2">
            <p className="text-sm text-muted-foreground">How VibeTradez works under the hood.</p>
            <Badge variant="secondary" className="text-[11px]">
              {faqs.length} questions
            </Badge>
          </div>
        </div>
      </div>

      <Accordion type="single" collapsible className="rounded-lg border bg-card shadow-sm">
        {faqs.map((faq, i) => (
          <AccordionItem key={faq.question} value={`item-${i}`} className="border-b last:border-b-0">
            <AccordionTrigger className="px-5 text-left text-base font-semibold hover:no-underline">{faq.question}</AccordionTrigger>
            <AccordionContent className="px-5 text-[15px] leading-relaxed text-muted-foreground">{faq.answer}</AccordionContent>
          </AccordionItem>
        ))}
      </Accordion>
    </div>
  );
}
