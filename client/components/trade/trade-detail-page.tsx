"use client";

import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { ExecutionBadge, findExecutionForTrade, liveMarkForTrade } from "@/components/execution-badge";
import { Badge } from "@/components/ui/badge";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { Card, CardContent } from "@/components/ui/card";
import { Metric } from "@/components/ui/metric";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { calcBreakeven, calcMaxLoss, calcMoneyness, sentimentColor, sentimentLabel } from "@/lib/calculations";
import { fmt, fmtMoney, fmtPctDec, fmtPnlInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade, Execution, LiveQuotesResponse, Trade } from "@/types/trade";

const LIVE_POLL_SECONDS = 15;

type LoadState =
  | { kind: "loading" }
  | { kind: "found"; dt: DashboardTrade; resolvedDate: string; execution: Execution | null }
  | { kind: "not-found"; tried: string }
  | { kind: "error"; message: string };

export function TradeDetailPage({ symbol, date }: { symbol: string; date?: string }) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    setState({ kind: "loading" });
    api
      .getTrades(date)
      .then((data) => {
        if (cancelled) return;
        const dt = (data.trades ?? []).find((row) => row.trade.symbol.toUpperCase() === symbol.toUpperCase());
        if (!dt) {
          setState({ kind: "not-found", tried: data.date ?? date ?? "today" });
          return;
        }
        const execution = findExecutionForTrade(data.executions, dt.trade);
        setState({ kind: "found", dt, resolvedDate: data.date, execution });
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setState({ kind: "error", message: e instanceof Error ? e.message : "Failed to load trade" });
      });
    return () => {
      cancelled = true;
    };
  }, [symbol, date]);

  return (
    <div className="mx-auto min-w-0 max-w-[1100px] px-4 py-6 sm:px-7">
      <BackLink />
      {state.kind === "loading" && <LoadingPanel symbol={symbol} />}
      {state.kind === "error" && <Panel tone="error" title="Couldn't load that trade" body={state.message} />}
      {state.kind === "not-found" && (
        <Panel tone="muted" title={`No $${symbol} pick on ${state.tried}`} body="The dashboard only shows picks for trading days the system ran. Try a different date or head back to the dashboard." />
      )}
      {state.kind === "found" && <TradeDetailBody dt={state.dt} resolvedDate={state.resolvedDate} execution={state.execution} />}
    </div>
  );
}

function BackLink() {
  return (
    <Link href="/dashboard" className="mb-4 inline-flex min-h-9 items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground">
      <ArrowLeft className="h-4 w-4" />
      Back to dashboard
    </Link>
  );
}

function LoadingPanel({ symbol }: { symbol: string }) {
  return (
    <Card className="lg-card">
      <CardContent className="space-y-3 p-6">
        <div className="text-sm text-muted-foreground">Loading ${symbol}…</div>
        <div className="h-6 w-1/3 animate-pulse rounded bg-muted/60" />
        <div className="h-4 w-2/3 animate-pulse rounded bg-muted/60" />
      </CardContent>
    </Card>
  );
}

function Panel({ tone, title, body }: { tone: "error" | "muted"; title: string; body: string }) {
  return (
    <Card className="lg-card">
      <CardContent className="space-y-2 p-6">
        <h2 className={cn("text-lg font-semibold", tone === "error" ? "text-red" : "text-foreground")}>{title}</h2>
        <p className="text-sm text-muted-foreground">{body}</p>
      </CardContent>
    </Card>
  );
}

function TradeDetailBody({ dt, resolvedDate, execution }: { dt: DashboardTrade; resolvedDate: string; execution: Execution | null }) {
  const { trade, summary } = dt;
  const moneyness = calcMoneyness(trade);
  const breakeven = calcBreakeven(trade);
  const maxLoss = calcMaxLoss(trade);

  /*
  P&L preference for the EOD result block: when a closed live execution
  exists, prefer its realized_pnl + actual broker fill prices over
  Claude's modeled summary. The summary uses Claude's morning estimate
  for entry and an EOD re-quote for close — both can drift from the real
  broker fill (which often differs by tens of cents on liquid options),
  so the modeled number occasionally over- or understates real P&L by
  10-20%. The badge already shows the correct broker number; this
  alignment removes the user-visible mismatch between the dashboard
  card and the EOD result block on the same trade.
  */
  const hasClosedExecution = execution?.state === "closed" && execution.open_price > 0 && execution.close_price > 0;
  const entryPrice = hasClosedExecution ? execution.open_price : (summary?.entry_price ?? 0);
  const closingPrice = hasClosedExecution ? execution.close_price : (summary?.closing_price ?? 0);
  const pnl = hasClosedExecution ? execution.realized_pnl : summary ? (summary.closing_price - summary.entry_price) * 100 : 0;
  const pctChange = entryPrice > 0 ? ((closingPrice - entryPrice) / entryPrice) * 100 : 0;
  const stockPctChange = summary && summary.stock_open > 0 ? ((summary.stock_close - summary.stock_open) / summary.stock_open) * 100 : 0;

  /*
  Only poll live quotes for unsettled trades — once an EOD summary
  lands the contract is closed for the day and the EOD block already
  shows realized P&L. Server only returns live data for today's
  morning picks anyway, so historical dates render no live block.
  */
  const liveQuotes = useLiveQuotes(!summary);

  return (
    <div className="space-y-5">
      {execution && <ExecutionBadge execution={execution} variant="full" liveMark={liveMarkForTrade(liveQuotes, trade)} />}
      {!summary && <LivePanel trade={trade} liveQuotes={liveQuotes} />}
      {/* Header: ticker + badges + price */}
      <Card className="lg-card">
        <CardContent className="space-y-4 p-5 sm:p-6">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="font-mono text-2xl font-bold tabular-nums text-foreground sm:text-3xl">${trade.symbol}</h1>
            <Badge variant="outline" className={cn(trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
              {trade.contract_type}
            </Badge>
            <Badge variant={moneyness.variant}>{moneyness.label}</Badge>
            <Badge variant="secondary">Rank #{trade.rank}</Badge>
            <Badge variant="secondary" className="text-xs">
              {trade.risk_level}
            </Badge>
            {summary && <Badge variant={pnl > 0 ? "default" : pnl < 0 ? "destructive" : "secondary"}>EOD {fmtPnlInt(pnl)}</Badge>}
            <span className="ml-auto text-xs text-muted-foreground">{resolvedDate}</span>
          </div>

          <div className="grid grid-cols-2 gap-x-4 gap-y-3 text-sm sm:grid-cols-3 md:grid-cols-4">
            <Metric label="Strike" value={fmtMoney(trade.strike_price)} />
            <Metric label="Expiration" value={`${trade.expiration} (${trade.dte}d)`} />
            <Metric label="Entry" value={fmtMoney(trade.estimated_price)} />
            <Metric
              label="Target"
              value={<span className={cn("text-sm font-semibold tabular-nums", trade.contract_type === "CALL" ? "text-green" : "text-red")}>{fmtMoney(trade.target_price)}</span>}
            />
            <Metric label="Stop loss" value={fmtMoney(trade.stop_loss)} />
            <Metric label="Breakeven" value={fmtMoney(breakeven)} />
            <Metric label="Max loss" value={<span className="text-sm font-semibold tabular-nums text-red">{fmtPnlInt(-maxLoss)}</span>} />
            <Metric
              label="Sentiment"
              value={
                <span className={cn("text-sm font-semibold tabular-nums", sentimentColor(trade.sentiment_score))}>
                  {sentimentLabel(trade.sentiment_score)} ({fmt(trade.sentiment_score, 2)})
                </span>
              }
            />
            <Metric label="Mentions" value={String(trade.mention_count)} />
            <Metric label="Stock at entry" value={fmtMoney(trade.current_price)} />
          </div>

          {trade.catalyst && (
            <div className="rounded-md bg-amber-bg px-3 py-2 text-sm">
              <span className="font-semibold text-amber">Catalyst:</span> {trade.catalyst}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Picker rationale */}
      {trade.rationale && (
        <Card className="lg-card">
          <CardContent className="space-y-3 p-5 sm:p-6">
            <div className="flex items-center gap-2">
              <ClaudeLogo className="h-5 w-5" />
              <span className="text-base font-semibold text-claude">Claude's read</span>
              {trade.score > 0 && (
                <Badge variant="secondary" className="tabular-nums">
                  {trade.score}/10
                </Badge>
              )}
            </div>
            <p className="text-sm leading-relaxed text-muted-foreground">{trade.rationale}</p>
          </CardContent>
        </Card>
      )}

      {/* EOD result if settled */}
      {summary && (
        <Card className="lg-card">
          <CardContent className="space-y-4 p-5 sm:p-6">
            <div className="flex flex-wrap items-baseline justify-between gap-3">
              <div className="flex items-baseline gap-2">
                <h2 className="text-base font-semibold">End-of-day result</h2>
                {hasClosedExecution && (
                  <Badge variant="secondary" className="text-[10px]">
                    actual fill
                  </Badge>
                )}
              </div>
              <span className={cn("text-2xl font-bold tabular-nums", pnl > 0 ? "text-green" : pnl < 0 ? "text-red" : "text-muted-foreground")}>
                {fmtPnlInt(pnl)}
                <span className="ml-2 text-sm font-medium text-muted-foreground">{fmtPctDec(pctChange)}</span>
              </span>
            </div>

            <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
              <Metric label="Contract entry" value={fmtMoney(entryPrice)} />
              <Metric label="Contract close" value={<span className={cn("text-sm font-semibold tabular-nums", pnl > 0 ? "text-green" : pnl < 0 ? "text-red" : "")}>{fmtMoney(closingPrice)}</span>} />
              <Metric label="Stock open" value={fmtMoney(summary.stock_open)} />
              <Metric
                label="Stock close"
                value={
                  <span className={cn("text-sm font-semibold tabular-nums", stockPctChange > 0 ? "text-green" : stockPctChange < 0 ? "text-red" : "")}>
                    {fmtMoney(summary.stock_close)}
                    <span className="ml-1 text-xs font-medium text-muted-foreground">({fmtPctDec(stockPctChange)})</span>
                  </span>
                }
              />
            </div>

            {summary.notes && <p className="rounded-md border bg-muted/30 p-3 text-sm text-muted-foreground">{summary.notes}</p>}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

/*
Polls /api/quotes/live on the same 15s cadence the dashboard uses, with
the same merge-over-prior semantics so a single transient miss on the
contract's option chain doesn't blank the live panel.
*/
function useLiveQuotes(enabled: boolean): LiveQuotesResponse | null {
  const [liveQuotes, setLiveQuotes] = useState<LiveQuotesResponse | null>(null);

  useEffect(() => {
    if (!enabled) return;
    const poll = () =>
      api
        .getLiveQuotes()
        .then((next) => {
          if (!next || typeof next !== "object") return;
          setLiveQuotes((prev) => ({
            ...next,
            quotes: { ...(prev?.quotes ?? {}), ...(next.quotes ?? {}) },
            options: { ...(prev?.options ?? {}), ...(next.options ?? {}) },
          }));
        })
        .catch(() => {});
    poll();
    const interval = setInterval(poll, LIVE_POLL_SECONDS * 1000);
    return () => clearInterval(interval);
  }, [enabled]);

  return liveQuotes;
}

function LivePanel({ trade, liveQuotes }: { trade: Trade; liveQuotes: LiveQuotesResponse | null }) {
  /*
  Reconstruct the same option key the server emits in handleLiveQuotes
  (server.go: "<SYMBOL>|<CALL|PUT>|<strike .2f>|<expiration>"). When the
  contract isn't in the response — historical date, market closed,
  Schwab disconnected — render nothing so the page doesn't lead with a
  table of em-dashes.
  */
  const optionKey = `${trade.symbol}|${trade.contract_type}|${trade.strike_price.toFixed(2)}|${trade.expiration}`;
  const liveOption = liveQuotes?.options?.[optionKey] ?? null;
  const liveStock = liveQuotes?.quotes?.[trade.symbol] ?? null;

  if (!liveOption && !liveStock) return null;

  const currentPrice = liveOption?.mark ?? null;
  const contractDelta = currentPrice !== null && trade.estimated_price > 0 ? currentPrice - trade.estimated_price : null;
  const contractDeltaPct = contractDelta !== null && trade.estimated_price > 0 ? (contractDelta / trade.estimated_price) * 100 : null;
  const livePnl = contractDelta !== null ? contractDelta * 100 : null;

  const stockDelta = liveStock && trade.current_price > 0 ? liveStock.last_price - trade.current_price : null;
  const stockDeltaPct = stockDelta !== null && trade.current_price > 0 ? (stockDelta / trade.current_price) * 100 : null;

  return (
    <Card className="lg-card">
      <CardContent className="space-y-4 p-5 sm:p-6">
        <div className="flex flex-wrap items-baseline justify-between gap-3">
          <div className="flex items-center gap-2">
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green opacity-75" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-green" />
            </span>
            <h2 className="text-base font-semibold">Live</h2>
            {liveQuotes?.market_open === false && (
              <Badge variant="secondary" className="text-[10px]">
                Market closed
              </Badge>
            )}
          </div>
          {livePnl !== null && (
            <span className={cn("text-2xl font-bold tabular-nums", pnlColor(livePnl))}>
              {fmtPnlInt(livePnl)}
              {contractDeltaPct !== null && <span className="ml-2 text-sm font-medium text-muted-foreground">{fmtPctDec(contractDeltaPct)}</span>}
            </span>
          )}
        </div>

        <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
          <Metric
            label="Contract mark"
            value={
              currentPrice !== null ? (
                <span className={cn("text-sm font-semibold tabular-nums", pnlColor(contractDelta ?? 0))}>{fmtMoney(currentPrice)}</span>
              ) : (
                <Skeleton className="inline-block h-4 w-16 align-middle" />
              )
            }
          />
          <Metric
            label="Bid / Ask"
            value={
              liveOption ? (
                <span className="text-sm font-medium tabular-nums">
                  {fmtMoney(liveOption.bid)} / {fmtMoney(liveOption.ask)}
                </span>
              ) : (
                <Skeleton className="inline-block h-4 w-20 align-middle" />
              )
            }
          />
          <Metric
            label="Stock"
            value={
              liveStock ? (
                <span className={cn("text-sm font-semibold tabular-nums", pnlColor(stockDelta ?? 0))}>
                  {fmtMoney(liveStock.last_price)}
                  {stockDeltaPct !== null && <span className="ml-1 text-xs font-medium text-muted-foreground">({fmtPctDec(stockDeltaPct)})</span>}
                </span>
              ) : (
                <Skeleton className="inline-block h-4 w-20 align-middle" />
              )
            }
          />
          <Metric
            label="Volume"
            value={liveOption ? <span className="text-sm font-medium tabular-nums">{liveOption.volume.toLocaleString()}</span> : <Skeleton className="inline-block h-4 w-14 align-middle" />}
          />
        </div>

        <p className="text-[11px] text-muted-foreground">
          Refreshes every {LIVE_POLL_SECONDS}s · entry was {fmtMoney(trade.estimated_price)}.
        </p>
      </CardContent>
    </Card>
  );
}
