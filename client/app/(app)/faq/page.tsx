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
    question: "Will this make me money?",
    answer: `Honestly? I dunno. Some picks have been good, some have been bad, the entire historical record is right there on /history. You tell me. But also: this is options trading. Statistically the answer is "no" for most retail traders most of the time, and an LLM picking your trades doesn't repeal statistics. Treat it like watching a poker stream, not like financial advice. If I knew it was going to print I'd be doing it from a yacht, not maintaining a docs page.`,
  },
  {
    question: "How are the daily trade picks generated?",
    answer: `Every weekday at 9:25 ET we yeet trending tickers from StockTwits, Yahoo, Finviz, and SEC EDGAR at Claude along with live Schwab quotes, full option chains, and web search as tools. Claude returns 10 ranked picks, each with a 1-10 conviction score and a written rationale defending the call. Whether any of that constitutes "analysis" or "vibes with extra steps" is a philosophical question I am not qualified to answer. The list gets saved, posted to the dashboard, and emailed before the opening bell.`,
  },
  {
    question: "What do the rankings (Top 1, Top 3, Top 5, Top 10) mean?",
    answer: `Rank 1 is the trade Claude is most willing to die on a hill for. Rank 10 is "sure, why not." The Top N filter narrows the visible picks to the highest-ranked entries, and the /history page recalculates every metric (win rate, P&L, Sharpe, drawdown, etc.) based on your selection, so you can ask "would I have made money if I only listened when Claude was really sure?" and find out for yourself. I'm not going to spoil the answer.`,
  },
  {
    question: "Is the P&L shown based on real trades?",
    answer: `Picks 2 through 10: fully hypothetical. The chart assumes you bought one contract at the morning mark price and sold at the closing mark price. Real life has bid-ask spreads, slippage, fills that never fill, and your broker's general mood, none of which are modeled here. Treat the historical P&L like a backtest some guy made on Reddit: directionally interesting, not a guarantee, please do not pay your rent with it. The rank-1 pick is different: it auto-fires as a real live order every weekday at 9:30 ET against my actual Schwab brokerage account, held until 3:55 ET when the close cron exits unconditionally. Those show up with a clearly-labeled badge so there's no ambiguity about which fills are real.`,
  },
  {
    question: "Should I trust an LLM with my retirement?",
    answer: `No. Lol no. Please no. I am begging you no. The entire premise of this site is "one model, no humans, see what happens" and that is a fun premise for a side project, not a retirement strategy. Use a target-date fund for your retirement and watch VibeTradez for entertainment.`,
  },
  {
    question: "How does the auto-execution pipeline work?",
    answer: `Every weekday at 9:30 ET the rank-1 pick fires automatically as a real live order against my actual Schwab brokerage account, with my actual money. There's a hard cap of $5 per share ($500 of capital exposure per contract); anything above that gets skipped for the day, no exceptions. At 3:55 ET (12:55 on half-trading days) the close cron unconditionally sells everything still open, with retry-cancel-replace if the first attempt doesn't fill. There's also a kill-switch endpoint I can hit if the whole thing starts looking sideways before close. Receipt emails go to me, not to subscribers, because nobody else needs to know about my fills.`,
  },
  {
    question: "Where does the market data come from?",
    answer: `Schwab Market Data API for quotes and option chains (bid, ask, mark, greeks, open interest, volume). All real prices, OAuth-gated. Sentiment and trending tickers come from StockTwits, Yahoo Finance, Finviz, and SEC EDGAR. Claude calls all of these as actual function tools, so the prices it sees are real and not hallucinated. That said: the LLM still picks the trades, so "the data is real" is doing a lot of heavy lifting in that sentence.`,
  },
  {
    question: "How often are emails sent, and what do they contain?",
    answer: `Up to three a market day. Morning email (before 9:30 ET): the headline pick with full contract spec, thesis, catalyst, conviction score, and Claude's whole essay defending the call. EOD email (after 4:05 ET): how every pick actually performed, with entry vs closing price, stock move, per-trade P&L, and day totals. Friday: weekly digest aggregating the whole week. All free, always free, there is no premium tier I'm secretly building. You can unsubscribe whenever; I won't chase you.`,
  },
  {
    question: "Is this even legal?",
    answer: `I asked. The answers I got were variations of "depends what you do with it." VibeTradez does not place trades on anyone's behalf except mine; you get an email, you get a dashboard, and what you do with that information is between you, your broker, your conscience, and ultimately your tax preparer. This is not investment advice, not financial advice, not legal advice, and not even particularly good advice. See /terms for the full lawyer-pleasing version.`,
  },
  {
    question: "How do I sign up?",
    answer: `Click Sign in in the nav bar, do the Google thing, you're in. That creates your account and subscribes you to the daily picks email in one step. Unsubscribe whenever you want; I'm not going to email you a sad cat photo asking why you left. There's nothing to upgrade to. There's nothing to pay for. There's just the email.`,
  },
];

export default function FAQPage() {
  return (
    <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
      <div className="mb-8 flex items-start gap-3">
        <div className="lg-control p-2">
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

      <Accordion type="single" collapsible className="lg-card overflow-hidden">
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
