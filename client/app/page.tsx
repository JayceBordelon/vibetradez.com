import { ArrowRight, LogIn } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";

import { LiveTranscript, type TranscriptLine } from "@/components/landing/live-transcript";
import { LandingNavAccount } from "@/components/landing/nav-account";
import { Reveal } from "@/components/landing/reveal";
import { ScrollIndicator } from "@/components/landing/scroll-indicator";
import { SubscribeCTA } from "@/components/landing/subscribe-cta";
import { LogSection } from "@/components/landing/terminal/log-section";
import { SessionClock } from "@/components/landing/terminal/session-clock";
import { CronTable, EquityReadout, GuardsReadout, HeroStatStrip, TickerTape, ToolLs } from "@/components/landing/terminal/terminal-visuals";
import { TypeLines } from "@/components/landing/terminal/type-lines";
import { Testimonials } from "@/components/landing/testimonials";
import { TrustedBy } from "@/components/landing/trusted-by";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { fetchAccountEquity } from "@/lib/portfolio-data";

/*
The landing page is the front door of the CRT trading terminal: dark
mode is the tube (phosphor glow, scanlines), light mode is the printout
of the same session on tractor-feed paper. The whole story reads as one
scrollback buffer: shell prompts open each beat and the supporting
visuals are command output. The palette and chrome are site-wide now
(globals.css tokens + overlays in the root layout).
*/

// The hero's headline figure is the LIVE account equity, refreshed at
// most once a minute (ISR) so the marketing page stays fast while the
// number stays honest. Falls back to the original $5,000 deposit when the
// manager is disabled or the API is unreachable (including at build time).
export const revalidate = 60;

export const metadata: Metadata = {
  title: "VibeTradez | A Model Runs My Real Brokerage Account",
  description:
    "I gave a language model my real brokerage account. It trades stocks and options with actual money, capped in code so it cannot fully ruin me. Free to watch.",
};

// Session stamp for the status bar, in market time. ISR re-renders keep
// it within a minute of true, which is plenty for a date.
function sessionStamp(): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone: "America/New_York" }).format(new Date());
}

export default async function LandingPage() {
  const equity = await fetchAccountEquity();
  return (
    <div className="relative min-h-dvh overflow-hidden">
      {/* ── Status bar: the terminal's title strip. Sticky so the session
            chrome never leaves the screen. ── */}
      <nav className="sticky top-0 z-50 border-b border-dashed border-border bg-background/90 backdrop-blur-sm">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-3 px-5 py-2.5 sm:px-6">
          <div className="flex min-w-0 items-center gap-3 font-mono">
            <Link href="/" className="inline-flex min-h-9 items-center text-[15px] font-extrabold tracking-tight">
              <span className="text-foreground">VIBE</span>
              <span className="phosphor text-green">TRADEZ</span>
            </Link>
            <span className="hidden text-[11px] text-muted-foreground md:inline">// session {sessionStamp()}</span>
          </div>
          <div className="flex items-center gap-2 font-mono">
            <span className="mr-1 hidden items-center gap-2 text-[11px] text-muted-foreground sm:inline-flex">
              <span className="relative flex h-2 w-2" aria-hidden>
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-red opacity-70" />
                <span className="relative inline-flex h-2 w-2 rounded-full bg-red" />
              </span>
              REC
              <SessionClock />
            </span>
            <ThemeToggle />
            <SubscribeCTA className="hidden h-9 items-center border border-border px-3 text-[12px] font-bold uppercase tracking-[0.08em] text-foreground transition-colors hover:border-green hover:text-green sm:inline-flex">
              Sign in
            </SubscribeCTA>
            <Link href="/dashboard" className="inline-flex h-9 items-center gap-1.5 bg-foreground px-3 text-[12px] font-bold uppercase tracking-[0.08em] text-background transition-opacity hover:opacity-85">
              Dashboard
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
            <LandingNavAccount />
          </div>
        </div>
      </nav>

      {/* ══ THE BOOT ══ Full-viewport hero. The headline types itself in,
           the fact strip prints below like a quote row. ══ */}
      <section className="relative flex min-h-svh flex-col justify-center px-5 pb-24 pt-16 sm:px-6">
        <div className="mx-auto w-full max-w-5xl">
          <Reveal effect="fade" duration={600}>
            <p className="flex flex-wrap items-center gap-2 font-mono text-[12px] text-muted-foreground sm:text-[13px]">
              <span className="phosphor font-bold text-green" aria-hidden>
                $
              </span>
              <span className="font-semibold text-foreground">./vibetradez --live --real-money</span>
              <span className="text-muted-foreground/70">&middot; questionable judgment included</span>
            </p>
          </Reveal>

          {/* Martian Mono advances ~0.72em per character and the longest
              typed line is 18 chars plus the caret, so the size must stay
              under ~6.5vw or the nowrap lines clip on small screens. */}
          <h1 className="term-display phosphor mt-8 text-[clamp(24px,6.4vw,96px)] font-extrabold uppercase leading-[1.04] tracking-tight text-foreground">
            <TypeLines lines={["> One silly model.", "> Zero humans."]} />
          </h1>

          <Reveal effect="rise" delay={2400} duration={900}>
            <p className="mt-8 max-w-[640px] text-base leading-relaxed text-muted-foreground sm:text-lg">
              I gave a language model my real brokerage account. Watch it beat the S&amp;P or invent new ways to lose my money. This is fine.
            </p>
          </Reveal>

          <Reveal effect="rise" delay={2700} duration={700}>
            <div className="mt-9 flex w-full flex-col gap-3 sm:flex-row">
              <Link href="/dashboard" className="term-btn term-btn-solid">
                Attach to session
                <ArrowRight className="h-4 w-4" />
              </Link>
              <SubscribeCTA className="term-btn term-btn-ghost">
                <LogIn className="h-4 w-4" />
                Subscribe --daily-recap
              </SubscribeCTA>
            </div>
          </Reveal>

          <Reveal effect="fade" delay={3000} duration={700}>
            <div className="mt-12 flex items-center gap-2 font-mono text-[12px] text-muted-foreground">
              <span className="text-[10px] uppercase tracking-[0.16em] text-muted-foreground/70">runtime</span>
              <ClaudeLogo className="h-4 w-4" />
              <span className="font-semibold">Claudia</span>
            </div>
          </Reveal>

          <Reveal effect="fade" delay={3200} duration={800}>
            <div className="mt-10">
              <HeroStatStrip equity={equity} />
            </div>
          </Reveal>
        </div>

        <ScrollIndicator />
      </section>

      {/* Tape seam between the boot screen and the scrollback. */}
      <TickerTape />

      {/* ══ 01 · THE SETUP ══ */}
      <LogSection id="setup" command="cat 01_setup.txt" comment="what this is" weight="text" title={<>One real account. <span className="phosphor text-green">No safety net.</span></>} aside={<EquityReadout equity={equity} />}>
        <p>A real brokerage account with my actual money in it, run by one model with unlimited confidence and zero dollars of its own at stake. You just watch and judge whether it has any idea what it is doing.</p>
      </LogSection>

      {/* ══ 02 · THE DAILY LOOP ══ */}
      <LogSection id="loop" command="crontab -l" comment="the whole schedule" flip title={<>Every weekday, it <span className="phosphor text-green">starts from scratch.</span></>} aside={<CronTable />}>
        <p>No fixed strategy. Three times a day — at the open, midday, and before the close — it checks the positions, the news and retail hype, and the tape, then decides to buy, trim, sell, or hide in cash and call that discipline. It holds what it believes in across days, which is either conviction or stubbornness depending on next week.</p>
      </LogSection>

      {/* ══ 03 · THE GUARDRAILS ══ */}
      <LogSection id="guardrails" command="cat guards.go" comment="what little governs the money" weight="figure" title={<>The leash came off. <span className="phosphor text-green">One rule.</span></>} aside={<GuardsReadout />}>
        <p>Any name, any size, any mix of stocks and options, all in if it likes the setup. There is no split it has to hold and no cap on a single name. The only rule in the file is the broker's: it spends cash that has settled, nothing more. It can absolutely lose my money. That is the show.</p>
      </LogSection>

      {/* ══ 04 · THE TOOLBOX ══ */}
      <LogSection id="tools" command="ls -l tools/" comment="the only actions that exist" flip title={<>It can only use the <span className="phosphor text-green">tools I built.</span></>} aside={<ToolLs />}>
        <p>Claudia gets the tools I hand-built and nothing else. No surprise powers, no rogue wire transfers, no ordering GPUs on my card. If a button does not exist, she cannot press it.</p>
      </LogSection>

      {/* ══ 05 · THE RECEIPTS ══ */}
      <LogSection id="receipts" command="tail -f sessions/latest.log" comment="streams live as you reach it" title={<>Every move, <span className="phosphor text-green">on the record.</span></>} aside={<LiveTranscript lines={transcript} />}>
        <p>Every move is logged, tool by tool. The session beside this streams on its own: watch it overthink a quote, buy something anyway, and file a very confident note about why. It cannot hide a bad call from you, and it has a real talent for them.</p>
      </LogSection>

      {/* ── Parody social proof, a breather between the log and the ask. ── */}
      <div className="relative pt-16 pb-8 sm:pt-20 sm:pb-10">
        <Testimonials />
        <TrustedBy />
      </div>

      {/* ══ THE ASK ══ final subscribe block, framed like one last command. ══ */}
      <section className="relative px-5 pt-10 pb-20 sm:px-6 sm:pt-12 sm:pb-28">
        <div className="mx-auto max-w-3xl">
          <Reveal effect="rise" duration={900}>
            <div className="border border-dashed border-border px-6 py-10 text-center sm:px-10 sm:py-12">
              <p className="font-mono text-[12px] text-muted-foreground">
                <span className="phosphor font-bold text-green" aria-hidden>
                  $
                </span>{" "}
                <span className="font-semibold text-foreground">vt subscribe --daily --free</span>
              </p>
              <h2 className="term-display mt-5 text-2xl font-extrabold uppercase tracking-tight sm:text-3xl">
                Start getting <span className="phosphor text-green">the recap</span>
              </h2>
              <p className="mx-auto mt-4 max-w-xl text-muted-foreground">
                One email a day so you can watch the carnage. Free, no catch. Leave whenever. <span className="italic">(I will hate you.)</span>
              </p>
              <div className="mt-8 flex flex-col items-center gap-4">
                <SubscribeCTA className="term-btn term-btn-solid">
                  <LogIn className="h-4 w-4" />
                  Sign in or sign up
                </SubscribeCTA>
                <Link href="/dashboard" className="group inline-flex items-center gap-1.5 font-mono text-sm font-medium text-muted-foreground transition-colors hover:text-foreground">
                  or just watch the book live
                  <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
                </Link>
              </div>
            </div>
          </Reveal>
        </div>
      </section>

      {/* ── Footer: the session signs off. ── */}
      <footer className="relative border-t border-dashed border-border">
        <div className="mx-auto flex max-w-6xl flex-col gap-4 px-5 py-8 font-mono text-xs text-muted-foreground sm:px-6">
          <p aria-hidden>
            <span className="phosphor font-bold text-green">$</span> <span className="font-semibold text-foreground">exit 0</span> <span className="text-muted-foreground/70">&middot; the account survives another day</span>
          </p>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex items-center gap-2">
              <span className="font-extrabold text-foreground">
                VIBE<span className="phosphor text-green">TRADEZ</span>
              </span>
              <span>&copy; {new Date().getFullYear()}</span>
            </div>
            <p className="max-w-lg leading-relaxed">Not financial advice. Trading involves substantial risk. Everything here is one real account trading live with real money. Past performance guarantees nothing.</p>
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
              <a href="https://github.com/JayceBordelon/vibetradez.com" target="_blank" rel="noopener noreferrer" className="inline-flex min-h-11 min-w-11 items-center justify-center underline underline-offset-2 hover:text-foreground sm:min-h-0 sm:min-w-0 sm:justify-start">
                GitHub
              </a>
              <a href="https://jaycebordelon.com" target="_blank" rel="noopener noreferrer" className="inline-flex min-h-11 min-w-11 items-center justify-center underline underline-offset-2 hover:text-foreground sm:min-h-0 sm:min-w-0 sm:justify-start">
                Built by Jayce
              </a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}

const transcript: TranscriptLine[] = [
  {
    type: "thinking",
    text: "Yesterday's note said add NVDA only if it holds the 50-day. It is holding and up on volume. Settled cash is thin, so this is one small add sized to the cash on hand.",
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
      action_items: ["Trim NVDA if it loses the 50-day", "Confirm the NVDA fill settled before redeploying"],
    },
    result: { ok: true, action: "write_summary", stored: true },
  },
];
