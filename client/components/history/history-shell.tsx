"use client";

import { BarChart3 } from "lucide-react";
import { useEffect, useState } from "react";

import { HistorySkeleton } from "@/components/layout/dashboard-skeleton";
import { Section } from "@/components/layout/section";
import { Separator } from "@/components/ui/separator";
import { api } from "@/lib/api";
import { computeTradePnl } from "@/lib/calculations";
import { toDateStr } from "@/lib/date-utils";
import type { WeekResponse } from "@/types/trade";

import { CapitalEfficiency } from "./capital-efficiency";
import { DailyBreakdown } from "./daily-breakdown";
import { DailyPnlChart } from "./daily-pnl-chart";
import { EquityCurveChart } from "./equity-curve-chart";
import { ExposureReturnsChart } from "./exposure-returns-chart";
import { HistoryStats } from "./history-stats";

const ALL_TIME_START = "2020-01-01";

type DayStat = {
  date: string;
  pnl: number;
  winners: number;
  losers: number;
  trades: number;
  hasSummaries: boolean;
  invested: number;
  returned: number;
  executions: import("@/types/trade").Execution[];
  details: {
    symbol: string;
    type: string;
    strike: number;
    entry: number;
    close: number;
    pnl: number;
    pct: number;
    result: string;
  }[];
};

function computeAggregates(data: WeekResponse) {
  let totalPnl = 0;
  let totalWinners = 0;
  let totalLosers = 0;
  let totalTrades = 0;
  let totalInvested = 0;
  let totalReturn = 0;
  let grossWins = 0;
  let grossLosses = 0;
  let bestPnl = -Infinity;
  let worstPnl = Infinity;
  let bestSym = "";
  let worstSym = "";
  let peakEquity = 0;
  let maxDrawdown = 0;
  let cumPnl = 0;
  const dailyReturns: number[] = [];
  const equityPoints: { date: string; cumPnl: number }[] = [];
  const dayStats: DayStat[] = [];

  for (const day of data.days ?? []) {
    let dayPnl = 0;
    let dayW = 0;
    let dayL = 0;
    let dayHasSummaries = false;
    let dayInvested = 0;
    let dayReturned = 0;
    const details: DayStat["details"] = [];

    for (const dt of day.trades ?? []) {
      const { trade } = dt;
      totalTrades++;
      const result = computeTradePnl(dt, day.executions ?? null);
      if (result.hasData) {
        dayHasSummaries = true;
        const pnl = result.pnl;
        const pct = result.pctChange;
        dayPnl += pnl;
        totalPnl += pnl;
        dayInvested += result.entryPrice * 100;
        dayReturned += result.closingPrice * 100;
        totalInvested += result.entryPrice * 100;
        totalReturn += result.closingPrice * 100;

        if (pnl > 0.5) {
          dayW++;
          totalWinners++;
          grossWins += pnl;
        } else if (pnl < -0.5) {
          dayL++;
          totalLosers++;
          grossLosses += Math.abs(pnl);
        }
        if (pnl > bestPnl) {
          bestPnl = pnl;
          bestSym = trade.symbol;
        }
        if (pnl < worstPnl) {
          worstPnl = pnl;
          worstSym = trade.symbol;
        }

        details.push({
          symbol: trade.symbol,
          type: trade.contract_type,
          strike: trade.strike_price,
          entry: result.entryPrice,
          close: result.closingPrice,
          pnl,
          pct,
          result: pnl > 0.5 ? "profit" : pnl < -0.5 ? "loss" : "flat",
        });
      }
    }

    cumPnl += dayPnl;
    if (cumPnl > peakEquity) peakEquity = cumPnl;
    if (peakEquity > 0) {
      const dd = ((peakEquity - cumPnl) / peakEquity) * 100;
      if (dd > maxDrawdown) maxDrawdown = dd;
    }
    if (dayHasSummaries && dayInvested > 0) dailyReturns.push(dayPnl / dayInvested);

    equityPoints.push({ date: day.date, cumPnl });
    dayStats.push({
      date: day.date,
      pnl: dayPnl,
      winners: dayW,
      losers: dayL,
      trades: day.trades.length,
      hasSummaries: dayHasSummaries,
      invested: dayInvested,
      returned: dayReturned,
      executions: day.executions ?? [],
      details,
    });
  }

  const winRate = totalWinners + totalLosers > 0 ? (totalWinners / (totalWinners + totalLosers)) * 100 : 0;
  const profitFactor = grossLosses > 0 ? grossWins / grossLosses : grossWins > 0 ? Number.POSITIVE_INFINITY : 0;
  const avgWin = totalWinners > 0 ? grossWins / totalWinners : 0;
  const avgLoss = totalLosers > 0 ? grossLosses / totalLosers : 0;
  const expectancy = (winRate / 100) * avgWin - (1 - winRate / 100) * avgLoss;
  const roc = totalInvested > 0 ? (totalPnl / totalInvested) * 100 : 0;

  let sharpe = 0;
  if (dailyReturns.length > 1) {
    const mean = dailyReturns.reduce((a, b) => a + b, 0) / dailyReturns.length;
    const variance = dailyReturns.reduce((a, r) => a + (r - mean) ** 2, 0) / (dailyReturns.length - 1);
    const stddev = Math.sqrt(variance);
    if (stddev > 0) sharpe = (mean / stddev) * Math.sqrt(252);
  }

  return {
    totalPnl,
    totalWinners,
    totalLosers,
    totalTrades,
    totalInvested,
    totalReturn,
    winRate,
    profitFactor,
    avgWin,
    avgLoss,
    expectancy,
    roc,
    sharpe,
    maxDrawdown,
    bestPnl: bestPnl === -Infinity ? 0 : bestPnl,
    bestSym,
    worstPnl: worstPnl === Infinity ? 0 : worstPnl,
    worstSym,
    equityPoints,
    dayStats,
  };
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <BarChart3 className="h-12 w-12 text-muted-foreground/60" aria-hidden />
      <h3 className="mt-4 text-base font-semibold">No trades yet</h3>
      <p className="mt-1 text-sm text-muted-foreground">Once trades land, you'll see them here.</p>
    </div>
  );
}

export function HistoryShell() {
  const [rawData, setRawData] = useState<WeekResponse | null>(null);

  useEffect(() => {
    api
      .getWeekTrades(ALL_TIME_START, toDateStr(new Date()))
      .then(setRawData)
      .catch(() => {});
  }, []);

  const agg = rawData?.days?.length ? computeAggregates(rawData) : null;
  const daysWithPnl = agg?.dayStats.filter((d) => d.hasSummaries) ?? [];

  return (
    <div className="animate-in fade-in duration-300">
      <div className="mx-auto max-w-[1200px] px-4 py-6 sm:px-7">
        {!rawData ? (
          <HistorySkeleton />
        ) : !rawData.days?.length ? (
          <EmptyState />
        ) : agg ? (
          <>
            {agg.totalWinners + agg.totalLosers > 0 && (
              <>
                <HistoryStats {...agg} />

                {agg.equityPoints.length > 1 && (
                  <Section className="mt-8">
                    <EquityCurveChart data={agg.equityPoints} />
                  </Section>
                )}

                {daysWithPnl.length > 1 && (
                  <Section>
                    <DailyPnlChart data={daysWithPnl} granularity="weekly" />
                  </Section>
                )}

                {daysWithPnl.length > 1 && (
                  <Section className="mt-6">
                    <ExposureReturnsChart data={daysWithPnl} />
                  </Section>
                )}

                <Section className="mt-6">
                  <CapitalEfficiency totalInvested={agg.totalInvested} totalReturn={agg.totalReturn} totalPnl={agg.totalPnl} roc={agg.roc} />
                </Section>
              </>
            )}

            <Separator className="my-6" />

            <Section title="Daily Breakdown" subtitle="Click any day to see individual trades">
              <DailyBreakdown dayStats={agg.dayStats} />
            </Section>
          </>
        ) : null}
      </div>
    </div>
  );
}
