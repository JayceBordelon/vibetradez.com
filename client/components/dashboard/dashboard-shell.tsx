"use client";

// NavBar moved to app/(app)/layout.tsx
import { CalendarX } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { DashboardSkeleton } from "@/components/layout/dashboard-skeleton";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { Section } from "@/components/layout/section";
import { Separator } from "@/components/ui/separator";
import { api } from "@/lib/api";
import type { DashboardResponse, DashboardTrade, LiveQuotesResponse } from "@/types/trade";

import { ActiveTickerChip } from "./active-ticker-chip";
import { ExposurePanel } from "./exposure-panel";
import { MorningCards } from "./morning-cards";
import { PnlChart } from "./pnl-chart";
import { StatsGrid } from "./stats-grid";
import { StockChart } from "./stock-chart";
import { SymbolTabs } from "./symbol-tabs";
import { TIMEFRAME_PRESETS, type TimeframePreset, TimeframeToggle } from "./timeframe-toggle";
import { TopNFilter } from "./top-n-filter";
import { TradeTable } from "./trade-table";

const STORAGE_KEY = "jt_dash_v3";
const REFRESH_SECONDS = 60;
const LIVE_POLL_SECONDS = 15;

function filterByRank(data: DashboardResponse, topFilter: number): DashboardResponse {
  /**
  Server should always return an empty array, but be defensive in
  case any deployment regresses to nil-slice → JSON null. Mirrors
  the same defensive coalescing the history page already does.
  */
  const trades = data.trades ?? [];
  if (topFilter >= 10) return { ...data, trades };
  return {
    ...data,
    trades: trades.filter((t) => t.trade.rank >= 1 && t.trade.rank <= topFilter),
  };
}

function computeStats(trades: DashboardTrade[]) {
  let winners = 0;
  let losers = 0;
  let totalPnl = 0;
  let bestPnl = -Infinity;
  let bestSym = "";
  let grossWins = 0;
  let grossLosses = 0;
  let hasSummaries = false;

  for (const { trade, summary } of trades ?? []) {
    if (summary) {
      hasSummaries = true;
      const pnl = (summary.closing_price - summary.entry_price) * 100;
      totalPnl += pnl;
      if (pnl > 0.5) {
        winners++;
        grossWins += pnl;
      } else if (pnl < -0.5) {
        losers++;
        grossLosses += Math.abs(pnl);
      }
      if (pnl > bestPnl) {
        bestPnl = pnl;
        bestSym = trade.symbol;
      }
    }
  }

  const winRate = winners + losers > 0 ? (winners / (winners + losers)) * 100 : 0;
  const profitFactor = grossLosses > 0 ? grossWins / grossLosses : grossWins > 0 ? Number.POSITIVE_INFINITY : 0;

  return { hasSummaries, totalPnl, winRate, profitFactor, bestPnl, bestSym };
}

export function DashboardShell() {
  const [topFilter, setTopFilter] = useState(10);
  const [rawData, setRawData] = useState<DashboardResponse | null>(null);
  const [liveQuotes, setLiveQuotes] = useState<LiveQuotesResponse | null>(null);
  const [activeSymbol, setActiveSymbol] = useState("");
  const [timeframe, setTimeframe] = useState<TimeframePreset>(() => TIMEFRAME_PRESETS.find((p) => p.id === "5D") ?? TIMEFRAME_PRESETS[0]);

  // Restore Top-N from localStorage
  useEffect(() => {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw) {
        const saved = JSON.parse(raw);
        if (saved.topFilter && [1, 3, 5, 10].includes(saved.topFilter)) setTopFilter(saved.topFilter);
      }
    } catch {}
  }, []);

  // Persist Top-N
  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ topFilter }));
    } catch {}
  }, [topFilter]);

  /*
  Dashboard is today-only — historical day-by-day analysis lives on
  /history. Always fetch the current trading day; auto-refresh keeps
  the morning-mode picks fresh and flips to EOD-mode automatically
  once summaries land.
  */
  const loadDay = useCallback(() => {
    api.getTrades().then(setRawData);
  }, []);

  useEffect(() => {
    loadDay();
  }, [loadDay]);

  // Auto-refresh
  useEffect(() => {
    const interval = setInterval(loadDay, REFRESH_SECONDS * 1000);
    return () => clearInterval(interval);
  }, [loadDay]);

  // Live quotes polling
  useEffect(() => {
    if (!rawData?.trades?.length) return;
    const hasSummaries = rawData.trades.some((t) => t.summary);
    if (hasSummaries) return;

    const poll = () => api.getLiveQuotes().then(setLiveQuotes);
    poll();
    const interval = setInterval(poll, LIVE_POLL_SECONDS * 1000);
    return () => clearInterval(interval);
  }, [rawData]);

  /*
  hasSummaries is computed off the unfiltered raw data so it stays
  stable regardless of TopN selection. On live (morning) days the
  Top-N filter and timeframe toggle are hidden because the only
  meaningful view is "today's 10 picks on a 1D chart" — anything
  else is noise before EOD lands.
  */
  const hasSummaries = !!rawData?.trades?.some((t) => t.summary);
  const effectiveTopFilter = hasSummaries ? topFilter : 10;
  const filtered = rawData ? filterByRank(rawData, effectiveTopFilter) : null;
  const stats = filtered?.trades ? computeStats(filtered.trades) : null;
  const liveTimeframe = TIMEFRAME_PRESETS[0];

  // Set first symbol when data loads
  useEffect(() => {
    if (filtered?.trades?.length && !activeSymbol) {
      setActiveSymbol(filtered.trades[0].trade.symbol);
    }
  }, [filtered, activeSymbol]);

  return (
    <div className="animate-in fade-in duration-300">
      {hasSummaries && filtered?.trades?.length ? (
        <div className="hidden sm:block">
          <PageToolbar rightSlot={<TopNFilter value={topFilter} onChange={setTopFilter} />} />
        </div>
      ) : null}

      <div className="mx-auto max-w-[1200px] px-4 py-6 sm:px-7">
        {hasSummaries && filtered?.trades?.length ? (
          <div className="mb-4 flex items-center justify-end gap-2 sm:hidden">
            <TopNFilter value={topFilter} onChange={setTopFilter} />
          </div>
        ) : null}
        {!rawData ? (
          <DashboardSkeleton />
        ) : !filtered?.trades?.length ? (
          <EmptyState />
        ) : stats?.hasSummaries ? (
          <>
            <StatsGrid totalPnl={stats.totalPnl} winRate={stats.winRate} profitFactor={stats.profitFactor} bestPnl={stats.bestPnl} bestSym={stats.bestSym} />
            <Section title="Price Chart" className="mt-8" actions={<ActiveTickerChip trades={filtered.trades} activeSymbol={activeSymbol} />}>
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
                <SymbolTabs trades={filtered.trades} activeSymbol={activeSymbol} onSelect={setActiveSymbol} />
                <TimeframeToggle value={timeframe.id} onChange={setTimeframe} />
              </div>
              <div className="lg-card mt-3 h-[280px] overflow-hidden p-1.5 sm:h-[360px] lg:h-[420px]">
                {activeSymbol &&
                  (() => {
                    const dt = filtered.trades.find((t) => t.trade.symbol === activeSymbol);
                    return <StockChart symbol={activeSymbol} timeframe={timeframe.params} strikePrice={dt?.trade.strike_price} trade={dt?.trade} summary={dt?.summary ?? undefined} />;
                  })()}
              </div>
            </Section>
            <Section title="Exposure" subtitle="How capital was deployed today. For long options, max loss is the premium paid.">
              <ExposurePanel trades={filtered.trades} hasSummaries />
            </Section>
            <Section title="P&L by Trade" subtitle="Per-contract performance, sorted">
              <PnlChart trades={filtered.trades} />
            </Section>
            <Separator />
            <Section title="Trade Details" subtitle="Click any row to view the single-contract page">
              <TradeTable trades={filtered.trades} date={filtered.date} />
            </Section>
          </>
        ) : (
          <>
            <Section title="Price Chart" actions={<ActiveTickerChip trades={filtered.trades} activeSymbol={activeSymbol} />}>
              <SymbolTabs trades={filtered.trades} activeSymbol={activeSymbol} onSelect={setActiveSymbol} />
              <div className="lg-card mt-3 h-[280px] overflow-hidden p-1.5 sm:h-[360px] lg:h-[420px]">
                {activeSymbol &&
                  (() => {
                    const dt = filtered.trades.find((t) => t.trade.symbol === activeSymbol);
                    return <StockChart symbol={activeSymbol} timeframe={liveTimeframe.params} strikePrice={dt?.trade.strike_price} trade={dt?.trade} summary={dt?.summary ?? undefined} />;
                  })()}
              </div>
            </Section>
            <Section title="Exposure" subtitle="Capital at risk for today's picks. For long options, max loss is the premium paid.">
              <ExposurePanel trades={filtered.trades} hasSummaries={false} />
            </Section>
            <Section title="Today's Picks" subtitle={`${filtered.trades.length} ranked plays · click any pick for the full single-contract view`}>
              <MorningCards trades={filtered.trades} liveQuotes={liveQuotes} date={filtered.date} execution={filtered.execution} />
            </Section>
          </>
        )}
      </div>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <CalendarX className="h-12 w-12 text-muted-foreground/50" />
      <h3 className="mt-4 text-base font-semibold">No trades for this date</h3>
      <p className="mt-1 text-sm text-muted-foreground">Trades publish at 9:25 AM ET on market days.</p>
    </div>
  );
}
