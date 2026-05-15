"use client";

import { BarChart3 } from "lucide-react";
import { useEffect, useState } from "react";

import { HistorySkeleton } from "@/components/layout/dashboard-skeleton";
import { Section } from "@/components/layout/section";
import { api } from "@/lib/api";
import { computeTradePnl } from "@/lib/calculations";
import { toDateStr } from "@/lib/date-utils";
import type { Execution, WeekResponse } from "@/types/trade";

import { CapitalEfficiency } from "./capital-efficiency";
import { DailyBreakdown } from "./daily-breakdown";
import { DailyPnlChart } from "./daily-pnl-chart";
import { EquityCurveChart } from "./equity-curve-chart";
import { ExposureReturnsChart } from "./exposure-returns-chart";
import { HistoryStats } from "./history-stats";
import { inTier, TIER_KEYS, type TierKey } from "./tiers";

const ALL_TIME_START = "2020-01-01";

export interface TradeDetail {
  symbol: string;
  type: string;
  strike: number;
  entry: number;
  close: number;
  pnl: number;
  pct: number;
  rank: number;
  result: string;
}

export interface DayTierStat {
  pnl: number;
  winners: number;
  losers: number;
  trades: number;
  hasSummaries: boolean;
  invested: number;
  returned: number;
  cumPnl: number;
  details: TradeDetail[];
}

export interface DayMultiStat {
  date: string;
  executions: Execution[];
  totalTrades: number;
  tiers: Record<TierKey, DayTierStat>;
}

export interface TierAggregate {
  totalPnl: number;
  totalWinners: number;
  totalLosers: number;
  totalTrades: number;
  totalInvested: number;
  totalReturn: number;
  winRate: number;
  profitFactor: number;
  avgWin: number;
  avgLoss: number;
  expectancy: number;
  roc: number;
  sharpe: number;
  maxDrawdown: number;
  bestPnl: number;
  bestSym: string;
  worstPnl: number;
  worstSym: string;
}

export interface AggregateResult {
  tiers: Record<TierKey, TierAggregate>;
  days: DayMultiStat[];
}

function emptyTierAccumulator() {
  return {
    totalPnl: 0,
    totalWinners: 0,
    totalLosers: 0,
    totalTrades: 0,
    totalInvested: 0,
    totalReturn: 0,
    grossWins: 0,
    grossLosses: 0,
    bestPnl: Number.NEGATIVE_INFINITY,
    bestSym: "",
    worstPnl: Number.POSITIVE_INFINITY,
    worstSym: "",
    peakEquity: 0,
    maxDrawdown: 0,
    cumPnl: 0,
    dailyReturns: [] as number[],
  };
}

function emptyDayTier(): DayTierStat {
  return {
    pnl: 0,
    winners: 0,
    losers: 0,
    trades: 0,
    hasSummaries: false,
    invested: 0,
    returned: 0,
    cumPnl: 0,
    details: [],
  };
}

function finalizeTier(acc: ReturnType<typeof emptyTierAccumulator>): TierAggregate {
  const winRate = acc.totalWinners + acc.totalLosers > 0 ? (acc.totalWinners / (acc.totalWinners + acc.totalLosers)) * 100 : 0;
  const profitFactor = acc.grossLosses > 0 ? acc.grossWins / acc.grossLosses : acc.grossWins > 0 ? Number.POSITIVE_INFINITY : 0;
  const avgWin = acc.totalWinners > 0 ? acc.grossWins / acc.totalWinners : 0;
  const avgLoss = acc.totalLosers > 0 ? acc.grossLosses / acc.totalLosers : 0;
  const expectancy = (winRate / 100) * avgWin - (1 - winRate / 100) * avgLoss;
  const roc = acc.totalInvested > 0 ? (acc.totalPnl / acc.totalInvested) * 100 : 0;

  let sharpe = 0;
  if (acc.dailyReturns.length > 1) {
    const mean = acc.dailyReturns.reduce((a, b) => a + b, 0) / acc.dailyReturns.length;
    const variance = acc.dailyReturns.reduce((a, r) => a + (r - mean) ** 2, 0) / (acc.dailyReturns.length - 1);
    const stddev = Math.sqrt(variance);
    if (stddev > 0) sharpe = (mean / stddev) * Math.sqrt(252);
  }

  return {
    totalPnl: acc.totalPnl,
    totalWinners: acc.totalWinners,
    totalLosers: acc.totalLosers,
    totalTrades: acc.totalTrades,
    totalInvested: acc.totalInvested,
    totalReturn: acc.totalReturn,
    winRate,
    profitFactor,
    avgWin,
    avgLoss,
    expectancy,
    roc,
    sharpe,
    maxDrawdown: acc.maxDrawdown,
    bestPnl: acc.bestPnl === Number.NEGATIVE_INFINITY ? 0 : acc.bestPnl,
    bestSym: acc.bestSym,
    worstPnl: acc.worstPnl === Number.POSITIVE_INFINITY ? 0 : acc.worstPnl,
    worstSym: acc.worstSym,
  };
}

function computeAggregates(data: WeekResponse): AggregateResult {
  const tierAcc = {
    top1: emptyTierAccumulator(),
    top3: emptyTierAccumulator(),
    top10: emptyTierAccumulator(),
  } satisfies Record<TierKey, ReturnType<typeof emptyTierAccumulator>>;

  const days: DayMultiStat[] = [];

  for (const day of data.days ?? []) {
    const dayTiers: Record<TierKey, DayTierStat> = {
      top1: emptyDayTier(),
      top3: emptyDayTier(),
      top10: emptyDayTier(),
    };

    for (const dt of day.trades ?? []) {
      const { trade } = dt;
      const result = computeTradePnl(dt, day.executions ?? null);

      for (const tier of TIER_KEYS) {
        if (!inTier(trade.rank, tier)) continue;

        const acc = tierAcc[tier];
        const dayT = dayTiers[tier];
        dayT.trades++;
        acc.totalTrades++;

        if (!result.hasData) continue;

        const pnl = result.pnl;
        const pct = result.pctChange;
        dayT.hasSummaries = true;
        dayT.pnl += pnl;
        dayT.invested += result.entryPrice * 100;
        dayT.returned += result.closingPrice * 100;
        acc.totalPnl += pnl;
        acc.totalInvested += result.entryPrice * 100;
        acc.totalReturn += result.closingPrice * 100;

        if (pnl > 0.5) {
          dayT.winners++;
          acc.totalWinners++;
          acc.grossWins += pnl;
        } else if (pnl < -0.5) {
          dayT.losers++;
          acc.totalLosers++;
          acc.grossLosses += Math.abs(pnl);
        }
        if (pnl > acc.bestPnl) {
          acc.bestPnl = pnl;
          acc.bestSym = trade.symbol;
        }
        if (pnl < acc.worstPnl) {
          acc.worstPnl = pnl;
          acc.worstSym = trade.symbol;
        }

        dayT.details.push({
          symbol: trade.symbol,
          type: trade.contract_type,
          strike: trade.strike_price,
          entry: result.entryPrice,
          close: result.closingPrice,
          pnl,
          pct,
          rank: trade.rank,
          result: pnl > 0.5 ? "profit" : pnl < -0.5 ? "loss" : "flat",
        });
      }
    }

    for (const tier of TIER_KEYS) {
      const acc = tierAcc[tier];
      const dayT = dayTiers[tier];
      acc.cumPnl += dayT.pnl;
      if (acc.cumPnl > acc.peakEquity) acc.peakEquity = acc.cumPnl;
      if (acc.peakEquity > 0) {
        const dd = ((acc.peakEquity - acc.cumPnl) / acc.peakEquity) * 100;
        if (dd > acc.maxDrawdown) acc.maxDrawdown = dd;
      }
      if (dayT.hasSummaries && dayT.invested > 0) acc.dailyReturns.push(dayT.pnl / dayT.invested);
      dayT.cumPnl = acc.cumPnl;
    }

    days.push({
      date: day.date,
      executions: day.executions ?? [],
      totalTrades: day.trades?.length ?? 0,
      tiers: dayTiers,
    });
  }

  return {
    tiers: {
      top1: finalizeTier(tierAcc.top1),
      top3: finalizeTier(tierAcc.top3),
      top10: finalizeTier(tierAcc.top10),
    },
    days,
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
  const top10 = agg?.tiers.top10;
  const daysWithAnyPnl = agg?.days.filter((d) => TIER_KEYS.some((t) => d.tiers[t].hasSummaries)) ?? [];
  const hasSettled = (top10?.totalWinners ?? 0) + (top10?.totalLosers ?? 0) > 0;

  return (
    <div className="animate-in fade-in duration-300">
      <div className="mx-auto max-w-[1200px] px-4 py-6 sm:px-7">
        {!rawData ? (
          <HistorySkeleton />
        ) : !rawData.days?.length ? (
          <EmptyState />
        ) : agg && top10 ? (
          <>
            {hasSettled && (
              <>
                <HistoryStats {...top10} />

                {agg.days.length > 1 && (
                  <Section title="Equity Curve" subtitle="Cumulative P&L over time, split by pick tier" className="mt-10 border-t border-border/40 pt-8">
                    <EquityCurveChart days={agg.days} />
                  </Section>
                )}

                {daysWithAnyPnl.length > 1 && (
                  <Section title="Weekly P&L" subtitle="Net P&L per trading week, by tier" className="mt-10 border-t border-border/40 pt-8">
                    <DailyPnlChart days={daysWithAnyPnl} granularity="weekly" />
                  </Section>
                )}

                {daysWithAnyPnl.length > 1 && (
                  <Section title="Exposure vs Returns" subtitle="Capital deployed compared to capital returned, per tier" className="mt-10 border-t border-border/40 pt-8">
                    <ExposureReturnsChart days={daysWithAnyPnl} />
                  </Section>
                )}

                <Section title="Capital Efficiency" subtitle="How efficiently capital has been deployed all-time, per tier" className="mt-10 border-t border-border/40 pt-8">
                  <CapitalEfficiency tiers={agg.tiers} />
                </Section>
              </>
            )}

            <Section title="Daily Breakdown" subtitle="Click any day to see individual trades grouped by tier" className="mt-10 border-t border-border/40 pt-8">
              <DailyBreakdown days={agg.days} />
            </Section>
          </>
        ) : null}
      </div>
    </div>
  );
}
