import { ArrowRight, LineChart, Lock, Mail } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";

import { LiveTranscript, type TranscriptLine } from "@/components/landing/live-transcript";
import { LandingNavAccount } from "@/components/landing/nav-account";
import { Reveal } from "@/components/landing/reveal";
import { ScrollIndicator } from "@/components/landing/scroll-indicator";
import { SubscribeCTA } from "@/components/landing/subscribe-cta";
import { LogSection } from "@/components/landing/terminal/log-section";
import { CronTable, EquityReadout, GuardsReadout, HeroStatStrip, ToolLs } from "@/components/landing/terminal/terminal-visuals";
import { Testimonials } from "@/components/landing/testimonials";
import { TrustedBy } from "@/components/landing/trusted-by";
import { Footer } from "@/components/layout/footer";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { Wordmark } from "@/components/layout/wordmark";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { fetchAccountEquity } from "@/lib/portfolio-data";

/*
The marketing landing in the soft-minimalist-light system: a calm hero
that leads with the live account number, then clean alternating feature
sections that tell the story (the setup, the daily loop, the guardrails,
the toolbox, the live receipts), parody social proof, and the subscribe
ask. Same design language as the in-app dashboard — one cohesive product.
*/

// The hero's headline figure is the LIVE account equity, refreshed at most
// once a minute (ISR). Falls back to the original $5,000 deposit when the
// manager is disabled or the API is unreachable (including at build time).
export const revalidate = 60;

export const metadata: Metadata = {
  title: "VibeTradez | A Model Runs My Real Brokerage Account",
  description:
    "I gave a language model my real brokerage account. It buys options with actual money — calls and puts, no stocks — sized in code so it can't faceplant the whole account on one contract. Free to watch.",
};

export default async function LandingPage() {
  const equity = await fetchAccountEquity();
  return (
    <div className="relative min-h-dvh overflow-hidden">
      {/* Soft ambient wash behind the hero. */}
      <div className="pointer-events-none absolute inset-x-0 top-0 h-[760px] overflow-hidden" aria-hidden>
        <div className="lg-mesh" />
      </div>

      {/* ── Nav ── */}
      <nav className="sticky top-0 z-50 border-b border-border/70 bg-background/80 backdrop-blur-xl">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-3 px-5 py-3 sm:px-6">
          <Link href="/" aria-label="VibeTradez home">
            <Wordmark />
          </Link>
          <div className="flex items-center gap-2">
            <ThemeToggle />
            <SubscribeCTA className="hidden h-9 items-center rounded-full border border-border px-4 text-[13px] font-medium text-foreground transition-colors hover:bg-muted sm:inline-flex">
              Sign in
            </SubscribeCTA>
            <Link href="/dashboard" className="inline-flex h-9 items-center gap-1.5 rounded-full bg-primary px-4 text-[13px] font-medium text-primary-foreground shadow-sm transition-all hover:shadow-md">
              Dashboard
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
            <LandingNavAccount />
          </div>
        </div>
      </nav>

      {/* ── Hero ── */}
      <section className="relative px-5 pb-20 pt-14 sm:px-6 sm:pt-20">
        <div className="mx-auto grid max-w-6xl items-center gap-12 lg:grid-cols-[1.04fr_0.96fr] lg:gap-16">
          <div>
            <Reveal effect="fade" duration={600}>
              <span className="inline-flex items-center gap-2 rounded-full border border-border bg-card px-3 py-1 text-xs font-medium text-muted-foreground shadow-xs">
                <span className="relative flex h-2 w-2" aria-hidden>
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green opacity-60" />
                  <span className="relative inline-flex h-2 w-2 rounded-full bg-green" />
                </span>
                Autonomous AI portfolio manager · Live
              </span>
            </Reveal>

            <Reveal effect="rise" delay={80} duration={700}>
              <h1 className="mt-6 font-display text-[clamp(34px,6vw,60px)] font-bold leading-[1.05] tracking-tight text-foreground">
                A language model runs a <span className="text-primary">real brokerage account.</span>
              </h1>
            </Reveal>

            <Reveal effect="rise" delay={160} duration={700}>
              <p className="mt-6 max-w-xl text-base leading-relaxed text-muted-foreground sm:text-lg">
                Every weekday, Claudia reads the book, the news, and the tape, then buys options with actual money — calls and puts, no stocks, sized in code so it can&apos;t faceplant the account on one trade. Watch it beat the S&amp;P or invent new ways to lose my money.
              </p>
            </Reveal>

            <Reveal effect="rise" delay={240} duration={700}>
              <div className="mt-8 flex flex-col gap-3 sm:flex-row">
                <Link href="/dashboard" className="term-btn term-btn-solid">
                  View the live dashboard
                  <ArrowRight className="h-4 w-4" />
                </Link>
                <SubscribeCTA className="term-btn term-btn-ghost">
                  <Mail className="h-4 w-4" />
                  Get the daily recap
                </SubscribeCTA>
              </div>
            </Reveal>

            <Reveal effect="fade" delay={320} duration={700}>
              <div className="mt-8 flex items-center gap-2 text-sm text-muted-foreground">
                <span className="text-xs uppercase tracking-[0.12em] text-muted-foreground/70">Run by</span>
                <ClaudeLogo className="h-4 w-4" />
                <span className="font-medium text-foreground">Claudia</span>
                <span className="text-border" aria-hidden>
                  ·
                </span>
                <span>free to watch</span>
              </div>
            </Reveal>
          </div>

          <Reveal effect="scale" delay={120} duration={800}>
            <EquityReadout equity={equity} />
          </Reveal>
        </div>

        <ScrollIndicator />
      </section>

      {/* ── 01 · The setup ── */}
      <LogSection id="setup" comment="What this is" weight="text" title={<>One real account. <span className="text-primary">No safety net.</span></>} aside={<HeroStatStrip equity={equity} />}>
        <p>A real brokerage account with my actual money in it, run by one model with unlimited confidence and zero dollars of its own at stake. You just watch and judge whether it has any idea what it&apos;s doing.</p>
      </LogSection>

      {/* ── 02 · The daily loop ── */}
      <LogSection id="loop" comment="The whole schedule" flip title={<>Every weekday, it <span className="text-primary">starts from scratch.</span></>} aside={<CronTable />}>
        <p>No fixed strategy. Three times a day — at the open, midday, and before the close — it checks the positions, the news and retail hype, and the tape, then decides to buy, trim, sell, or hide in cash and call that discipline. It holds what it believes in across days, which is either conviction or stubbornness depending on next week.</p>
      </LogSection>

      {/* ── 03 · The guardrails ── */}
      <LogSection id="guardrails" comment="What governs the money" weight="figure" title={<>Options only. <span className="text-primary">A short leash.</span></>} aside={<GuardsReadout />}>
        <p>Calls and puts only — no stocks. If it&apos;s still holding shares from before, it dumps them for cash and buys contracts. It can crowd into a name, but the code won&apos;t let it faceplant the whole account on one contract or one ticker, and it spends settled cash only. Past that it sizes however it likes. It can absolutely still lose my money. That&apos;s the show.</p>
      </LogSection>

      {/* ── 04 · The toolbox ── */}
      <LogSection id="tools" comment="The only actions that exist" flip title={<>It can only use the <span className="text-primary">tools I built.</span></>} aside={<ToolLs />}>
        <p>Claudia gets the tools I hand-built and nothing else. No surprise powers, no rogue wire transfers, no ordering GPUs on my card. If a button doesn&apos;t exist, she can&apos;t press it.</p>
      </LogSection>

      {/* ── 05 · The receipts ── */}
      <LogSection
        id="receipts"
        comment="Streams live as you reach it"
        title={<>Every move, <span className="text-primary">on the record.</span></>}
        aside={
          <div className="rounded-2xl border border-border bg-card p-5 shadow-lg sm:p-6">
            <LiveTranscript lines={transcript} />
          </div>
        }
      >
        <p>Every move is logged, tool by tool. The session beside this streams on its own: watch it overthink a quote, buy something anyway, and file a very confident note about why. It can&apos;t hide a bad call from you, and it has a real talent for them.</p>
      </LogSection>

      {/* ── Parody social proof ── */}
      <div className="relative pt-12 sm:pt-16">
        <Testimonials />
        <TrustedBy />
      </div>

      {/* ── The ask ── */}
      <section className="relative px-5 pb-20 pt-14 sm:px-6 sm:pb-24 sm:pt-20">
        <div className="mx-auto max-w-3xl">
          <Reveal effect="rise" duration={800}>
            <div className="relative overflow-hidden rounded-3xl border border-border bg-card px-6 py-12 text-center shadow-lg sm:px-12 sm:py-14">
              <div className="pointer-events-none absolute inset-0" aria-hidden>
                <div className="lg-mesh" />
              </div>
              <div className="relative">
                <span className="inline-flex items-center gap-1.5 rounded-full bg-secondary px-3 py-1 text-xs font-medium text-secondary-foreground">
                  <Mail className="h-3.5 w-3.5" />
                  Daily recap · free
                </span>
                <h2 className="mt-5 font-display text-3xl font-bold tracking-tight sm:text-4xl">
                  Start getting <span className="text-primary">the recap</span>
                </h2>
                <p className="mx-auto mt-4 max-w-xl text-muted-foreground">
                  One email a day so you can watch the carnage. Free, no catch, leave whenever. <span className="italic">(I will hate you.)</span>
                </p>
                <div className="mt-8 flex flex-col items-center gap-4">
                  <SubscribeCTA className="term-btn term-btn-solid">
                    <Mail className="h-4 w-4" />
                    Sign in or sign up
                  </SubscribeCTA>
                  <Link href="/dashboard" className="group inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground">
                    or just watch the book live
                    <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
                  </Link>
                </div>
              </div>
            </div>
          </Reveal>

          {/* Three quiet reassurances under the ask. */}
          <Reveal effect="fade" delay={150} duration={700}>
            <div className="mt-8 grid grid-cols-1 gap-3 sm:grid-cols-3">
              <Assurance Icon={LineChart} title="Fully transparent" body="Every position and decision is public, marked live." />
              <Assurance Icon={Lock} title="Hard-capped" body="Position and per-name sizing enforced in code." />
              <Assurance Icon={Mail} title="Always free" body="No fees, no upsell — watch and get the recap." />
            </div>
          </Reveal>
        </div>
      </section>

      <Footer />
    </div>
  );
}

function Assurance({ Icon, title, body }: { Icon: typeof LineChart; title: string; body: string }) {
  return (
    <div className="rounded-xl border border-border bg-card p-4 text-left shadow-sm">
      <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-secondary text-primary">
        <Icon className="h-4 w-4" />
      </span>
      <div className="mt-3 text-sm font-medium">{title}</div>
      <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{body}</div>
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
    tool: "buy_option",
    open: true,
    payload: { symbol: "NVDA 150C", quantity: 5, limit_price: 4.2 },
    result: { ok: true, action: "buy_option", order_id: "100482731", status: "working" },
  },
  {
    type: "tool",
    tool: "get_order_status",
    payload: { order_id: "100482731" },
    result: { status: "filled", filled_quantity: 5, fill_price: 4.18 },
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
      synopsis: "Added a small NVDA call on a 50-day hold; kept MSFT.",
      action_items: ["Trim NVDA if it loses the 50-day", "Confirm the NVDA fill settled before redeploying"],
    },
    result: { ok: true, action: "write_summary", stored: true },
  },
];
