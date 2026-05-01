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
    "An LLM ranks 10 options contracts every weekday with live market data and a written rationale for each. The rank-1 pick auto-fires in my actual brokerage account every morning. Free to watch, expensive to run.",
};

export default function LandingPage() {
  return (
    <div className="relative min-h-dvh overflow-hidden bg-background text-foreground">
      <AmbientBackground position="absolute" />

      {/* ── Glass nav ── */}
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
              Live now &middot; free daily picks before market open
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
              Every weekday at 9:25 ET, an LLM ranks 10 options contracts with live market data and a written essay defending each one. At 9:30 the rank-1 pick auto-fires in my actual brokerage
              account, with my actual money. By close you find out whether Claude was right. <span className="italic">It is sometimes.</span>
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
              <SubscribeCTA className="lg-panel lg-edge-shine inline-flex h-12 items-center justify-center gap-2 rounded-full px-7 text-[15px] font-semibold text-foreground transition-transform hover:-translate-y-0.5">
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

      {/* ── Auto-execution timeline panel ── */}
      <section className="relative px-5 pb-20 sm:px-6 sm:pb-28">
        <div className="relative z-10 mx-auto max-w-5xl">
          <Reveal effect="rise" duration={700}>
            <div className="lg-panel lg-edge-shine overflow-hidden p-6 sm:p-10">
              <div className="flex flex-col gap-2 text-center sm:gap-3">
                <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">The pipeline</span>
                <h2 className="text-2xl font-extrabold tracking-tight text-foreground sm:text-3xl">A real trade fires every weekday. No clicks needed.</h2>
                <p className="mx-auto max-w-2xl text-sm text-muted-foreground sm:text-base">
                  Picks land in your inbox before the bell. At 9:30 ET the rank-1 contract gets ordered live in my actual Schwab account with my actual money, held until 3:55 ET, and unconditionally
                  closed before the close. You watch it happen on the dashboard. I watch it happen on the dashboard. Honestly, that&apos;s most of what &ldquo;oversight&rdquo; means around here.
                </p>
              </div>

              <div className="mt-10 grid gap-4 sm:grid-cols-4">
                {pipeline.map((p, i) => (
                  <Reveal key={p.time} effect="rise" delay={i * 90} duration={600}>
                    <div className="lg-panel-strong relative h-full rounded-2xl border border-foreground/5 p-4 dark:border-white/5">
                      <div className="flex items-center justify-between">
                        <span className="text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">{p.time}</span>
                        <span className="rounded-full bg-foreground/5 px-2 py-0.5 text-[10px] font-semibold text-muted-foreground dark:bg-white/5">{String(i + 1).padStart(2, "0")}</span>
                      </div>
                      <div className="mt-3 flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-brand text-background">
                        <p.Icon className="h-4 w-4" />
                      </div>
                      <h3 className="mt-3 text-sm font-bold text-foreground">{p.title}</h3>
                      <p className="mt-1 text-[13px] leading-relaxed text-muted-foreground">{p.detail}</p>
                    </div>
                  </Reveal>
                ))}
              </div>
            </div>
          </Reveal>
        </div>
      </section>

      {/* ── Features ── */}
      <section className="relative px-5 pb-20 sm:px-6 sm:pb-28">
        <div className="relative z-10 mx-auto max-w-6xl">
          <Reveal effect="blur" duration={1000} className="mx-auto mb-12 max-w-2xl text-center sm:mb-14">
            <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">What you get</span>
            <h2 className="mt-3 text-3xl font-extrabold tracking-tight sm:text-4xl">
              Built to <span className="text-gradient-brand">show its work</span>
            </h2>
            <p className="mt-3 text-muted-foreground">
              Every pick comes with the contract spec, a 1-10 conviction score, and Claude&apos;s whole essay defending it. We don&apos;t cherry-pick the wins because we don&apos;t actually know which
              ones will be wins.
            </p>
          </Reveal>

          <div className="grid gap-4 sm:grid-cols-2 sm:gap-5 lg:grid-cols-3">
            {features.map((f, i) => {
              const col = i % 3;
              const row = Math.floor(i / 3);
              const stagger = (col + row) * 90;
              return (
                <Reveal key={f.title} effect="rise" delay={stagger} duration={750}>
                  <div className="lg-panel lg-edge-shine group h-full p-6 transition-transform duration-300 hover:-translate-y-0.5">
                    <div className="inline-flex h-10 w-10 items-center justify-center rounded-xl bg-foreground/5 dark:bg-white/5">
                      <f.Icon className="h-5 w-5 text-foreground/80" />
                    </div>
                    <h3 className="mt-4 text-base font-bold tracking-tight">{f.title}</h3>
                    <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{f.description}</p>
                  </div>
                </Reveal>
              );
            })}
          </div>
        </div>
      </section>

      {/* ── Testimonials (clearly-satirical) ── */}
      <Testimonials />

      {/* ── Subscribe CTA ── */}
      <section className="relative px-5 pb-20 sm:px-6 sm:pb-28">
        <div className="relative z-10 mx-auto max-w-3xl">
          <Reveal effect="blur" duration={1000}>
            <div className="lg-panel lg-edge-shine relative overflow-hidden p-8 text-center sm:p-12">
              <div className="lg-orb lg-orb-claude absolute h-[420px] w-[420px] -top-24 -left-24 opacity-40" aria-hidden />
              <div className="lg-orb lg-orb-cyan absolute h-[360px] w-[360px] -bottom-24 -right-20 opacity-40" aria-hidden />
              <div className="relative">
                <h2 className="text-3xl font-extrabold tracking-tight sm:text-4xl">
                  Start getting <span className="text-gradient-brand">picks</span>
                </h2>
                <p className="mx-auto mt-3 max-w-xl text-muted-foreground">
                  Completely free. No credit card. No premium tier I&apos;m secretly building. One silly model doing its best, one human absolutely hoping it knows what it&apos;s doing. Unsubscribe
                  any time &mdash; I won&apos;t email you a sad cat photo.
                </p>
                <div className="mt-7 flex w-full flex-col items-center justify-center gap-3 sm:flex-row">
                  <SubscribeCTA className="inline-flex h-12 items-center justify-center gap-2 rounded-full bg-gradient-brand px-7 text-[15px] font-semibold text-white shadow-lg transition-opacity hover:opacity-90">
                    <LogIn className="h-4 w-4" />
                    Sign in or sign up
                  </SubscribeCTA>
                  <Link
                    href="/dashboard"
                    className="lg-panel lg-edge-shine inline-flex h-12 items-center justify-center gap-2 rounded-full px-7 text-[15px] font-semibold text-foreground transition-transform hover:-translate-y-0.5"
                  >
                    Open dashboard
                    <ArrowRight className="h-4 w-4" />
                  </Link>
                </div>
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
    time: "9:25 AM ET",
    title: "Claude picks",
    detail: "Pulls market signals, calls Schwab quotes and option chains, ranks 10 contracts. Whether it 'analyzed' anything is between Claude and its conscience.",
    Icon: Sparkles,
  },
  {
    time: "9:30 AM ET",
    title: "Auto-fire rank-1",
    detail: "Top pick fires live in my actual Schwab account. Real money, real fills. Capped at $5/share so the worst case is a manageable amount of regret on my part.",
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
    title: "Auto-fired live trade",
    description:
      "The rank-1 pick auto-fires every morning in my actual brokerage account, with my actual money. Capped at $5/share, mandatory close at 3:55 ET. No clicks, no overnight risk, no chance for me to second-guess it.",
  },
  {
    Icon: Mail,
    title: "Pre-market email",
    description: "Ranked picks in your inbox before the opening bell. EOD results at close. Friday digest. All free, all automated, unsubscribe whenever.",
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
