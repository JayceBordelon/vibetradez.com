import { HelpCircle } from "lucide-react";
import type { Metadata } from "next";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";

const OG_IMAGE = "/opengraph-image";

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
    answer: `Every market day at 9:25 AM ET the system aggregates trending tickers from StockTwits, Yahoo Finance, Finviz, and SEC EDGAR. That payload is handed to Claude with live Schwab quotes, full options chains, and web search as tools. Claude produces 10 ranked picks, each with a 1-10 conviction score and a written rationale defending the score. The list is saved to the database, surfaced on the dashboard, and emailed to subscribers before the opening bell.`,
  },
  {
    question: "What do the rankings (Top 1, Top 3, Top 5, Top 10) mean?",
    answer: `The Top N filter narrows the visible picks to only the highest-ranked entries from the daily list. Rank 1 is the single highest-conviction trade of the day. The historical performance page recalculates all metrics (win rate, P&L, Sharpe, expectancy, drawdown) based on your selected Top N so you can directly compare how each tier has performed over time.`,
  },
  {
    question: "Is the P&L shown based on real trades?",
    answer: `Mostly hypothetical for the picks list, real for the rank-1 paper trade. The historical P&L for picks #2 through #10 assumes you bought one contract at the estimated market open price (the option's mark price from Schwab at 9:25 AM) and sold at the closing mark price (captured at 4:05 PM). The calculation is (closing premium minus entry premium) times 100 per contract. Real-world results would differ due to bid-ask spreads, slippage, commissions, liquidity, and execution timing. The rank-1 pick is different: every weekday at 9:30 AM ET the system auto-fires a paper trade on it (or a live trade if TRADING_MODE=live is configured), held until 3:55 PM ET when the close cron exits the position unconditionally. Those positions surface on the dashboard with a clearly-labeled PAPER or LIVE badge.`,
  },
  {
    question: "How does the auto-execution pipeline work?",
    answer: `Every weekday morning at 9:30 ET, the rank-1 pick of the day fires automatically. There's no email confirmation step, no five-minute window, just a paper order placed at the live Schwab option mark. If TRADING_MODE=live is configured, real orders go to the Schwab Trader API instead. The hard cap is $5/share (= $500 capital exposure per contract); anything above gets skipped for the day. At 3:55 PM ET (or 12:55 PM on half-trading days) the close cron unconditionally sells everything still open, with a retry-cancel-replace fallback if the first close attempt doesn't fill. Receipt emails fire after every fill so you have a paper trail. There's a kill-switch endpoint (POST /api/execution/cancel-all, auth-gated) if you ever need to bail before 3:55 PM.`,
  },
  {
    question: "Where does the market data come from?",
    answer: `Stock quotes and option chain data (bid, ask, mark, greeks, open interest, volume) come from the Schwab Market Data API via OAuth. Market signals are aggregated from StockTwits, Yahoo Finance, Finviz, and SEC EDGAR. Claude calls into the Schwab + web search tool surface via function-calling so prices are real, not hallucinated.`,
  },
  {
    question: "How often are emails sent, and what do they contain?",
    answer: `Subscribers receive up to three emails per market day. The morning email (before 9:30 AM ET) contains the headline pick with full contract details, thesis, catalyst, sentiment, risk level, conviction score, and Claude's written rationale defending the call. The end-of-day email (after 4:05 PM ET) shows how each pick performed: entry vs closing price, stock movement, per-trade P&L, and the day's totals. On Fridays the weekly digest aggregates everything across the week. All emails are free and always will be. Auto-execution receipts (open fill, close fill, close-failed alerts) go only to the operator, not to subscribers.`,
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
