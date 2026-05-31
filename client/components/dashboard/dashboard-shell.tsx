"use client";

// NavBar moved to app/(app)/layout.tsx
import { useCallback, useEffect, useState } from "react";

import { DashboardSkeleton } from "@/components/layout/dashboard-skeleton";
import { Section } from "@/components/layout/section";
import { useMarketStatus } from "@/hooks/use-market-status";
import { useQuoteStream } from "@/hooks/use-quote-stream";
import { useVisiblePoll } from "@/hooks/use-visible-poll";
import { api } from "@/lib/api";
import { computeTradePnl } from "@/lib/calculations";
import type { DashboardResponse, DashboardTrade, Execution } from "@/types/trade";

import { ExposurePanel } from "./exposure-panel";
import { MarketClosedBanner } from "./market-closed-banner";
import { MorningLayout } from "./morning-layout";
import { NoTradesToday } from "./no-trades-today";
import { PnlChart } from "./pnl-chart";
import { StatsGrid } from "./stats-grid";
import { TradeTable } from "./trade-table";

const REFRESH_SECONDS = 60;

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
  const [rawData, setRawData] = useState<DashboardResponse | null>(null);
  const marketStatus = useMarketStatus();

  /*
  Dashboard is today-only, historical day-by-day analysis lives on
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

  // Auto-refresh while the market is open: pauses when the tab is hidden,
  // immediate refresh on return. No recurring poll when closed, the last
  // trading day's data is static until the next session.
  useVisiblePoll(loadDay, REFRESH_SECONDS * 1000, marketStatus?.open === true);

  // When the market is closed we still load the most recent trading day
  // once (server resolves it via GetLatestTradeDate) so the dashboard
  // shows real picks + EOD outcomes behind a "closed" banner instead of
  // a blank placeholder. One-shot, no interval.
  useEffect(() => {
    if (marketStatus && !marketStatus.open) loadDay();
  }, [marketStatus, loadDay]);

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
  Server should always return an empty array, but be defensive in
  case any deployment regresses to nil-slice → JSON null. Mirrors
  the same defensive coalescing the history page already does.
  */
  const trades = rawData?.trades ?? [];
  const executions = rawData?.executions ?? null;
  const stats = trades.length ? computeStats(trades, executions) : null;

  /*
  Market closed → don't blank the page. We render the most recent
  trading day's picks + EOD outcomes (loaded above) behind a slim
  banner that says the data is the last session, not live. No SSE
  stream and no recurring poll while dark.

  marketStatus is null on first render (mid-fetch); the skeleton
  branch below covers that, so we never flash a "closed" banner on a
  page where the market is actually open.
  */
  const marketClosed = !!marketStatus && !marketStatus.open;

  return (
    <div className="animate-in fade-in duration-300">
      <div className="mx-auto max-w-[1200px] px-4 py-6 sm:px-7">
        {marketClosed && rawData && trades.length > 0 && <MarketClosedBanner status={marketStatus} date={rawData.date} />}
        {!rawData || !marketStatus ? (
          <DashboardSkeleton />
        ) : !trades.length ? (
          <NoTradesToday />
        ) : stats?.hasSummaries ? (
          <>
            <StatsGrid totalPnl={stats.totalPnl} winRate={stats.winRate} profitFactor={stats.profitFactor} bestPnl={stats.bestPnl} bestSym={stats.bestSym} />
            <Section title="P&L by Trade" subtitle="Realized P&L per pick, sorted" className="mt-10 border-t border-border/40 pt-8">
              <PnlChart trades={trades} executions={executions} />
            </Section>
            <Section title="Trade Details" subtitle="Click any row to view the contract page" className="mt-10 border-t border-border/40 pt-8">
              <TradeTable trades={trades} executions={executions} date={rawData.date} liveQuotes={liveQuotes} />
            </Section>
            <Section title="Exposure" subtitle="How capital was deployed today. For long options, max loss is the premium paid." className="mt-10 border-t border-border/40 pt-8">
              <ExposurePanel trades={trades} executions={executions} hasSummaries />
            </Section>
          </>
        ) : (
          <>
            <Section title="Today's Picks" subtitle={`${trades.length} ranked play${trades.length === 1 ? "" : "s"} · click any pick for the full single-contract view`}>
              <MorningLayout trades={trades} liveQuotes={liveQuotes} date={rawData.date} executions={executions} />
            </Section>
            <Section title="Exposure" subtitle="Capital at risk for today's picks. For long options, max loss is the premium paid." className="mt-10 border-t border-border/40 pt-8">
              {/*
              Pass executions (not null) to ExposurePanel on the morning
              branch too. exposure-panel.tsx multiplies premium by
              executionQuantity(exec); null would collapse every pick to
              qty=1 and under-report Capital at Risk on legacy historical
              days where a rank fired qty > 1.
              */}
              <ExposurePanel trades={trades} executions={executions} hasSummaries={false} />
            </Section>
          </>
        )}
      </div>
    </div>
  );
}
