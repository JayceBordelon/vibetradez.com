import { ArrowRight, Brain, Clock, LogIn, Mail, Shield, TrendingUp } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { Reveal } from "@/components/landing/reveal";
import { SubscribeCTA } from "@/components/landing/subscribe-cta";
import { ClaudeLogo, OpenAILogo } from "@/components/ui/brand-icons";

export const metadata: Metadata = {
  title: "VibeTradez | AI-Powered Options Picks",
  description:
    "A controlled experiment between two silly models. GPT-5.4 and Claude Opus 4.6 get the exact same market signals, the exact same prompt, and the exact same tools, then independently pick their top 10 and roast each other's picks. Delivered before market open.",
};

export default function LandingPage() {
  return (
    <div className="min-h-dvh bg-background text-foreground">
      {/* ── Nav ── */}
      <nav className="fixed top-0 z-50 w-full border-b border-border/50 bg-background/95 backdrop-blur-md will-change-transform">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-5 sm:px-6">
          <span className="text-xl font-extrabold tracking-tight">
            <span className="text-foreground">Vibe</span>
            <span className="text-gradient-brand">Tradez</span>
          </span>
          <div className="flex items-center gap-3">
            <SubscribeCTA className="hidden rounded-lg border border-border px-3.5 py-2 text-sm font-semibold text-foreground transition-colors hover:bg-muted sm:inline-flex">Sign in</SubscribeCTA>
            <Link href="/dashboard" className="inline-flex items-center gap-2 rounded-lg bg-foreground px-3.5 py-2 text-sm font-semibold text-background transition-opacity hover:opacity-90 sm:px-4">
              Dashboard
              <ArrowRight className="h-4 w-4" />
            </Link>
          </div>
        </div>
      </nav>

      {/* ── Hero ── */}
      <section className="relative flex min-h-dvh items-center justify-center overflow-hidden pt-16">
        {/* Gradient orbs */}
        <div className="pointer-events-none absolute inset-0 overflow-hidden will-change-transform">
          <div className="absolute -top-1/4 left-1/4 h-[600px] w-[600px] rounded-full bg-gpt/10 blur-[80px] sm:blur-[120px]" />
          <div className="absolute -bottom-1/4 right-1/4 h-[600px] w-[600px] rounded-full bg-claude/10 blur-[80px] sm:blur-[120px]" />
          <div className="absolute top-1/2 left-1/2 h-[400px] w-[400px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary/5 blur-[60px] sm:blur-[100px]" />
        </div>

        {/* Grid pattern */}
        <div
          className="pointer-events-none absolute inset-0 opacity-[0.03] dark:opacity-[0.04]"
          style={{
            backgroundImage: "linear-gradient(var(--foreground) 1px, transparent 1px), linear-gradient(90deg, var(--foreground) 1px, transparent 1px)",
            backgroundSize: "60px 60px",
            contain: "strict",
          }}
        />

        <div className="relative z-10 mx-auto max-w-4xl px-5 py-20 text-center sm:px-6 sm:py-24">
          {/* Badge */}
          <Reveal
            effect="fall"
            duration={600}
            className="mb-8 inline-flex items-center gap-2 rounded-full border border-border bg-card/80 px-3 py-1.5 text-xs font-medium text-muted-foreground backdrop-blur-sm sm:mb-8 sm:px-4 sm:py-2 sm:text-sm"
          >
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green opacity-75" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-green" />
            </span>
            Free daily picks before market open
          </Reveal>

          {/* Headline */}
          <Reveal effect="blur" delay={120} duration={1100} as="header">
            <h1 className="mb-6 text-4xl font-extrabold leading-[1.1] tracking-tight sm:mb-6 sm:text-6xl lg:text-7xl">
              Two silly models.
              <br />
              <span className="text-gradient-brand">Zero humans.</span>
            </h1>
          </Reveal>

          <Reveal effect="rise" delay={280} duration={900}>
            <p className="mx-auto mb-10 max-w-2xl text-base leading-relaxed text-muted-foreground sm:mb-10 sm:text-xl">
              A controlled experiment between two silly models. Every morning, GPT-5.4 and Claude Opus 4.6 get the exact same market signals, the exact same prompt, and the exact same tools, then go
              off alone and each pick their top 10. After they finish, each model reads the other&apos;s picks and writes a one-sentence roast (or grudging nod) on every trade. You get the union
              before market open, both rationales attached, and a live argument in your inbox.
            </p>
          </Reveal>

          {/* CTAs */}
          <Reveal effect="scale" delay={420} duration={700} className="flex w-full flex-col gap-4 px-2 sm:w-auto sm:flex-row sm:justify-center sm:gap-4 sm:px-0">
            <Link
              href="/dashboard"
              className="inline-flex items-center justify-center gap-2 rounded-xl bg-foreground px-8 py-3.5 text-base font-semibold text-background shadow-lg transition-opacity hover:opacity-90"
            >
              View Live Dashboard
              <ArrowRight className="h-4 w-4" />
            </Link>
            <SubscribeCTA className="inline-flex items-center justify-center gap-2 rounded-xl bg-gradient-brand px-8 py-3.5 text-base font-semibold text-white shadow-lg transition-opacity hover:opacity-90">
              <LogIn className="h-4 w-4" />
              Sign in or sign up
            </SubscribeCTA>
          </Reveal>

          {/* Model badges: opposing slide-ins meeting at the divider */}
          <div className="mt-12 flex items-center justify-center gap-6 sm:mt-12 sm:gap-8">
            <Reveal effect="left" delay={560} duration={700} className="flex items-center gap-2.5 text-sm text-muted-foreground">
              <OpenAILogo className="h-5 w-5" />
              <span className="font-medium">GPT-5.4</span>
            </Reveal>
            <Reveal effect="scale" delay={680} duration={500} className="h-4 w-px bg-border" />
            <Reveal effect="right" delay={560} duration={700} className="flex items-center gap-2.5 text-sm text-muted-foreground">
              <ClaudeLogo className="h-5 w-5" />
              <span className="font-medium">Claude Opus 4.6</span>
            </Reveal>
          </div>
        </div>
      </section>

      {/* ── Features ── */}
      <section className="border-t bg-card py-20 sm:py-24">
        <div className="mx-auto max-w-6xl px-5 sm:px-6">
          <Reveal effect="blur" duration={1000} className="mb-14 text-center sm:mb-16">
            <h2 className="mb-4 text-3xl font-extrabold tracking-tight sm:text-4xl">
              Built to <span className="text-gradient-brand">disagree</span>
            </h2>
            <p className="mx-auto max-w-2xl text-muted-foreground">
              Not another signal bot. Same data, same prompt, same tools: the only thing that changes is which model is reasoning. Then they cross-examine each other&apos;s picks and you get to watch
              the drama in real time. It&apos;s a very expensive A/B test.
            </p>
          </Reveal>

          <div className="grid gap-5 sm:grid-cols-2 sm:gap-6 lg:grid-cols-3">
            {features.map((f, i) => {
              // Diagonal stagger across the 3-col grid (row + col), with a
              // gentle alternating tilt so adjacent cards never animate the
              // same direction. Mobile collapses to one col which still
              // staggers nicely top-to-bottom.
              const col = i % 3;
              const row = Math.floor(i / 3);
              const stagger = (col + row) * 90;
              const effect = i % 2 === 0 ? "rise" : "tilt";
              return (
                <Reveal key={f.title} effect={effect} delay={stagger} duration={750} className="group rounded-2xl border border-border bg-background p-6 transition-all hover:shadow-md sm:p-6">
                  <div className="mb-4 inline-flex rounded-xl bg-muted p-3">{f.icon}</div>
                  <h3 className="mb-2 text-lg font-bold">{f.title}</h3>
                  <p className="text-sm leading-relaxed text-muted-foreground">{f.description}</p>
                </Reveal>
              );
            })}
          </div>
        </div>
      </section>

      {/* ── How It Works ── */}
      <section id="how-it-works" className="scroll-mt-16 border-t py-20 sm:py-24">
        <div className="mx-auto max-w-5xl px-5 sm:px-6">
          <Reveal effect="blur" duration={1000} className="mb-14 text-center sm:mb-16">
            <h2 className="mb-4 text-3xl font-extrabold tracking-tight sm:text-4xl">How it works</h2>
            <p className="mx-auto max-w-2xl text-muted-foreground">The whole pipeline runs automatically. I mostly just watch and try not to intervene.</p>
          </Reveal>

          <div className="relative">
            {/* Vertical gradient line */}
            <div
              className="absolute left-6 top-0 bottom-0 hidden w-px sm:block"
              style={{
                background: "linear-gradient(180deg, var(--gpt), var(--claude))",
              }}
            />

            <div className="space-y-10 sm:space-y-12">
              {steps.map((step, i) => (
                // Each step slides in from the left, walking down the
                // gradient timeline. Slight delay growth as you scroll
                // creates a stairstep cadence.
                <Reveal key={step.title} effect="left" delay={i * 110} duration={750} className="relative flex gap-6">
                  <div className="relative z-10 hidden flex-shrink-0 sm:block">
                    <div className="flex h-12 w-12 items-center justify-center rounded-full border-2 border-border bg-card text-sm font-bold text-foreground">{i + 1}</div>
                  </div>
                  <div className="flex-1 rounded-2xl border border-border bg-card p-5 sm:p-6">
                    <div className="mb-1 flex items-center gap-3">
                      <span className="inline-flex h-8 w-8 items-center justify-center rounded-full bg-muted text-xs font-bold text-foreground sm:hidden">{i + 1}</span>
                      <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">{step.time}</span>
                    </div>
                    <h3 className="mb-2 text-lg font-bold">{step.title}</h3>
                    <p className="text-sm leading-relaxed text-muted-foreground">{step.description}</p>
                  </div>
                </Reveal>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* ── Subscribe CTA ── */}
      <section className="border-t py-20 sm:py-24">
        <div className="mx-auto max-w-3xl px-5 text-center sm:px-6">
          <Reveal effect="blur" duration={1000}>
            <h2 className="mb-4 text-3xl font-extrabold tracking-tight sm:text-4xl">
              Start getting <span className="text-gradient-brand">picks</span>
            </h2>
            <p className="mx-auto mb-10 max-w-xl text-muted-foreground sm:mb-8">
              Completely free. No credit card. No premium tier. Just two silly models doing their best and one human hoping they know what they&apos;re doing. Unsubscribe any time, no hard feelings.
            </p>
          </Reveal>
          <Reveal effect="scale" delay={200} duration={650} className="flex w-full flex-col gap-4 px-2 sm:w-auto sm:flex-row sm:justify-center sm:gap-4 sm:px-0">
            <SubscribeCTA className="inline-flex items-center justify-center gap-2 rounded-xl bg-gradient-brand px-8 py-3.5 text-base font-semibold text-white shadow-lg transition-opacity hover:opacity-90">
              <LogIn className="h-4 w-4" />
              Sign in or sign up
            </SubscribeCTA>
            <Link
              href="/dashboard"
              className="inline-flex items-center justify-center gap-2 rounded-xl border border-border px-8 py-3.5 text-base font-semibold text-foreground transition-colors hover:bg-muted"
            >
              Open Dashboard
              <ArrowRight className="h-4 w-4" />
            </Link>
          </Reveal>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer className="border-t bg-card">
        <div className="mx-auto flex max-w-6xl flex-col gap-4 px-5 py-8 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div className="flex items-center gap-2">
            <span className="font-extrabold text-foreground">
              Vibe<span className="text-gradient-brand">Tradez</span>
            </span>
            <span>&copy; {new Date().getFullYear()}</span>
          </div>
          <p className="max-w-lg leading-relaxed">
            Not financial advice. Options trading involves substantial risk. All P&amp;L figures are hypothetical. Past performance does not guarantee future results.
          </p>
          <div className="flex gap-4">
            <Link href="/terms" className="underline underline-offset-2 hover:text-foreground">
              Terms
            </Link>
            <Link href="/faq" className="underline underline-offset-2 hover:text-foreground">
              FAQ
            </Link>
            <a href="https://jaycebordelon.com" target="_blank" rel="noopener noreferrer" className="underline underline-offset-2 hover:text-foreground">
              Built by Jayce
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}

const features = [
  {
    icon: <Brain className="h-6 w-6 text-gpt" />,
    title: "Independent picks",
    description: "Same data, same prompt, same tools. Each model goes off alone and ranks its top 10. No groupthink, no peeking, no shared scratchpad. The only variable is the model itself.",
  },
  {
    icon: <TrendingUp className="h-6 w-6 text-claude" />,
    title: "Live market data",
    description: "Real-time quotes and full option chains from Schwab. Both models call tools mid-analysis to look up actual prices instead of hallucinating them. Progress.",
  },
  {
    icon: <Brain className="h-6 w-6 text-claude" />,
    title: "Cross-examination pass",
    description: "Once both lists are locked, each model reads the other's picks and writes a one-sentence verdict on every trade. Roasts, grudging respect, and the occasional cosign.",
  },
  {
    icon: <Mail className="h-6 w-6 text-claude" />,
    title: "Pre-market email",
    description: "Ranked picks in your inbox before the opening bell. EOD results at close. Weekly digest on Fridays. You can also just watch the dashboard and judge silently.",
  },
  {
    icon: <Shield className="h-6 w-6 text-gpt" />,
    title: "Completely free",
    description: "No paywalls, no premium tiers, no credit card. A live experiment in letting silly models trade. Follow along and see how it goes.",
  },
  {
    icon: <Clock className="h-6 w-6 text-claude" />,
    title: "End-of-day tracking",
    description: 'Every pick is tracked to close. Win rates, P&L, Sharpe, and drawdown are all computed automatically. No cherry-picking, no "trust me bro" screenshots.',
  },
];

const steps = [
  {
    time: "9:00 AM ET",
    title: "Market signal scan",
    description: "The system aggregates trending tickers and market signals from StockTwits, Yahoo Finance, Finviz, and SEC EDGAR filings. The exact same payload is handed to both models.",
  },
  {
    time: "9:15 AM ET",
    title: "Independent picks",
    description:
      "GPT-5.4 and Claude Opus 4.6 are run in parallel, alone, with the same prompt and the same Schwab + web search tools. Each produces 10 ranked contracts with conviction scores and written rationales. Neither model sees the other's work yet.",
  },
  {
    time: "9:20 AM ET",
    title: "Cross-examination",
    description:
      "Once both pick lists are locked, each model reads the other's 10 trades and writes a one-sentence verdict on every single one. Roasts, grudging respect, the occasional cosign. Verdicts are stored on the trade and shown next to the original rationale.",
  },
  {
    time: "9:25 AM ET",
    title: "Merge & deliver",
    description:
      "Picks are unioned. Trades both models picked rank ahead of solo picks, ties broken by combined conviction. The final list, both rationales, and both verdicts are emailed to subscribers before the opening bell.",
  },
  {
    time: "4:05 PM ET",
    title: "End-of-day results",
    description: "After close, the system fetches closing prices, computes hypothetical P&L for every pick, and emails results. Everything is saved to the database and surfaces on the dashboard.",
  },
];
