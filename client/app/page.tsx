import { ArrowRight, Clock, Eye, LogIn, Mail, Shield, Sparkles, TrendingUp, Zap } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { AmbientBackground } from "@/components/landing/ambient-background";
import { LandingNavAccount } from "@/components/landing/nav-account";
import { Reveal } from "@/components/landing/reveal";
import { SubscribeCTA } from "@/components/landing/subscribe-cta";
import { Testimonials } from "@/components/landing/testimonials";
import { ClaudeLogo } from "@/components/ui/brand-icons";

export const metadata: Metadata = {
  title: "VibeTradez | AI-Powered Options Picks",
  description:
    "An LLM ranks 10 options contracts every weekday with live market data and a written rationale for each. The top 3 picks auto-fire in my actual brokerage account every morning. Free to watch, expensive to run.",
};

export default function LandingPage() {
  return (
    <div className="relative min-h-dvh overflow-hidden bg-background text-foreground">
      <AmbientBackground position="absolute" />

      {/* ── Floating glass nav (kept — it's a fixed surface, not a content card) ── */}
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
              Every weekday at 9:30 ET, an LLM ranks 10 options contracts with live market data and a written essay defending each one. The top 3 picks auto-fire in my actual brokerage account, with
              my actual money. By close you find out whether Claude was right. <span className="italic">It is sometimes.</span>
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
              <span className="font-medium">Claude</span>
            </div>
          </Reveal>
        </div>
      </section>

      {/* ── Pipeline as a flat horizontal timeline ── */}
      <section className="relative px-5 pb-24 sm:px-6 sm:pb-32">
        <div className="relative z-10 mx-auto max-w-6xl">
          <Reveal effect="rise" duration={700}>
            <div className="mx-auto max-w-2xl text-center">
              <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">The pipeline</span>
              <h2 className="mt-3 text-3xl font-extrabold tracking-tight text-foreground sm:text-4xl">Real trades fire every weekday. No clicks needed.</h2>
              <p className="mx-auto mt-3 max-w-2xl text-sm text-muted-foreground sm:text-base">
                At 9:30 ET sharp, Claude ranks 10 contracts against live post-open quotes and emails the list. The top 3 picks get ordered live in my actual Schwab account, held until 3:55 ET, and
                unconditionally closed before the bell.
              </p>
            </div>
          </Reveal>

          {/* Timeline — connecting hairline runs through the middle of the row on desktop;
              vertical accent line on mobile. */}
          <div className="relative mt-14">
            <div className="absolute top-[34px] left-[12.5%] hidden h-px w-[75%] bg-gradient-to-r from-transparent via-foreground/15 to-transparent md:block" aria-hidden />
            <div className="absolute top-0 bottom-0 left-[14px] w-px bg-gradient-to-b from-transparent via-foreground/15 to-transparent md:hidden" aria-hidden />

            <ol className="grid grid-cols-1 gap-10 md:grid-cols-4 md:gap-6">
              {pipeline.map((p, i) => (
                <Reveal key={p.time} effect="rise" delay={i * 100} duration={600}>
                  <li className="relative flex gap-4 md:block">
                    {/* Numbered dot — sits on the timeline */}
                    <div className="flex shrink-0 flex-col items-center md:items-start">
                      <span className="relative inline-flex h-8 w-8 items-center justify-center rounded-full bg-background text-xs font-bold ring-1 ring-foreground/15">
                        <span className="absolute -inset-[5px] rounded-full bg-gradient-brand opacity-10" aria-hidden />
                        <span className="relative">{String(i + 1).padStart(2, "0")}</span>
                      </span>
                    </div>

                    <div className="min-w-0 flex-1 md:mt-5">
                      <div className="flex items-center gap-2">
                        <p.Icon className="h-3.5 w-3.5 text-foreground/60" aria-hidden />
                        <span className="text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">{p.time}</span>
                      </div>
                      <h3 className="mt-2 text-base font-bold tracking-tight text-foreground">{p.title}</h3>
                      <p className="mt-2 text-[13.5px] leading-relaxed text-muted-foreground">{p.detail}</p>
                    </div>
                  </li>
                </Reveal>
              ))}
            </ol>
          </div>
        </div>
      </section>

      {/* ── Features — flat 3-col flow with hairline dividers, no card chrome ── */}
      <section className="relative border-t border-foreground/5 px-5 pb-24 sm:px-6 sm:pb-32 dark:border-white/5">
        <div className="relative z-10 mx-auto max-w-6xl pt-20 sm:pt-24">
          <Reveal effect="blur" duration={1000} className="mx-auto mb-14 max-w-2xl text-center sm:mb-16">
            <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">What you get</span>
            <h2 className="mt-3 text-3xl font-extrabold tracking-tight sm:text-4xl">
              Built to <span className="text-gradient-brand">show its work</span>
            </h2>
            <p className="mt-3 text-muted-foreground">
              Every pick comes with the contract spec, a 1-10 conviction score, and Claude&apos;s whole essay defending it. We don&apos;t cherry-pick the wins because we don&apos;t actually know which
              ones will be wins.
            </p>
          </Reveal>

          <div className="grid grid-cols-1 gap-x-10 gap-y-12 sm:grid-cols-2 lg:grid-cols-3 lg:gap-x-12 lg:gap-y-14">
            {features.map((f, i) => {
              const col = i % 3;
              const row = Math.floor(i / 3);
              const stagger = (col + row) * 90;
              return (
                <Reveal key={f.title} effect="rise" delay={stagger} duration={750}>
                  <div className="group h-full">
                    <div className="inline-flex h-10 w-10 items-center justify-center rounded-xl bg-foreground/5 text-foreground/80 transition-colors group-hover:bg-foreground/10 dark:bg-white/5">
                      <f.Icon className="h-5 w-5" />
                    </div>
                    <h3 className="mt-5 text-base font-bold tracking-tight">{f.title}</h3>
                    <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{f.description}</p>
                  </div>
                </Reveal>
              );
            })}
          </div>
        </div>
      </section>

      {/* ── Testimonials ── */}
      <Testimonials />

      {/* ── Subscribe CTA — flat hero-style block, no outer panel ── */}
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
                Completely free. No credit card. No premium tier I&apos;m secretly building. One silly model doing its best, one human absolutely hoping it knows what it&apos;s doing. Unsubscribe any
                time &mdash; I won&apos;t email you a sad cat photo.
              </p>
              <div className="mt-8 flex w-full flex-col items-center justify-center gap-3 sm:flex-row">
                <SubscribeCTA className="inline-flex h-12 items-center justify-center gap-2 rounded-full bg-gradient-brand px-7 text-[15px] font-semibold text-white shadow-lg transition-opacity hover:opacity-90">
                  <LogIn className="h-4 w-4" />
                  Sign in or sign up
                </SubscribeCTA>
                <Link
                  href="/dashboard"
                  className="inline-flex h-12 items-center justify-center gap-2 rounded-full border border-foreground/10 bg-foreground/[0.03] px-7 text-[15px] font-semibold text-foreground transition-colors hover:border-foreground/25 hover:bg-foreground/[0.06]"
                >
                  Open dashboard
                  <ArrowRight className="h-4 w-4" />
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

const pipeline = [
  {
    time: "9:30 AM ET",
    title: "Claude picks against the bell",
    detail:
      "Right at the open, an LLM pulls market signals, calls Schwab for live quotes and option chains, and ranks 10 contracts with a written rationale for each. Live post-open prices, not pre-market guesses.",
    Icon: Sparkles,
  },
  {
    time: "9:31-9:33 AM ET",
    title: "Auto-fire the basket",
    detail:
      "The top 3 picks fire live in my actual Schwab account a minute or two after the bell. Real money, real fills. Each contract capped at $5/share, with a $1,000 daily basket ceiling so the worst case is a manageable amount of regret on my part.",
    Icon: Zap,
  },
  {
    time: "Mid-day",
    title: "Live dashboard",
    detail: "Buy and Current marks tick in real time. Position is clearly badged so there's no mystery about whether the trade is real (it is).",
    Icon: Eye,
  },
  {
    time: "3:55 PM ET",
    title: "Mandatory close",
    detail: "Position is unconditionally closed five minutes before the bell. No overnight risk. No 'let it ride' improvisation.",
    Icon: Clock,
  },
];

const features = [
  {
    Icon: Sparkles,
    title: "Conviction-scored picks",
    description: "Every pick has a 1-10 conviction score and a written rationale defending it. Claude is opinionated. Sometimes that's a good thing.",
  },
  {
    Icon: TrendingUp,
    title: "Live market data",
    description: "Real-time quotes and full option chains from Schwab. Claude calls tools mid-analysis instead of hallucinating prices, which is a genuine upgrade from 'making it up'.",
  },
  {
    Icon: Zap,
    title: "Auto-fired live basket",
    description:
      "The top 3 picks auto-fire every morning in my actual brokerage account, with my actual money. $5/share per contract under a $1,000 daily basket ceiling, mandatory close at 3:55 ET. No clicks, no overnight risk, no chance for me to second-guess it.",
  },
  {
    Icon: Mail,
    title: "Pre-market email",
    description: "Ranked picks in your inbox right after the open. EOD results at close. Friday digest. All free, all automated, unsubscribe whenever.",
  },
  {
    Icon: Clock,
    title: "End-of-day tracking",
    description: "Every pick tracked to close. Win rate, P&L, Sharpe, drawdown — all computed honestly. No 'trust me bro' screenshots, no quietly-removed losers.",
  },
  {
    Icon: Shield,
    title: "Completely free",
    description: "No paywalls. No premium tier. No 'pro plan'. A live experiment in letting one model trade. You watch, you judge, you bring your own popcorn.",
  },
];
