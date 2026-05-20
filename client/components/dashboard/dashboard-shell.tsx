"use client";

// NavBar moved to app/(app)/layout.tsx
import { useCallback, useEffect, useState } from "react";

import { DashboardSkeleton } from "@/components/layout/dashboard-skeleton";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { Section } from "@/components/layout/section";
import { useMarketStatus } from "@/hooks/use-market-status";
import { useQuoteStream } from "@/hooks/use-quote-stream";
import { useVisiblePoll } from "@/hooks/use-visible-poll";
import { api } from "@/lib/api";
import { computeTradePnl } from "@/lib/calculations";
import type { DashboardResponse, DashboardTrade, Execution } from "@/types/trade";

import { ExposurePanel } from "./exposure-panel";
import { MarketClosedPage } from "./market-closed-page";
import { MorningLayout } from "./morning-layout";
import { NoTradesToday } from "./no-trades-today";
import { PnlChart } from "./pnl-chart";
import { StatsGrid } from "./stats-grid";
import { TopNFilter } from "./top-n-filter";
import { TradeTable } from "./trade-table";

const STORAGE_KEY = "jt_dash_v3";
const REFRESH_SECONDS = 60;

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

function computeStats(trades: DashboardTrade[], executions: Execution[] | null | undefined) {
  let winners = 0;
  let losers = 0;
  let totalPnl = 0;
  let bestPnl = -Infinity;
  let bestSym = "";
  let grossWins = 0;
  let grossLosses = 0;
  let hasSummaries = false;

  for (const dt of trades ?? []) {
    const result = computeTradePnl(dt, executions);
    if (!result.hasData) continue;
    hasSummaries = true;
    const pnl = result.pnl;
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
      bestSym = dt.trade.symbol;
    }
  }

  const winRate = winners + losers > 0 ? (winners / (winners + losers)) * 100 : 0;
  const profitFactor = grossLosses > 0 ? grossWins / grossLosses : grossWins > 0 ? Number.POSITIVE_INFINITY : 0;

  return { hasSummaries, totalPnl, winRate, profitFactor, bestPnl, bestSym };
}

export function DashboardShell() {
  const [topFilter, setTopFilter] = useState(10);
  const [rawData, setRawData] = useState<DashboardResponse | null>(null);
  const marketStatus = useMarketStatus();

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
    api
      .getTrades()
      .then(setRawData)
      .catch(() => {
        /*
        Swallow transient fetch failures so a single 5xx doesn't
        silence the auto-refresh chain. The existing rawData stays
        rendered; the next interval will retry.
        */
      });
  }, []);

  // Auto-refresh: pauses when the tab is hidden, immediate refresh on return.
  // Disabled when the market is closed — the page is just the closed-page
  // CTA, no point polling.
  useVisiblePoll(loadDay, REFRESH_SECONDS * 1000, marketStatus?.open !== false);

  /*
  Live quotes via SSE stream. Each tick lands one symbol or one
  option_key at a time, so the dashboard rows light up progressively
  as Schwab pushes data, not all-at-once on a 15s poll. The stream
  is only opened when:
    - Today's picks are loaded
    - At least one pick is still mid-day (no summaries yet)
    - The market is actually open (server returns 503 otherwise)
  */
  const liveEnabled = !!rawData?.trades?.length && !rawData.trades.some((t) => t.summary) && marketStatus?.open === true;
  const liveQuotes = useQuoteStream(liveEnabled);

  /*
  hasSummaries is computed off the unfiltered raw data so it stays
  stable regardless of TopN selection. On live (morning) days the
  Top-N filter is hidden because the only meaningful view is
  "today's 10 picks" — anything else is noise before EOD lands.
  */
  const hasSummaries = !!rawData?.trades?.some((t) => t.summary);
  const effectiveTopFilter = hasSummaries ? topFilter : 10;
  const filtered = rawData ? filterByRank(rawData, effectiveTopFilter) : null;
  const executions = rawData?.executions ?? null;
  const stats = filtered?.trades ? computeStats(filtered.trades, executions) : null;

  /*
  Market closed → render the closed-page full-bleed. No SSE stream,
  no auto-refresh, no skeleton spinner. The page links to /history,
  which is DB-only and the right home for "what did the model do
  recently?" while the live dashboard is dark.

  marketStatus is null on first render (mid-fetch); we show the
  loading state then so we don't briefly flash the closed-page on a
  page where the market is actually open.
  */
  if (marketStatus && !marketStatus.open) {
    return (
      <div className="animate-in fade-in duration-300">
        <MarketClosedPage status={marketStatus} />
      </div>
    );
  }

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
        {!rawData || !marketStatus ? (
          <DashboardSkeleton />
        ) : !filtered?.trades?.length ? (
          <NoTradesToday />
        ) : stats?.hasSummaries ? (
          <>
            <StatsGrid totalPnl={stats.totalPnl} winRate={stats.winRate} profitFactor={stats.profitFactor} bestPnl={stats.bestPnl} bestSym={stats.bestSym} />
            <Section title="P&L by Trade" subtitle="Per-contract performance, sorted" className="mt-10 border-t border-border/40 pt-8">
              <PnlChart trades={filtered.trades} executions={executions} />
            </Section>
            <Section title="Trade Details" subtitle="Click any row to view the single-contract page" className="mt-10 border-t border-border/40 pt-8">
              <TradeTable trades={filtered.trades} executions={executions} date={filtered.date} liveQuotes={liveQuotes} />
            </Section>
            <Section title="Exposure" subtitle="How capital was deployed today. For long options, max loss is the premium paid." className="mt-10 border-t border-border/40 pt-8">
              <ExposurePanel trades={filtered.trades} executions={executions} hasSummaries />
            </Section>
          </>
        ) : (
          <>
            <Section title="Today's Picks" subtitle={`${filtered.trades.length} ranked plays · click any pick for the full single-contract view`}>
              <MorningLayout trades={filtered.trades} liveQuotes={liveQuotes} date={filtered.date} executions={executions} />
            </Section>
            <Section title="Exposure" subtitle="Capital at risk for today's picks. For long options, max loss is the premium paid." className="mt-10 border-t border-border/40 pt-8">
              {/*
              Pass executions (not null) to ExposurePanel on the morning
              branch too. exposure-panel.tsx multiplies premium by
              executionQuantity(exec); null collapses every pick to qty=1,
              so a greedy-fill morning that landed qty=3 on rank 1 showed
              ~33% of the real Capital at Risk on the dashboard pre-EOD.
              The EOD branch above already passes executions correctly.
              */}
              <ExposurePanel trades={filtered.trades} executions={executions} hasSummaries={false} />
            </Section>
          </>
        )}
      </div>
    </div>
  );
}
