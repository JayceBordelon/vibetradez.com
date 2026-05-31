import { ArrowRight, Clock, Eye, LogIn, Sparkles, Zap } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { AmbientBackground } from "@/components/landing/ambient-background";
import { LandingNavAccount } from "@/components/landing/nav-account";
import { Reveal } from "@/components/landing/reveal";
import { SubscribeCTA } from "@/components/landing/subscribe-cta";
import { Testimonials } from "@/components/landing/testimonials";
import { TrustedBy } from "@/components/landing/trusted-by";
import { ClaudeLogo } from "@/components/ui/brand-icons";

export const metadata: Metadata = {
  title: "VibeTradez | AI-Powered Options Picks",
  description:
    "An LLM picks 3 options contracts every weekday with live market data and a written rationale for each. All 3 auto-fire in my actual brokerage account at the open. Free to watch, expensive to run.",
};

export default function LandingPage() {
  return (
    <div className="relative min-h-dvh overflow-hidden bg-background text-foreground">
      <AmbientBackground position="absolute" />

      {/* ── Floating glass nav (kept, it's a fixed surface, not a content card) ── */}
      <nav className="fixed top-3 left-1/2 z-50 w-[calc(100%-1.5rem)] -translate-x-1/2 sm:top-4 sm:max-w-5xl">
        <div className="lg-panel lg-edge-shine flex items-center justify-between px-4 py-2 sm:px-5 sm:py-2.5">
          <Link href="/" className="inline-flex min-h-11 items-center text-xl font-extrabold tracking-tight sm:min-h-9">
            <span className="text-foreground">Vibe</span>
            <span className="text-gradient-brand">Tradez</span>
          </Link>
          <div className="flex items-center gap-2">
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

      {/* ── Hero ── */}
      <section className="relative px-5 pt-32 pb-20 sm:px-6 sm:pt-40 sm:pb-28">
        <div className="relative z-10 mx-auto max-w-4xl text-center">
          <Reveal effect="fall" duration={600}>
            <span className="lg-pill inline-flex items-center gap-2 px-3.5 py-1.5 text-xs font-medium text-foreground/80 sm:text-[13px]">
              <span className="relative flex h-2 w-2">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green opacity-75" />
                <span className="relative inline-flex h-2 w-2 rounded-full bg-green" />
              </span>
              Live now &middot; free daily picks at the open
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
              Every weekday, a language model picks 3 options contracts before the bell and auto-fires all 3 in my real brokerage account at the open. By close, you see whether Claudia was right. She is sometimes.
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
      </section>

      {/* ── Pipeline as a market-hours bar: one gradient track spanning the trading day with the
              four moments pinned as icon markers. Horizontal on desktop, vertical rail on mobile. ── */}
      <section className="relative px-5 pb-24 sm:px-6 sm:pb-32">
        <div className="relative z-10 mx-auto max-w-6xl">
          <Reveal effect="rise" duration={700}>
            <div className="mx-auto max-w-2xl text-center">
              <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">The pipeline</span>
              <h2 className="mt-3 text-3xl font-extrabold tracking-tight text-foreground sm:text-4xl">
                Real trades fire every weekday. <span className="text-gradient-brand">No clicks needed.</span>
              </h2>
            </div>
          </Reveal>

          {/* Desktop: horizontal market-hours bar */}
          <div className="mt-16 hidden md:block">
            <div className="mb-5 flex items-center justify-between px-[12.5%] text-[10px] font-semibold uppercase tracking-[0.2em] text-muted-foreground/60">
              <span>Market open</span>
              <span>Close</span>
            </div>
            <div className="relative">
              {/* the trading-day track, marker centers sit at 12.5% / 37.5% / 62.5% / 87.5% */}
              <div className="absolute top-[19px] left-[12.5%] h-1.5 w-[75%] rounded-full bg-gradient-brand opacity-30" aria-hidden />
              <ol className="relative grid grid-cols-4 gap-6">
                {pipeline.map((p, i) => (
                  <Reveal as="li" key={p.time} effect="rise" delay={i * 100} duration={600} className="flex flex-col items-center text-center">
                    <span className="relative z-10 inline-flex h-10 w-10 items-center justify-center rounded-full bg-background ring-1 ring-foreground/15">
                      <span className="absolute -inset-1 rounded-full bg-gradient-brand opacity-15" aria-hidden />
                      <p.Icon className="relative h-4 w-4 text-foreground/70" aria-hidden />
                    </span>
                    <span className="mt-5 text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">{p.time}</span>
                    <h3 className="mt-1.5 text-[15px] font-bold tracking-tight text-foreground">{p.title}</h3>
                    <p className="mt-1.5 max-w-[15rem] text-[13px] leading-relaxed text-muted-foreground">{p.detail}</p>
                  </Reveal>
                ))}
              </ol>
            </div>
          </div>

          {/* Mobile: vertical rail with the same markers */}
          <Reveal effect="rise" delay={150} duration={700} className="mt-12 block md:hidden">
            <ol className="relative space-y-7">
              <div className="absolute top-3 bottom-3 left-[19px] w-1.5 -translate-x-1/2 rounded-full bg-gradient-brand opacity-25" aria-hidden />
              {pipeline.map((p) => (
                <li key={p.time} className="relative flex gap-4">
                  <span className="relative z-10 inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-background ring-1 ring-foreground/15">
                    <span className="absolute -inset-1 rounded-full bg-gradient-brand opacity-15" aria-hidden />
                    <p.Icon className="relative h-4 w-4 text-foreground/70" aria-hidden />
                  </span>
                  <div className="min-w-0 flex-1 pt-0.5">
                    <span className="text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">{p.time}</span>
                    <h3 className="mt-1 text-[15px] font-bold tracking-tight text-foreground">{p.title}</h3>
                    <p className="mt-1 text-[13px] leading-relaxed text-muted-foreground">{p.detail}</p>
                  </div>
                </li>
              ))}
            </ol>
          </Reveal>
        </div>
      </section>

      {/* ── Open source: flat two-column block (no panel/card), left pitch + right plain-text
              source tree, separated by a single hairline so it reads as page content, not a card.
              Sits right after the pipeline so the genuine "read the code" trust beat lands before
              the two parody social-proof sections, not buried among them. ── */}
      <section className="relative px-5 pb-20 sm:px-6 sm:pb-24">
        <div className="relative z-10 mx-auto max-w-5xl">
          <Reveal effect="rise" duration={700}>
            <div className="grid gap-10 md:grid-cols-[1.05fr_0.95fr] md:items-start md:gap-14">
              {/* Left: the pitch */}
              <div>
                <span className="inline-flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                  <GitHubMark className="h-3.5 w-3.5" />
                  Open source
                </span>
                <h2 className="mt-3 text-2xl font-extrabold tracking-tight sm:text-3xl">
                  No black box. <span className="text-gradient-brand">Read the code.</span>
                </h2>
                <p className="mt-3 max-w-md text-sm leading-relaxed text-muted-foreground sm:text-[15px]">
                  Backend, frontend, broker wiring, and the exact prompt Claudia runs on, all public. See a smarter way to pick? Open a PR and I&apos;ll merge it in.
                </p>
                <div className="mt-7 flex flex-wrap items-center gap-x-5 gap-y-3">
                  <a
                    href="https://github.com/JayceBordelon/vibetradez.com"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex h-11 items-center justify-center gap-2 rounded-full border border-foreground/10 bg-foreground/[0.03] px-5 text-sm font-semibold text-foreground transition-colors hover:border-foreground/25 hover:bg-foreground/[0.06]"
                  >
                    <GitHubMark className="h-4 w-4" />
                    View on GitHub
                    <ArrowRight className="h-3.5 w-3.5" />
                  </a>
                  <a
                    href="https://github.com/JayceBordelon/vibetradez.com/issues"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm font-medium text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline"
                  >
                    Browse open issues
                  </a>
                </div>
              </div>

              {/* Right: plain-text source tree, no box, just a hairline rule on desktop */}
              <div className="font-mono text-[13px] md:border-l md:border-foreground/10 md:pl-14">
                <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
                  <GitHubMark className="h-3.5 w-3.5" />
                  JayceBordelon/vibetradez.com
                </div>
                <ul className="mt-4 space-y-2.5">
                  {stack.map((s, i) => (
                    <li key={s.path} className="grid grid-cols-[auto_1fr] items-baseline gap-x-3">
                      <span className="text-foreground/85">
                        <span className="text-muted-foreground/50">{i === stack.length - 1 ? "└─ " : "├─ "}</span>
                        {s.path}
                      </span>
                      <span className="text-[12px] text-muted-foreground">{s.note}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          </Reveal>
        </div>
      </section>

      {/* ── Testimonials (parody social proof) ── */}
      <Testimonials />

      {/* ── Trusted-by parody marquee, comedic warm-up straight into the subscribe CTA ── */}
      <TrustedBy />

      {/* ── Subscribe CTA, flat hero-style block, no outer panel ── */}
      <section className="relative px-5 pb-24 sm:px-6 sm:pb-32">
        <div className="relative z-10 mx-auto max-w-3xl text-center">
          <div className="lg-orb lg-orb-claude absolute h-[420px] w-[420px] -top-24 -left-24 opacity-30" aria-hidden />
          <div className="lg-orb lg-orb-cyan absolute h-[360px] w-[360px] -bottom-24 -right-20 opacity-30" aria-hidden />
          <Reveal effect="blur" duration={1000}>
            <div className="relative">
              <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">Free, forever</span>
              <h2 className="mt-3 text-3xl font-extrabold tracking-tight sm:text-4xl">
                Start getting <span className="text-gradient-brand">picks</span>
              </h2>
              <p className="mx-auto mt-3 max-w-xl text-muted-foreground">
                Free, no credit card, no premium tier. Unsubscribe any time. <span className="italic">(I will hate you.)</span>
              </p>
              <div className="mt-8 flex flex-col items-center gap-4">
                <SubscribeCTA className="inline-flex h-12 items-center justify-center gap-2 rounded-full bg-gradient-brand px-8 text-[15px] font-semibold text-white shadow-lg transition-opacity hover:opacity-90">
                  <LogIn className="h-4 w-4" />
                  Sign in or sign up
                </SubscribeCTA>
                <Link
                  href="/dashboard"
                  className="group inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
                >
                  or just watch the picks live
                  <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
                </Link>
              </div>
            </div>
          </Reveal>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer className="relative border-t border-foreground/5 bg-background/60 backdrop-blur-xl dark:border-white/5">
        <div className="mx-auto flex max-w-6xl flex-col gap-4 px-5 py-8 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div className="flex items-center gap-2">
            <span className="font-extrabold text-foreground">
              Vibe<span className="text-gradient-brand">Tradez</span>
            </span>
            <span>&copy; {new Date().getFullYear()}</span>
          </div>
          <p className="max-w-lg leading-relaxed">
            Not financial advice. Options trading involves substantial risk. All P&amp;L figures are hypothetical except for auto-fired paper or live trades. Past performance does not guarantee future
            results.
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

// GitHub Octocat mark, inline so we don't depend on a lucide icon that
// upstream removed (lucide dropped Github citing trademark concerns).
function GitHubMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" focusable="false" className={className} fill="currentColor">
      <title>GitHub</title>
      <path d="M8 0C3.58 0 0 3.58 0 8a8 8 0 0 0 5.47 7.59c.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.42 7.42 0 0 1 4 0c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

const stack = [
  { path: "server/", note: "Go · cron · daily lifecycle" },
  { path: "execagent/", note: "at-open trader · hard caps" },
  { path: "trades/prompt.go", note: "the exact picker prompt" },
  { path: "schwab/", note: "live quotes · order wiring" },
  { path: "client/", note: "Next.js 16 · React 19" },
];

const pipeline = [
  {
    time: "9:25 AM ET",
    title: "Picks 3 tickers",
    detail: "Reads overnight news and live signals, then returns 3 tickers, each with a direction and a rationale.",
    Icon: Sparkles,
  },
  {
    time: "9:30:00 AM ET",
    title: "Buys the contracts",
    detail: "Reads the live chain and fires real orders, sizing up on conviction against a $1,000 daily budget.",
    Icon: Zap,
  },
  {
    time: "Mid-day",
    title: "Live dashboard",
    detail: "Buy and current marks tick in real time. Every position is badged real, because it is.",
    Icon: Eye,
  },
  {
    time: "3:55 PM ET",
    title: "Mandatory close",
    detail: "Every position closes five minutes before the bell. No overnight risk.",
    Icon: Clock,
  },
];
