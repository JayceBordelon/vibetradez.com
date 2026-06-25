"use client";

import { ArrowRight, ArrowUpRight, Coins, Gauge, Wallet } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";

import { TerminalLoader } from "@/components/ui/terminal-loader";
import { Section } from "@/components/layout/section";
import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { AnimatedNumber } from "@/components/ui/animated-number";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { freshMark, repricePositions } from "@/lib/live-pricing";
import { useVisiblePoll } from "@/hooks/use-visible-poll";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { ClosedTrade, EquityCurvePoint, PortfolioResponse } from "@/types/portfolio";

import { EquityCurveChart } from "./equity-curve-chart";
import { PerformanceMetrics } from "./performance-metrics";
import { TodaysExecutions } from "./todays-executions";

// todaysExecutionsSubtitle counts the day's money moves ("3 moves · $4,680
// deployed") so the section header carries the headline before a single row
// is read. Hold-only days read as the quiet coda they are.
function todaysExecutionsSubtitle(decisions: { action: string; notional?: number }[]): string {
  const moves = decisions.filter((d) => d.action !== "hold");
  const holds = decisions.length - moves.length;
  if (moves.length === 0) {
    return holds > 0 ? `No money moved. ${holds} position${holds === 1 ? "" : "s"} held as-is.` : "Every move the model commits today lands here as it happens.";
  }
  const deployed = moves.filter((d) => d.action.startsWith("buy")).reduce((s, d) => s + (d.notional ?? 0), 0);
  const parts = [`${moves.length} move${moves.length === 1 ? "" : "s"}`];
  if (deployed > 0) parts.push(`$${Math.round(deployed).toLocaleString("en-US")} deployed`);
  if (holds > 0) parts.push(`${holds} held`);
  return parts.join(" · ");
}

const REFRESH_SECONDS = 60;

/*
PortfolioShell is the live dashboard for the single account the agent
manages. It leads with one clear number (account equity), supports it with
a calm KPI row (invested, cash, unrealized), surfaces Claudia's current
read, then the performance section (account value vs SPY plus the risk
strip), today's executions, and links into the rest of the book.
*/
export function PortfolioShell() {
  const [data, setData] = useState<PortfolioResponse | null>(null);
  const [curve, setCurve] = useState<EquityCurvePoint[]>([]);
  const [trades, setTrades] = useState<ClosedTrade[]>([]);
  // Streamed quotes re-price the book between polls, so the strip moves
  // tick by tick rather than once a minute.
  const quotes = useLiveQuotes(Boolean(data?.enabled));

  // Sequence guard: useVisiblePoll fires load() on an interval AND immediately
  // on tab re-focus, so two polls can be in flight at once. Stamp each load
  // and only commit responses from the latest one, or a slow earlier poll can
  // resolve last and clobber fresher data with a stale snapshot.
  const loadSeq = useRef(0);
  const load = useCallback(() => {
    const seq = ++loadSeq.current;
    const fresh = () => seq === loadSeq.current;
    api
      .getPortfolio()
      .then((d) => {
        if (fresh()) setData(d);
      })
      .catch(() => {});
    api
      .getEquityCurve()
      .then((r) => {
        if (fresh()) setCurve(r.points ?? []);
      })
      .catch(() => {});
    // Closed trades feed the win-rate / profit-factor strip; polled with
    // the rest so an intraday fill (the 15-minute reconcile) shows up.
    api
      .getClosedTrades()
      .then((r) => {
        if (fresh()) setTrades(r.trades ?? []);
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    load();
  }, [load]);
  // Always live: refresh whenever the tab is visible, regardless of market
  // hours, so the book/summary stay current even when the market is closed.
  useVisiblePoll(load, REFRESH_SECONDS * 1000, true);

  if (!data) {
    return (
      <div className="mx-auto max-w-[1200px] px-4 py-8 sm:px-7">
        <TerminalLoader
          command="Loading the live book"
          lines={["Connecting to the broker", "Loading the live book and cash", "Marking option positions", "Pulling the equity curve vs SPY", "Reading the executions tape"]}
        />
      </div>
    );
  }

  if (!data.enabled) {
    return (
      <div className="mx-auto max-w-[1200px] px-4 py-8 sm:px-7">
        <div className="mx-auto max-w-md px-2 py-20 text-center">
          <h2 className="font-display text-xl font-semibold tracking-tight">The portfolio manager isn&apos;t running yet</h2>
          <p className="mx-auto mt-2.5 text-sm leading-relaxed text-muted-foreground">
            Once it&apos;s switched on, this page shows the live book: holdings, cash, the equity curve against SPY, and every move the model makes with its reasoning.
          </p>
        </div>
      </div>
    );
  }

  // positions can be a nil slice (JSON null) for an enabled but all-cash
  // account, so default it before any reduce/length access. The streamed
  // quotes then overlay the latest marks so every number below is live to
  // the tick, not to the last poll.
  const positions = repricePositions(data.positions ?? [], quotes);
  const positionsValue = positions.reduce((s, p) => s + p.market_value, 0);
  const totalUnrealized = positions.reduce((s, p) => s + p.unrealized_pnl, 0);
  // Equity is positions + settled cash, the SAME definition the server uses
  // for `data.equity` and the equity curve (positionsValue + settledCash).
  // We recompute it here (rather than read data.equity) only so the live
  // quote overlay moves it tick by tick. Do NOT add unsettled_cash: the
  // server's equity omits it, so adding it here would double-count sale
  // proceeds the moment unsettled cash is ever populated (it is always 0
  // today) and overstate equity against every server-reported figure.
  const equity = positionsValue + data.settled_cash;
  const investedPct = equity > 0 ? (positionsValue / equity) * 100 : 0;
  // Today's delta: LIVE equity against the last end-of-day close BEFORE
  // today (one clean number — the change since yesterday's close).
  const baseline = [...curve].reverse().find((p) => p.date < data.date && p.account_equity > 0)?.account_equity ?? 0;
  const dayDollars = baseline > 0 ? equity - baseline : null;
  const dayChangePct = baseline > 0 && dayDollars !== null ? (dayDollars / baseline) * 100 : null;

  return (
    <div className="animate-soft-rise mx-auto max-w-[1200px] px-4 py-8 sm:px-7">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="font-display text-2xl font-semibold tracking-tight sm:text-[28px]">Live dashboard</h1>
          <p className="mt-1 text-sm text-muted-foreground">One account, managed by Claudia, marked live to the tick.</p>
        </div>
        <LiveBadge />
      </div>

      <SummaryStrip equity={equity} settledCash={data.settled_cash} unsettledCash={data.unsettled_cash} investedPct={investedPct} unrealized={totalUnrealized} dayChangePct={dayChangePct} />

      {/* The day's narrative, right under the numbers — one of the few
          intentional containers: a soft clay tint with a left accent rule
          (Claudia's voice), no hard border or shadow. */}
      <section className="mt-8 rounded-r-xl border-l-2 border-claude/40 bg-claude-light px-5 py-5">
        <div className="flex items-center gap-2">
          <ClaudeLogo className="h-4 w-4" />
          <span className="text-[12px] font-semibold uppercase tracking-[0.1em] text-claude">Claudia&apos;s read</span>
        </div>
        {data.stance ? (
          <p className="mt-2.5 max-w-3xl text-[15px] leading-relaxed text-foreground/90">{data.stance}</p>
        ) : (
          <p className="mt-2.5 max-w-3xl text-[15px] leading-relaxed text-muted-foreground">
            No stance yet today. The midday session (~12:30 PM ET) writes one after it looks at the book; until then the session log has yesterday&apos;s.
          </p>
        )}
        <a href={`/transcripts/${data.date}`} className="group mt-3.5 inline-flex min-h-9 items-center gap-1.5 text-sm font-medium text-claude sm:min-h-0">
          Read the full session, tool call by tool call
          <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />
        </a>
      </section>

      <Section title="Performance" subtitle="Total account value over time against a buy-and-hold SPY benchmark." className="mt-4 border-t border-border/60 pt-9">
        <EquityCurveChart points={curve} today={data.date} liveEquity={equity} liveUnrealized={totalUnrealized} liveSpyMark={freshMark(quotes, "SPY")} />
        {curve.length >= 2 && <PerformanceMetrics points={curve} trades={trades} today={data.date} liveEquity={equity} />}
      </Section>

      <Section title="Today's executions" subtitle={todaysExecutionsSubtitle(data.decisions ?? [])} className="border-t border-border/60 pt-9">
        <TodaysExecutions decisions={data.decisions ?? []} />
      </Section>

      <Section title="Explore the book" subtitle="The full book, the trade history, and the session log live on their own pages." className="border-t border-border/60 pt-9">
        <div className="-mt-2 divide-y divide-border/60">
          <ExploreLink href="/holdings" title="Holdings" blurb={`${positions.length} open position${positions.length === 1 ? "" : "s"} the account holds right now.`} />
          <ExploreLink href="/closed" title="Closed trades" blurb="Every completed round trip, with realized P&L." />
          <ExploreLink href="/transcripts" title="Sessions" blurb="The day-by-day session log, back through every trading day." />
        </div>
      </Section>
    </div>
  );
}

function LiveBadge() {
  return (
    <span className="inline-flex items-center gap-2 rounded-full border border-green-border bg-green-bg px-3 py-1.5 text-xs font-medium text-green">
      <span className="relative flex h-2 w-2" aria-hidden>
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green opacity-60" />
        <span className="relative inline-flex h-2 w-2 rounded-full bg-green" />
      </span>
      Live
    </span>
  );
}

function ExploreLink({ href, title, blurb }: { href: string; title: string; blurb: string }) {
  return (
    <Link href={href} className="group flex items-center justify-between gap-4 py-4 transition-colors hover:bg-foreground/[0.02]">
      <span className="min-w-0">
        <span className="block font-medium transition-colors group-hover:text-primary">{title}</span>
        <span className="mt-0.5 block text-xs leading-relaxed text-muted-foreground">{blurb}</span>
      </span>
      <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
    </Link>
  );
}

function SummaryStrip({
  equity,
  settledCash,
  unsettledCash,
  investedPct,
  unrealized,
  dayChangePct,
}: {
  equity: number;
  settledCash: number;
  unsettledCash: number;
  investedPct: number;
  unrealized: number;
  dayChangePct: number | null;
}) {
  const investedClamped = Math.min(100, Math.max(0, investedPct));
  return (
    <div className="mt-9">
      {/* Hero metric: account equity leads everything, open on the page. */}
      <div>
        <div className="flex items-center gap-1.5">
          <Wallet className="h-4 w-4 text-muted-foreground" aria-hidden />
          <span className="text-[11px] font-medium uppercase tracking-[0.08em] text-muted-foreground">Account equity</span>
        </div>
        <div className="mt-2.5 text-[48px] font-semibold leading-none tracking-tight tabular-nums sm:text-[56px]">
          <AnimatedNumber value={equity} kind="money" crumb />
        </div>
        <div className="mt-3 text-sm">
          {dayChangePct !== null ? (
            <span className={cn("font-medium", dayChangePct >= 0 ? "text-green" : "text-red")}>
              <AnimatedNumber value={dayChangePct} kind="pctSigned2" /> today
            </span>
          ) : (
            <span className="text-muted-foreground">positions plus settled cash</span>
          )}
        </div>
      </div>

      {/* Supporting KPIs: an open divided row under a hairline, not a row of tiles. */}
      <div className="mt-8 border-t border-border/60 pt-7">
        <StatStrip cols={3}>
          <Stat
            label="Invested"
            value={<AnimatedNumber value={investedPct} kind="pct0" crumb neutral />}
            icon={Gauge}
            sub={
              <span className="mt-0.5 block h-1.5 w-full max-w-[160px] overflow-hidden rounded-full bg-foreground/[0.07]">
                <span className="block h-full rounded-full bg-primary transition-[width] duration-700 ease-out" style={{ width: `${investedClamped}%` }} />
              </span>
            }
          />
          <Stat
            label="Settled cash"
            value={<AnimatedNumber value={settledCash} kind="money" crumb />}
            icon={Coins}
            sub={
              equity > 0 ? (
                <>
                  <AnimatedNumber value={(settledCash / equity) * 100} kind="pct0" neutral /> of the book
                  {unsettledCash > 0 && (
                    <>
                      {" · "}
                      <AnimatedNumber value={unsettledCash} kind="moneyInt" neutral /> settling
                    </>
                  )}
                </>
              ) : (
                "dry powder"
              )
            }
          />
          <Stat
            label="Unrealized P&L"
            value={<AnimatedNumber value={unrealized} kind="pnl" crumb />}
            tone={unrealized > 0 ? "positive" : unrealized < 0 ? "negative" : "neutral"}
            icon={ArrowUpRight}
            sub="open positions"
          />
        </StatStrip>
      </div>
    </div>
  );
}
