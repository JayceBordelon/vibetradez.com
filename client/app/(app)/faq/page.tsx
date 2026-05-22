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
    answer: `Two Claude invocations per trading day. Pre-bell at 9:25 ET, Claudia gets trending tickers from StockTwits, Yahoo, Finviz, and SEC EDGAR plus live Schwab equity quotes and web search for overnight news. She returns exactly 3 high-conviction candidate tickers with direction (call vs put), a 1-to-10 conviction score, a written rationale, and intent (how far OTM she wants the strike, minimum days to expiration). Specific contracts are NOT chosen yet, because US options don't trade pre-market and the option chain is still showing yesterday's closing print. At 9:30:00 ET sharp the at-open agent (same model, separate run) wakes up with the 3 candidates plus live tools (Schwab quotes, live chain, account funds, web search, and a real BUY_TO_OPEN order tool). For each candidate Claudia reads the now-live chain and either fires a real order with a specific strike, expiration, and limit price, or declines with a written reason that lands inline on the dashboard. The morning email goes out at 9:25 with intent text ("~1.5% OTM, 3+ DTE"); final contracts appear on the dashboard as the at-open agent fires them.`,
  },
  {
    question: "What do the rankings (Top 1, Top 2, Top 3) mean?",
    answer: `Three picks per day, ranked 1 through 3 by Claudia's own conviction. Rank 1 is the trade she's most willing to die on a hill for; rank 3 is still a real bet she's putting real money on, just with the lowest conviction of the three. The /history page splits every metric (win rate, P&L, Sharpe, drawdown, etc.) across Top 1 / Top 2 / Top 3 so you can ask "would I have made money if I only listened when Claudia was really sure?" and find out for yourself. I'm not going to spoil the answer.`,
  },
  {
    question: "Is the P&L shown based on real trades?",
    answer: `Yes, all three. Every weekday at 9:30 ET, all 3 picks fire as real live orders against my actual Schwab brokerage account, held until 3:55 ET when the close cron exits unconditionally. No greedy fill, no duplicates, no $1k basket cap, three picks, three contracts, one per rank. The dashboard sources realized P&L directly from broker truth and tags each row with a clearly-labeled execution badge. If the LIMIT never fills at the open (rare; the 1.10× ask buffer eats most spread closure), the 9:35 cancel-dangling cron kills it and the pick is dead for the day. There's still a per-contract safety cap at $10/share so a runaway live ask can't blow up the account.`,
  },
  {
    question: "Should I trust an LLM with my retirement?",
    answer: `No. Lol no. Please no. I am begging you no. The entire premise of this site is "one model, no humans, see what happens" and that is a fun premise for a side project, not a retirement strategy. Use a target-date fund for your retirement and watch VibeTradez for entertainment.`,
  },
  {
    question: "How does the auto-execution pipeline work?",
    answer: `At 9:30:00 ET the at-open agent (Claudia, second invocation of the day) wakes up with the 3 morning candidates plus live Schwab tools and a real BUY_TO_OPEN order tool. For each candidate she walks the now-live option chain, picks a specific strike + expiration + limit price, and either fires a real order against my actual Schwab account or declines with a written reason. The order tool is locked down in code: symbol must match a morning candidate, max one order per candidate, max 3 orders per run, $10/share per-contract premium ceiling. Worst-case daily exposure is bounded at $3,000 even on a model misfire. Skipped candidates land on the dashboard as a small "Skipped at open" pill with Claudia's reasoning behind it. At 9:35 ET any LIMIT that hasn't filled gets canceled (the spread went stale, pick is dead for the day) and the operator email summarizing the day's buys + skips goes out. At 3:55 ET the close cron unconditionally sells everything still open with retry-cancel-replace if the first attempt doesn't fill. There's a kill-switch endpoint I can hit if anything looks sideways. Receipt emails go to me, not to subscribers.`,
  },
  {
    question: "Where does the market data come from?",
    answer: `Schwab Market Data API for quotes and option chains (bid, ask, mark, greeks, open interest, volume). All real prices, OAuth-gated. Sentiment and trending tickers come from StockTwits, Yahoo Finance, Finviz, and SEC EDGAR. Claudia calls all of these as actual function tools, so the prices it sees are real and not hallucinated. That said: the LLM still picks the trades, so "the data is real" is doing a lot of heavy lifting in that sentence.`,
  },
  {
    question: "How often are emails sent, and what do they contain?",
    answer: `Up to three a market day. Morning email (around 9:25 ET, before the bell): the headline pick with ticker, direction, intent (e.g. "~1.5% OTM put, 3+ DTE"), thesis, catalyst, conviction score, and Claudia's whole essay defending the call. Specific strikes aren't quoted because they get chosen at the open against live prices, not yesterday's stale chain. EOD email (right after 4:00 ET): how every pick actually performed, with entry vs closing price, stock move, per-trade P&L, and day totals. Friday: weekly digest aggregating the whole week. All free, always free, there is no premium tier I'm secretly building. You can unsubscribe whenever; I won't chase you.`,
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
