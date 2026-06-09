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
import type { EquityCurvePoint, PortfolioResponse } from "@/types/portfolio";

import { EquityCurveChart } from "./equity-curve-chart";
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
unrealized), the dollar P&L decomposition vs buy-and-hold SPY (realized +
unrealized + the SPY ghost), today's executions tape, and the link into
the day's full session transcript. Single-account view, not a multi-user
product.
*/
export function PortfolioShell() {
  const [data, setData] = useState<PortfolioResponse | null>(null);
  const [curve, setCurve] = useState<EquityCurvePoint[]>([]);
  // Streamed quotes re-price the book between polls, so the strip moves
  // tick by tick rather than once a minute.
  const quotes = useLiveQuotes(Boolean(data?.enabled));

  const load = useCallback(() => {
    api.getPortfolio().then(setData).catch(() => {});
    api
      .getEquityCurve()
      .then((r) => setCurve(r.points ?? []))
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
  // BEFORE today, split into the overnight gap (prior close to today's
  // open, summed from the position anchors) and the session move (the
  // residual, so realized intraday P&L stays included). When no position
  // can anchor an overnight (quote source down), fall back to the unsplit
  // day change.
  const baseline = [...curve].reverse().find((p) => p.date < data.date)?.account_equity ?? 0;
  const overnightDollars = aggregateOvernight(positions);
  const dayDollars = baseline > 0 ? equity - baseline : null;
  const overnightPct = baseline > 0 && overnightDollars !== null ? (overnightDollars / baseline) * 100 : null;
  const todayPct = baseline > 0 && dayDollars !== null && overnightDollars !== null ? ((dayDollars - overnightDollars) / baseline) * 100 : null;
  const dayChangePct = baseline > 0 && dayDollars !== null ? (dayDollars / baseline) * 100 : null;

  return (
    <div className="animate-in fade-in mx-auto max-w-[1200px] px-4 py-6 duration-300 sm:px-7">
      <SummaryStrip equity={equity} settledCash={data.settled_cash} investedPct={investedPct} unrealized={totalUnrealized} todayPct={todayPct} overnightPct={overnightPct} dayChangePct={dayChangePct} />

      {/* No border-t here: the summary strip above already draws its own
          bottom rule, and the doubled hairline read as a ghost band in dark. */}
      <Section title="P&L vs SPY" subtitle="Realized is booked round trips, unrealized is the open book's mark, and the dashed line is what buy-and-hold SPY would have earned on the same starting equity." className="mt-8">
        <EquityCurveChart points={curve} today={data.date} liveUnrealized={totalUnrealized} liveSpyMark={quotes.get("SPY")?.mark ?? null} />
      </Section>

      <Section
        title="Today's executions"
        subtitle={todaysExecutionsSubtitle(data.decisions ?? [])}
        className="mt-8 border-t border-border/40 pt-6"
      >
        <TodaysExecutions decisions={data.decisions ?? []} />
      </Section>

      {/* The session's own synopsis and action items live on the transcript
          page; the dashboard links there rather than duplicating them. */}
      <a href={`/transcripts/${data.date}`} className="group mt-6 flex items-center gap-3 border-t border-border/50 pt-5">
        <ClaudeLogo className="h-5 w-5 shrink-0" />
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-semibold text-foreground transition-colors group-hover:text-claude">Open today&apos;s full session</span>
          <span className="block text-xs text-muted-foreground">Every quote and chart Claude read, the reasoning in between, and every order it placed, tool call by tool call.</span>
        </span>
        <ArrowRight className="h-4 w-4 shrink-0 text-claude transition-transform group-hover:translate-x-0.5" />
      </a>

      <Section title="Explore" subtitle="The full book and the complete trade history live on their own pages." className="mt-8 border-t border-border/40 pt-6">
        <div className="divide-y divide-border/50 border-t border-border/50">
          <ExploreLink
            href="/holdings"
            title="Holdings"
            blurb={`${positions.length} open position${positions.length === 1 ? "" : "s"} the account holds right now.`}
          />
          <ExploreLink href="/closed" title="Closed trades" blurb="Every completed round trip, with realized P&L." />
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

function SummaryStrip({ equity, settledCash, investedPct, unrealized, todayPct, overnightPct, dayChangePct }: { equity: number; settledCash: number; investedPct: number; unrealized: number; todayPct: number | null; overnightPct: number | null; dayChangePct: number | null }) {
  const investedClamped = Math.min(100, Math.max(0, investedPct));
  return (
    <div>
      <StatStrip cols={4}>
        <Stat
          label="Account equity"
          value={<AnimatedNumber value={equity} kind="money" />}
          icon={Wallet}
          sub={
            todayPct !== null && overnightPct !== null ? (
              <span className="font-semibold">
                <span className={todayPct >= 0 ? "text-green" : "text-red"}>
                  <AnimatedNumber value={todayPct} kind="pctSigned2" /> today
                </span>
                <span className="text-muted-foreground"> · </span>
                <span className={overnightPct >= 0 ? "text-green" : "text-red"}>
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
          value={<AnimatedNumber value={investedPct} kind="pct0" />}
          icon={Gauge}
          sub={
            <span className="mt-1 block h-1.5 w-full max-w-[140px] overflow-hidden rounded-full bg-foreground/[0.08]">
              {/* The bar glides with its number: same ~700ms ease. */}
              <span className="block h-full rounded-full bg-gradient-brand transition-[width] duration-700 ease-out" style={{ width: `${investedClamped}%` }} />
            </span>
          }
        />
        <Stat
          label="Settled cash"
          value={<AnimatedNumber value={settledCash} kind="money" />}
          icon={Coins}
          sub={
            equity > 0 ? (
              <>
                <AnimatedNumber value={(settledCash / equity) * 100} kind="pct0" /> of the book
              </>
            ) : (
              "dry powder"
            )
          }
        />
        <Stat
          label="Unrealized P&L"
          value={<AnimatedNumber value={unrealized} kind="pnl" />}
          tone={unrealized > 0 ? "positive" : unrealized < 0 ? "negative" : "neutral"}
          icon={unrealized >= 0 ? ArrowUpRight : ArrowDownRight}
          sub="open positions"
        />
      </StatStrip>
    </div>
  );
}

