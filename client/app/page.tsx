import { ArrowRight, Clock, Eye, LogIn, Sparkles, Zap } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { AmbientBackground } from "@/components/landing/ambient-background";
import { Chapter } from "@/components/landing/chapter";
import { ChapterProgress } from "@/components/landing/chapter-progress";
import { CapList, StatBlock, StepTimeline, ToolManifest, TranscriptCard } from "@/components/landing/chapter-visuals";
import type { TranscriptLine } from "@/components/landing/chapter-visuals";
import { CountUp } from "@/components/landing/count-up";
import { LandingNavAccount } from "@/components/landing/nav-account";
import { Reveal } from "@/components/landing/reveal";
import { ScrollIndicator } from "@/components/landing/scroll-indicator";
import { SubscribeCTA } from "@/components/landing/subscribe-cta";
import { Testimonials } from "@/components/landing/testimonials";
import { TrustedBy } from "@/components/landing/trusted-by";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { fetchAccountEquity } from "@/lib/portfolio-data";

// The setup chapter's headline figure is the LIVE account equity, refreshed
// at most once a minute (ISR) so the marketing page stays fast while the
// number stays honest. Falls back to the original $5,000 deposit when the
// manager is disabled or the API is unreachable (including at build time).
export const revalidate = 60;

export const metadata: Metadata = {
  title: "VibeTradez | A Model Runs My Real Brokerage Account",
  description:
    "I gave a language model my real brokerage account. It trades stocks and options with actual money, capped in code so it cannot fully ruin me. Free to watch.",
};

// The story beats, in order. The sticky progress rail mirrors this list,
// so the source of truth for "what chapter am I in" lives here.
const CHAPTERS = [
  { id: "setup", label: "The setup" },
  { id: "loop", label: "The daily loop" },
  { id: "guardrails", label: "The guardrails" },
  { id: "tools", label: "The toolbox" },
  { id: "receipts", label: "The receipts" },
];

export default async function LandingPage() {
  const equity = await fetchAccountEquity();
  return (
    <div className="relative min-h-dvh overflow-hidden bg-background text-foreground">
      <AmbientBackground position="absolute" />

      {/* Sticky narrative progress cue, large screens only, anchors to each chapter */}
      <ChapterProgress chapters={CHAPTERS} />

      {/* ── Glass nav, overlaid on the hero at the top (does not follow scroll) ── */}
      <nav className="absolute inset-x-0 top-3 z-50 mx-auto w-[calc(100%-1.5rem)] sm:top-4 sm:max-w-5xl">
        <div className="flex items-center justify-between px-1 py-2 sm:py-2.5">
          <Link href="/" className="inline-flex min-h-11 items-center text-xl font-extrabold tracking-tight sm:min-h-9">
            <span className="text-foreground">Vibe</span>
            <span className="text-gradient-brand">Tradez</span>
          </Link>
          <div className="flex items-center gap-2">
            <ThemeToggle />
            <SubscribeCTA className="hidden h-9 items-center rounded-full border border-foreground/10 bg-foreground/5 px-3.5 text-sm font-semibold text-foreground transition-colors hover:bg-foreground/10 sm:inline-flex">
              Sign in
            </SubscribeCTA>
            <Link
              href="/dashboard"
              className="inline-flex h-9 items-center gap-1.5 rounded-full bg-foreground px-3.5 text-sm font-semibold text-background transition-opacity hover:opacity-90 sm:px-4"
            >
              Dashboard
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
            <LandingNavAccount />
          </div>
        </div>
      </nav>

      {/* ══ THE HOOK ══ Full-viewport hero, nothing else shows until you scroll ══ */}
      <section className="relative flex min-h-svh flex-col items-center justify-center px-5 py-24 sm:px-6">
        <div className="relative z-10 mx-auto w-full max-w-4xl text-center">
          <Reveal effect="fall" duration={600}>
            <span className="inline-flex items-center gap-2 text-xs font-medium text-muted-foreground sm:text-[13px]">
              <span className="relative flex h-2 w-2">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green opacity-75" />
                <span className="relative inline-flex h-2 w-2 rounded-full bg-green" />
              </span>
              Live now &middot; real money, questionable judgment
            </span>
          </Reveal>

          <Reveal effect="blur" delay={120} duration={1100} as="header">
            <h1 className="mt-7 text-[44px] font-extrabold leading-[1.05] tracking-[-0.025em] text-foreground sm:text-[64px] lg:text-[76px]">
              One silly model.
              <br />
              <span className="text-gradient-brand">Zero humans.</span>
            </h1>
          </Reveal>

          <Reveal effect="rise" delay={280} duration={900}>
            <p className="mx-auto mt-7 max-w-[640px] text-base leading-relaxed text-muted-foreground sm:text-lg">
              I gave a language model my real brokerage account. Watch it beat the S&amp;P or invent new ways to lose my money. This is fine.
            </p>
          </Reveal>

          <Reveal effect="scale" delay={420} duration={700}>
            <div className="mt-9 flex w-full flex-col items-center justify-center gap-3 sm:flex-row sm:gap-3">
              <Link
                href="/dashboard"
                className="inline-flex h-12 items-center justify-center gap-2 rounded-full bg-foreground px-7 text-[15px] font-semibold text-background shadow-lg transition-opacity hover:opacity-90"
              >
                View live dashboard
                <ArrowRight className="h-4 w-4" />
              </Link>
              <SubscribeCTA className="inline-flex h-12 items-center justify-center gap-2 rounded-full border border-foreground/10 bg-foreground/[0.03] px-7 text-[15px] font-semibold text-foreground transition-colors hover:border-foreground/25 hover:bg-foreground/[0.06]">
                <LogIn className="h-4 w-4" />
                Sign in or sign up
              </SubscribeCTA>
            </div>
          </Reveal>

          <Reveal effect="rise" delay={580} duration={700}>
            <div className="mt-12 flex items-center justify-center gap-2 text-[13px] text-muted-foreground">
              <span className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground/70">Powered by</span>
              <ClaudeLogo className="h-4 w-4" />
              <span className="font-medium">Claudia</span>
            </div>
          </Reveal>
        </div>

        <ScrollIndicator />
      </section>

      {/* ══ 01 · THE SETUP ══ what it is, the opening beat. Text-led, an oversized
           open numeral on the right. The figure column stays slim so the prose carries. ══ */}
      <Chapter
        id="setup"
        index="01"
        kicker="The setup"
        weight="text"
        title={
          <>
            One real account. <span className="text-gradient-brand">No safety net.</span>
          </>
        }
        aside={<StatBlock value={<CountUp to={Math.round(equity ?? 5000)} prefix="$" durationMs={2600} />} label={equity ? "of real money in the account right now" : "of real money, not a simulation"} sub="Unfortunately, I am writing this money off as a loss right now." />}
      >
        <p>
          It is a real brokerage account with my actual money in it, run by one model. You just watch and judge whether it has any idea what it is doing.
        </p>
      </Chapter>

      {/* ══ 02 · THE DAILY LOOP ══ the pipeline. Flipped: open timeline left, text right.
           Figure-weighted so the stepped loop gets room to breathe. No band, no card. ══ */}
      <Chapter
        id="loop"
        index="02"
        kicker="The daily loop"
        weight="even"
        flip
        title={
          <>
            Every weekday, it <span className="text-gradient-brand">starts from scratch.</span>
          </>
        }
        aside={<StepTimeline steps={pipeline} />}
      >
        <p>
          No fixed strategy. Each morning it checks the positions, the news, and the tape, then decides to buy, trim, sell, or hide in cash.
        </p>
      </Chapter>

      {/* ══ 03 · THE GUARDRAILS ══ the toolset + hard caps. Text left, cap list right. ══ */}
      <Chapter
        id="guardrails"
        index="03"
        kicker="The guardrails"
        weight="figure"
        title={
          <>
            A short leash, <span className="text-gradient-brand">written in code.</span>
          </>
        }
        aside={<CapList caps={caps} />}
      >
        <p>
          So it can be dumb, but not bankrupt-me dumb. The hard caps live in the code, not in a prompt it could sweet-talk. Every limit is checked before a trade reaches the broker, then checked again at the broker. It can lose money, just not all of it at once.
        </p>
      </Chapter>

      {/* ══ 04 · THE TOOLBOX ══ the only actions it can take. Open tool manifest
           on one side, the locked-down pitch on the other. ══ */}
      <Chapter
        id="tools"
        index="04"
        kicker="The toolbox"
        weight="even"
        title={
          <>
            It can only use the <span className="text-gradient-brand">tools I built.</span>
          </>
        }
        aside={<ToolManifest groups={toolGroups} />}
      >
        <p>
          Claude does not get free rein. It can only touch the tools I hand-coded, each one tested before it ships. No surprise powers, no rogue wire transfers, nothing off the menu.
        </p>
      </Chapter>

      {/* ══ 05 · THE RECEIPTS ══ transcripts. Flipped: open transcript readout
           left, text right. Text-weighted close to the story. No card, no band. ══ */}
      <Chapter
        id="receipts"
        index="05"
        kicker="The receipts"
        weight="even"
        flip
        accent="green"
        title={
          <>
            Every move, <span className="text-green">on the record.</span>
          </>
        }
        aside={<TranscriptCard lines={transcript} />}
      >
        <p>
          Every move is logged, tool by tool. The example beside this streams on its own as you reach it: watch the session overthink a quote, buy something anyway, and file a very confident note about why. It cannot hide a bad call from you, and it has a real talent for them.
        </p>
      </Chapter>

      {/* ══ Light parody social proof, a breather between the story and the ask. ══ */}
      <div className="relative py-20 sm:py-28 lg:py-32">
        <Testimonials />
        <TrustedBy />
      </div>

      {/* ══ THE ASK ══ final subscribe CTA, the close of the funnel ══ */}
      <section className="relative px-5 py-20 sm:px-6 sm:py-28 lg:py-32">
        <div className="relative z-10 mx-auto max-w-3xl text-center">
          <div className="lg-orb lg-orb-claude absolute h-[420px] w-[420px] -top-24 -left-24 opacity-30" aria-hidden />
          <div className="lg-orb lg-orb-cyan absolute h-[360px] w-[360px] -bottom-24 -right-20 opacity-30" aria-hidden />
          <Reveal effect="blur" duration={1000}>
            <div className="relative">
              <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">Free, forever</span>
              <h2 className="mt-3 text-3xl font-extrabold tracking-tight sm:text-4xl">
                Start getting the <span className="text-gradient-brand">recap</span>
              </h2>
              <p className="mx-auto mt-3 max-w-xl text-muted-foreground">
                One email a day so you can watch the carnage. Free, no catch. Leave whenever. <span className="italic">(I will hate you.)</span>
              </p>
              <div className="mt-8 flex flex-col items-center gap-4">
                <SubscribeCTA className="inline-flex h-12 items-center justify-center gap-2 rounded-full bg-gradient-brand px-8 text-[15px] font-semibold text-primary-foreground shadow-sm transition-opacity hover:opacity-90">
                  <LogIn className="h-4 w-4" />
                  Sign in or sign up
                </SubscribeCTA>
                <Link
                  href="/dashboard"
                  className="group inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
                >
                  or just watch the book live
                  <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
                </Link>
              </div>
            </div>
          </Reveal>
        </div>
      </section>

      {/* ── Footer ── no hard top rule, just whitespace so the surface keeps flowing ── */}
      <footer className="relative">
        <div className="mx-auto flex max-w-6xl flex-col gap-4 px-5 py-8 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div className="flex items-center gap-2">
            <span className="font-extrabold text-foreground">
              Vibe<span className="text-gradient-brand">Tradez</span>
            </span>
            <span>&copy; {new Date().getFullYear()}</span>
          </div>
          <p className="max-w-lg leading-relaxed">
            Not financial advice. Trading involves substantial risk. Everything here is one real account trading live with real money. Past performance guarantees nothing.
          </p>
          <div className="flex flex-wrap gap-4">
            <Link href="/terms" className="inline-flex min-h-11 min-w-11 items-center justify-center underline underline-offset-2 hover:text-foreground sm:min-h-0 sm:min-w-0 sm:justify-start">
              Terms
            </Link>
            <Link href="/privacy" className="inline-flex min-h-11 min-w-11 items-center justify-center underline underline-offset-2 hover:text-foreground sm:min-h-0 sm:min-w-0 sm:justify-start">
              Privacy
            </Link>
            <Link href="/faq" className="inline-flex min-h-11 min-w-11 items-center justify-center underline underline-offset-2 hover:text-foreground sm:min-h-0 sm:min-w-0 sm:justify-start">
              FAQ
            </Link>
            <a
              href="https://github.com/JayceBordelon/vibetradez.com"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex min-h-11 min-w-11 items-center justify-center underline underline-offset-2 hover:text-foreground sm:min-h-0 sm:min-w-0 sm:justify-start"
            >
              GitHub
            </a>
            <a
              href="https://jaycebordelon.com"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex min-h-11 min-w-11 items-center justify-center underline underline-offset-2 hover:text-foreground sm:min-h-0 sm:min-w-0 sm:justify-start"
            >
              Built by Jayce
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}

const toolGroups = [
  { label: "Reading", note: "look, never touch", names: ["get_portfolio", "get_stock_quotes", "get_option_chain", "get_price_history", "get_fundamentals", "get_cap_headroom", "get_recent_decisions", "get_order_status", "web_search", "web_fetch"] },
  { label: "Execution", note: "capped, real money", names: ["buy_equity", "sell_equity", "buy_option", "sell_option", "cancel_order", "hold"] },
  { label: "Documentation", note: "writes the log", names: ["write_summary"] },
];

const caps = [
  { label: "Per name, max position", num: 40 },
  { label: "Options sleeve, total", num: 50 },
  { label: "Per order, max size", num: 30 },
  { label: "Deployed per session", num: 75 },
  { label: "Drawdown breaker", num: -35 },
];


const transcript: TranscriptLine[] = [
  {
    type: "thinking",
    text: "Yesterday's note said add NVDA only if it holds the 50-day. It is holding and up on volume. Settled cash is thin, so this has to be one small add that stays under the per-name cap.",
  },
  {
    type: "text",
    text: "I read the book and the tape, then sized one small add into NVDA.",
  },
  {
    type: "tool",
    tool: "get_portfolio",
    payload: {},
    result: {
      total_equity: 41280.55,
      settled_cash: 3920.1,
      positions: 6,
      unrealized_pl: 1840.22,
      new_buy_budget: 1960.0,
    },
  },
  {
    type: "tool",
    tool: "get_stock_quotes",
    payload: { symbols: "NVDA" },
    result: {
      NVDA: { last: 141.18, bid: 141.12, ask: 141.21, mark: 141.17, volume: 38214900, day_change_pct: 2.14 },
    },
  },
  {
    type: "tool",
    tool: "buy_equity",
    open: true,
    payload: { symbol: "NVDA", quantity: 9, limit_price: 141.25 },
    result: { ok: true, action: "buy_equity", order_id: "100482731", status: "working" },
  },
  {
    type: "tool",
    tool: "get_order_status",
    payload: { order_id: "100482731" },
    result: { status: "filled", filled_quantity: 9, fill_price: 141.19 },
  },
  {
    type: "tool",
    tool: "hold",
    payload: { symbol: "MSFT" },
    result: { ok: true, action: "hold", symbol: "MSFT" },
  },
  {
    type: "tool",
    tool: "write_summary",
    payload: {
      synopsis: "Added a small NVDA position on a 50-day hold; kept MSFT.",
      action_items: ["Trim NVDA if it loses the 50-day", "Re-check options-sleeve room before adding again"],
    },
    result: { ok: true, action: "write_summary", stored: true },
  },
];

const pipeline = [
  {
    time: "Each session",
    title: "Reads the room",
    detail: "Checks what it owns, the news, and the tape. Mostly to confirm its own genius.",
    Icon: Sparkles,
  },
  {
    time: "Makes its moves",
    title: "Buys, trims, or chickens out",
    detail: "Grabs a stock or an option, cuts a loser, or hides in cash. Always within the caps.",
    Icon: Zap,
  },
  {
    time: "Across days",
    title: "Holds the line",
    detail: "Keeps what it believes in instead of panic-selling at 3:55. Brave, or stubborn.",
    Icon: Eye,
  },
  {
    time: "Every evening",
    title: "Files a report",
    detail: "One email: what it did, why, and whether it is beating the S&P or coping.",
    Icon: Clock,
  },
];
