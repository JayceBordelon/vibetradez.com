"use client";

import { ArrowDownRight, ArrowRight, ArrowUpRight, Coins, Gauge, Wallet } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { DashboardSkeleton } from "@/components/layout/dashboard-skeleton";
import { Section } from "@/components/layout/section";
import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { AnimatedNumber } from "@/components/ui/animated-number";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { aggregateOvernight } from "@/lib/day-split";
import { repricePositions } from "@/lib/live-pricing";
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
PortfolioShell is the v2 dashboard: one personal brokerage account managed
by the agent. It shows the live stat strip (equity, invested, cash,
unrealized), the performance section (account value over time / P&L
decomposition vs buy-and-hold SPY, plus the drawdown and round-trip
quality strip), today's executions tape, and the link into the day's full
session transcript. Single-account view, not a multi-user product.
*/
export function PortfolioShell() {
  const [data, setData] = useState<PortfolioResponse | null>(null);
  const [curve, setCurve] = useState<EquityCurvePoint[]>([]);
  const [trades, setTrades] = useState<ClosedTrade[]>([]);
  // Streamed quotes re-price the book between polls, so the strip moves
  // tick by tick rather than once a minute.
  const quotes = useLiveQuotes(Boolean(data?.enabled));

  const load = useCallback(() => {
    api.getPortfolio().then(setData).catch(() => {});
    api
      .getEquityCurve()
      .then((r) => setCurve(r.points ?? []))
      .catch(() => {});
    // Closed trades feed the win-rate / profit-factor strip; polled with
    // the rest so an intraday fill (the 15-minute reconcile) shows up.
    api
      .getClosedTrades()
      .then((r) => setTrades(r.trades ?? []))
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
      <div className="mx-auto max-w-[1200px] px-4 py-6 sm:px-7">
        <DashboardSkeleton />
      </div>
    );
  }

  if (!data.enabled) {
    return (
      <div className="mx-auto max-w-[1200px] px-4 py-6 sm:px-7">
        <div className="rounded-lg border border-border bg-muted/30 px-6 py-16 text-center">
          <h2 className="text-lg font-semibold">The portfolio manager isn&apos;t running yet</h2>
          <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">
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
  const equity = positionsValue + data.settled_cash + data.unsettled_cash;
  const investedPct = equity > 0 ? (positionsValue / equity) * 100 : 0;
  // Day change measures LIVE equity against the last end-of-day close
  // BEFORE today, split phase-aware:
  //   in session: overnight = the gap that preceded the open (summed from
  //     position anchors), today = the residual (so realized intraday P&L
  //     stays included);
  //   post-close (today's REAL curve point exists): today freezes at the
  //     completed day and overnight becomes the LIVE drift since the
  //     close, the period we're actually in.
  // When neither split is anchorable, fall back to the unsplit day change.
  const baseline = [...curve].reverse().find((p) => p.date < data.date)?.account_equity ?? 0;
  const todayClosePoint = curve.find((p) => p.date === data.date && !p.live);
  const dayDollars = baseline > 0 ? equity - baseline : null;
  let todayPct: number | null = null;
  let overnightPct: number | null = null;
  if (baseline > 0 && todayClosePoint && todayClosePoint.account_equity > 0) {
    todayPct = ((todayClosePoint.account_equity - baseline) / baseline) * 100;
    overnightPct = ((equity - todayClosePoint.account_equity) / baseline) * 100;
  } else {
    const overnightDollars = aggregateOvernight(positions);
    if (baseline > 0 && dayDollars !== null && overnightDollars !== null) {
      overnightPct = (overnightDollars / baseline) * 100;
      todayPct = ((dayDollars - overnightDollars) / baseline) * 100;
    }
  }
  const dayChangePct = baseline > 0 && dayDollars !== null ? (dayDollars / baseline) * 100 : null;

  return (
    <div className="animate-in fade-in mx-auto max-w-[1200px] px-4 py-6 duration-300 sm:px-7">
      <SummaryStrip equity={equity} settledCash={data.settled_cash} unsettledCash={data.unsettled_cash} investedPct={investedPct} unrealized={totalUnrealized} todayPct={todayPct} overnightPct={overnightPct} dayChangePct={dayChangePct} />

      {/* The day's narrative, right under the numbers: "what is it
          thinking?" is the first question the dashboard gets asked, and
          the answer used to live a click away at the bottom of a very
          long transcript page. */}
      <section className="mt-6 rounded-md border border-claude-border bg-claude-light px-4 py-4 sm:px-5">
        <div className="flex items-center gap-2">
          <ClaudeLogo className="h-4 w-4" />
          <span className="font-mono text-[11px] font-bold uppercase tracking-[0.14em] text-claude">Claudia&apos;s read</span>
        </div>
        {data.stance ? (
          <p className="mt-2 text-sm leading-relaxed text-foreground/90">{data.stance}</p>
        ) : (
          <p className="mt-2 text-sm leading-relaxed text-muted-foreground">No stance yet today. The midday session (~12:30 PM ET) writes one after it looks at the book; until then the session log has yesterday&apos;s.</p>
        )}
        <a href={`/transcripts/${data.date}`} className="group mt-3 inline-flex min-h-9 items-center gap-1.5 text-xs font-semibold text-claude sm:min-h-0">
          Read the full session, tool call by tool call
          <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
        </a>
      </section>

      {/* No border-t here: the summary strip above already draws its own
          bottom rule, and the doubled hairline read as a ghost band in dark. */}
      <Section title="Performance" className="mt-8">
        <EquityCurveChart points={curve} today={data.date} liveEquity={equity} liveUnrealized={totalUnrealized} liveSpyMark={quotes.get("SPY")?.mark ?? null} />
        {curve.length >= 2 && <PerformanceMetrics points={curve} trades={trades} today={data.date} liveEquity={equity} />}
      </Section>

      <Section
        title="Today's executions"
        subtitle={todaysExecutionsSubtitle(data.decisions ?? [])}
        className="mt-8 border-t border-border/40 pt-6"
      >
        <TodaysExecutions decisions={data.decisions ?? []} />
      </Section>

      <Section title="Explore" subtitle="The full book, the trade history, and the session log live on their own pages." className="mt-8 border-t border-border/40 pt-6">
        <div className="divide-y divide-border/50 border-t border-border/50">
          <ExploreLink
            href="/holdings"
            title="Holdings"
            blurb={`${positions.length} open position${positions.length === 1 ? "" : "s"} the account holds right now.`}
          />
          <ExploreLink href="/closed" title="Closed trades" blurb="Every completed round trip, with realized P&L." />
          <ExploreLink href="/transcripts" title="Sessions" blurb="The day-by-day session log, browsable back through every trading day." />
        </div>
      </Section>
    </div>
  );
}

function ExploreLink({ href, title, blurb }: { href: string; title: string; blurb: string }) {
  return (
    <Link href={href} className="group flex items-center justify-between gap-3 py-3.5">
      <span className="min-w-0">
        <span className="block font-semibold group-hover:underline">{title}</span>
        <span className="block text-xs text-muted-foreground">{blurb}</span>
      </span>
      <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
    </Link>
  );
}

function SummaryStrip({ equity, settledCash, unsettledCash, investedPct, unrealized, todayPct, overnightPct, dayChangePct }: { equity: number; settledCash: number; unsettledCash: number; investedPct: number; unrealized: number; todayPct: number | null; overnightPct: number | null; dayChangePct: number | null }) {
  const investedClamped = Math.min(100, Math.max(0, investedPct));
  return (
    <div>
      <StatStrip cols={4}>
        <Stat
          label="Account equity"
          value={<AnimatedNumber value={equity} kind="money" crumb />}
          icon={Wallet}
          sub={
            todayPct !== null && overnightPct !== null ? (
              // The two segments stack on phones: side by side they
              // overflow the 2-up grid cell and truncate to "+3.38% o…".
              <span className="font-semibold">
                <span className={todayPct >= 0 ? "text-green" : "text-red"}>
                  <AnimatedNumber value={todayPct} kind="pctSigned2" /> today
                </span>
                <span className="text-muted-foreground max-sm:hidden"> · </span>
                <span className={cn("max-sm:block", overnightPct >= 0 ? "text-green" : "text-red")}>
                  <AnimatedNumber value={overnightPct} kind="pctSigned2" /> overnight
                </span>
              </span>
            ) : dayChangePct !== null ? (
              <span className={cn("font-semibold", dayChangePct >= 0 ? "text-green" : "text-red")}>
                <AnimatedNumber value={dayChangePct} kind="pctSigned2" /> today
              </span>
            ) : undefined
          }
        />
        <Stat
          label="Invested"
          value={<AnimatedNumber value={investedPct} kind="pct0" crumb neutral />}
          icon={Gauge}
          sub={
            <span className="mt-1 block h-1.5 w-full max-w-[140px] overflow-hidden bg-foreground/[0.08]">
              {/* The bar glides with its number: same ~700ms ease. */}
              <span className="block h-full bg-green transition-[width] duration-700 ease-out" style={{ width: `${investedClamped}%` }} />
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
                {/* T+1 money in flight is invisible otherwise, and a buy
                    that "won't fit" makes no sense until you can see it. */}
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
          icon={unrealized >= 0 ? ArrowUpRight : ArrowDownRight}
          sub="open positions"
        />
      </StatStrip>
    </div>
  );
}

