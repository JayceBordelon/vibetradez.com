"use client";

import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { findExecutionForTrade } from "@/components/execution-badge";
import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { Badge } from "@/components/ui/badge";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { Metric } from "@/components/ui/metric";
import { Skeleton } from "@/components/ui/skeleton";
import { useMarketStatus } from "@/hooks/use-market-status";
import { useQuoteStream } from "@/hooks/use-quote-stream";
import { api } from "@/lib/api";
import { calcBreakeven, calcMaxLoss, calcMoneyness, sentimentColor, sentimentLabel } from "@/lib/calculations";
import { fmt, fmtMoney, fmtPctDec, fmtPnlInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade, Execution, LiveQuotesResponse, Trade } from "@/types/trade";

type LoadState =
  | { kind: "loading" }
  | {
      kind: "found";
      dt: DashboardTrade;
      resolvedDate: string;
      execution: Execution | null;
    }
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
        setState({
          kind: "error",
          message: e instanceof Error ? e.message : "Failed to load trade",
        });
      });
    return () => {
      cancelled = true;
    };
  }, [symbol, date]);

  return (
    <div className="mx-auto min-w-0 max-w-[1100px] px-4 py-6 sm:px-7">
      <BackLink />
      {state.kind === "loading" && <TradeDetailSkeleton symbol={symbol} />}
      {state.kind === "error" && <StatusBlock tone="error" title="Couldn't load that trade" body={state.message} />}
      {state.kind === "not-found" && (
        <StatusBlock
          tone="muted"
          title={`No $${symbol} pick on ${state.tried}`}
          body="The dashboard only shows picks for trading days the system ran. Try a different date or head back to the dashboard."
        />
      )}
      {state.kind === "found" && <TradeDetailBody dt={state.dt} resolvedDate={state.resolvedDate} execution={state.execution} />}
    </div>
  );
}

function BackLink() {
  return (
    <Link href="/dashboard" className="mb-6 inline-flex min-h-9 items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground">
      <ArrowLeft className="h-4 w-4" />
      Back to dashboard
    </Link>
  );
}

/*
Mirrors the TradeDetailBody layout so the page doesn't jump when the
fetch resolves: ticker header, contract grid (real labels, skeleton
values, the labels are informative-loading), one generic live/EOD
section block, and Claudia's read. Symbol is known from the URL so
it renders as the real H1 instead of pulsing. Execution badge and
catalyst block are conditional in the loaded view, so we don't reserve
space for them here, a small one-line shift on those is acceptable.

Mobile: 2-col contract grid + 2-col stat strip (matches sm:grid-cols-4
breakpoint on the real components). Skeleton widths stay narrow enough
to fit a 320px viewport without horizontal overflow.
*/
const SKELETON_METRIC_LABELS = ["Strike", "Expiration", "Entry", "Target", "Stop loss", "Breakeven", "Max loss", "Sentiment", "Mentions", "Stock at entry"] as const;

function TradeDetailSkeleton({ symbol }: { symbol: string }) {
  return (
    <div className="space-y-2" aria-busy="true" aria-live="polite">
      {/* Ticker header, symbol is known, surrounding badges + date pulse */}
      <div className="flex flex-wrap items-center gap-2">
        <h1 className="font-mono text-3xl font-bold tabular-nums text-foreground sm:text-4xl">${symbol}</h1>
        <Skeleton className="h-5 w-14" />
        <Skeleton className="h-5 w-20" />
        <Skeleton className="h-5 w-16" />
        <Skeleton className="h-5 w-14" />
        <Skeleton className="ml-auto h-3 w-24" />
      </div>

      {/* Catalyst, reserved (most picks have one) */}
      <div className="mt-5 rounded-md border border-amber-border/40 bg-amber-bg/60 px-4 py-3">
        <Skeleton className="h-4 w-full max-w-md" />
      </div>

      {/* Contract properties, real labels, skeleton values */}
      <div className="mt-6 grid grid-cols-2 gap-x-6 gap-y-3 text-sm sm:grid-cols-3 md:grid-cols-4">
        {SKELETON_METRIC_LABELS.map((label) => (
          <Metric key={label} label={label} value={<Skeleton className="ml-auto h-4 w-14" />} />
        ))}
      </div>

      {/* Live / EOD section head, generic placeholder, real component picks one once loaded */}
      <div className="mb-4 mt-2 flex flex-wrap items-baseline justify-between gap-3 border-t border-border/40 pt-8">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-7 w-28" />
      </div>
      {/* StatStrip, mirrors grid-cols-2 mobile / sm:grid-cols-4 desktop with hairline dividers */}
      <div className="grid grid-cols-2 divide-x divide-y divide-border/50 border-y border-border/50 sm:grid-cols-4 sm:divide-y-0 sm:border-y-0 sm:border-t sm:border-b">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="min-w-0 px-4 py-4 sm:px-5">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="mt-2 h-7 w-20" />
          </div>
        ))}
      </div>

      {/* Claudia's read, reserved (rationale is always set) */}
      <div className="mt-10 rounded-lg border border-claude-border/40 bg-claude-light px-5 py-4">
        <div className="mb-2 flex items-center gap-2">
          <ClaudeLogo className="h-4 w-4" />
          <span className="text-sm font-semibold text-claude">Claudia's read</span>
          <Skeleton className="h-5 w-10" />
        </div>
        <div className="space-y-2">
          <Skeleton className="h-3.5 w-full" />
          <Skeleton className="h-3.5 w-11/12" />
          <Skeleton className="h-3.5 w-3/4" />
        </div>
      </div>
    </div>
  );
}

function StatusBlock({ tone, title, body }: { tone: "error" | "muted"; title: string; body: string }) {
  return (
    <div className="space-y-2">
      <h2 className={cn("text-xl font-semibold", tone === "error" ? "text-red" : "text-foreground")}>{title}</h2>
      <p className="text-sm text-muted-foreground">{body}</p>
    </div>
  );
}

function SectionHead({ title, right, eyebrow }: { title: string; right?: React.ReactNode; eyebrow?: React.ReactNode }) {
  return (
    <div className="mb-4 flex flex-wrap items-baseline justify-between gap-3 border-t border-border/40 pt-8">
      <div className="flex items-baseline gap-2">
        {eyebrow}
        <h2 className="text-lg font-semibold tracking-tight">{title}</h2>
      </div>
      {right}
    </div>
  );
}

function TradeDetailBody({ dt, resolvedDate, execution }: { dt: DashboardTrade; resolvedDate: string; execution: Execution | null }) {
  const { trade, summary } = dt;
  const moneyness = calcMoneyness(trade);
  const breakeven = calcBreakeven(trade);
  const maxLoss = calcMaxLoss(trade);
  const isSkipped = execution?.state === "skipped";

  /*
  P&L preference for the EOD block: when a closed live execution
  exists, prefer its realized_pnl + actual broker fill prices over
  Claude's modeled summary.
  */
  const hasClosedExecution = execution?.state === "closed" && execution.open_price > 0 && execution.close_price > 0;
  const entryPrice = hasClosedExecution ? execution.open_price : (summary?.entry_price ?? 0);
  const closingPrice = hasClosedExecution ? execution.close_price : (summary?.closing_price ?? 0);
  const pnl = hasClosedExecution ? execution.realized_pnl : summary && summary.entry_price > 0 ? (summary.closing_price - summary.entry_price) * 100 : 0;
  const pctChange = entryPrice > 0 ? ((closingPrice - entryPrice) / entryPrice) * 100 : 0;
  const stockPctChange = summary && summary.stock_open > 0 ? ((summary.stock_close - summary.stock_open) / summary.stock_open) * 100 : 0;

  /*
  Live quotes streaming: only open when (a) the pick is still mid-day
  (no summary yet), (b) the agent actually fired an order (skipped
  picks have no position to watch), and (c) the market is open.
  Outside hours, the SSE endpoint 503s anyway and the EOD result
  block below is what matters.
  */
  const marketStatus = useMarketStatus();
  const liveQuotes = useQuoteStream(!summary && !isSkipped && marketStatus?.open === true);

  /*
  Compute the live snapshot once at the body level so the hero P&L
  block at the top of the page and the Live StatStrip near the bottom
  can share the same numbers, single source of truth, no risk of
  drift between the two surfaces.
  */
  const liveSnap = !summary && !isSkipped ? computeLiveSnapshot(trade, liveQuotes, execution) : null;

  return (
    <div className="space-y-2">
      {/* Ticker header, flat, no card. Paper/Taken pill lives inline here instead of a separate panel. */}
      <div className="flex flex-wrap items-center gap-2">
        <h1 className="font-mono text-3xl font-bold tabular-nums text-foreground sm:text-4xl">${trade.symbol}</h1>
        <Badge variant="outline" className={cn(trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
          {trade.contract_type}
        </Badge>
        <Badge variant={moneyness.variant}>{moneyness.label}</Badge>
        <Badge variant="secondary">Rank #{trade.rank}</Badge>
        <Badge variant="secondary" className="text-xs">
          {trade.risk_level}
        </Badge>
        {execution && <PositionTag execution={execution} />}
        {summary && <Badge variant={pnl > 0 ? "default" : pnl < 0 ? "destructive" : "secondary"}>EOD {fmtPnlInt(pnl)}</Badge>}
        <span className="ml-auto text-xs text-muted-foreground">{resolvedDate}</span>
      </div>

      {/* Live P&L hero, the single highest-priority number on the page when a live position is active. Sits right under the ticker header so the user's eye lands on it first. */}
      {liveSnap && liveSnap.livePnl !== null && <LivePnlHero snap={liveSnap} />}

      {/* Catalyst, soft tinted prose container, NOT a full card */}
      {trade.catalyst && (
        <div className="mt-5 rounded-md border border-amber-border/40 bg-amber-bg/60 px-4 py-3 text-sm">
          <span className="font-semibold text-amber">Catalyst:</span> <span className="text-foreground/90">{trade.catalyst}</span>
        </div>
      )}

      {/* Claudia's read, promoted near the top so the thesis is the first prose the user reads */}
      {trade.rationale && (
        <div className="mt-5 rounded-lg border border-claude-border/50 bg-claude-light px-6 py-5">
          <div className="mb-3 flex items-center gap-2">
            <ClaudeLogo className="h-5 w-5" />
            <span className="text-base font-semibold text-claude">Claudia's read</span>
            {trade.score > 0 && (
              <Badge variant="secondary" className="tabular-nums">
                {trade.score}/10
              </Badge>
            )}
          </div>
          <p className="text-[15px] leading-relaxed text-foreground/90">{trade.rationale}</p>
        </div>
      )}

      {/* Contract properties, properties grid, no card. Strike/expiration/
          entry/stop are filled in by the 9:30 ET executor; render an em-
          dash placeholder while the contract is still being resolved.
          For skipped picks, no contract was ever chosen — collapse the
          unresolved fields to em-dashes (no "Finding contracts..."
          placeholder, the contract search ended with a decline). */}
      <div className="mt-6 grid grid-cols-2 gap-x-6 gap-y-3 text-sm sm:grid-cols-3 md:grid-cols-4">
        <Metric label="Strike" value={trade.strike_price !== null ? fmtMoney(trade.strike_price) : "-"} />
        <Metric
          label="Expiration"
          value={trade.expiration !== null && trade.dte !== null ? `${trade.expiration} (${trade.dte}d)` : isSkipped ? "-" : "Finding contracts..."}
        />
        <Metric label="Entry" value={trade.estimated_price !== null ? fmtMoney(trade.estimated_price) : "-"} />
        <Metric label="Target" value={<span className={cn("text-sm font-semibold tabular-nums", trade.contract_type === "CALL" ? "text-green" : "text-red")}>{fmtMoney(trade.target_price)}</span>} />
        <Metric label="Stop loss" value={trade.stop_loss !== null ? fmtMoney(trade.stop_loss) : "-"} />
        <Metric label="Breakeven" value={breakeven !== null ? fmtMoney(breakeven) : "-"} />
        <Metric label="Max loss" value={maxLoss !== null ? <span className="text-sm font-semibold tabular-nums text-red">{fmtPnlInt(-maxLoss)}</span> : "-"} />
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

      {/* Skipped picks get a dedicated reason block in place of the
          live/EOD section. The agent looked at this candidate at 9:30
          and chose not to fire; the reason is the entire payload. */}
      {isSkipped && execution && (
        <div className="mt-8 rounded-lg border border-border bg-muted/30">
          <div className="border-b border-border px-4 py-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">Skipped at open by the at-open agent</div>
          <div className="px-4 py-3 text-sm leading-relaxed text-foreground/85">{execution.note?.trim() ? execution.note : "No reason provided."}</div>
        </div>
      )}

      {/* Live block, flat section, StatStrip metrics. Same shape whether or not a position was taken; execution only nudges the P&L baseline. */}
      {!summary && !isSkipped && liveSnap && <LiveSection snap={liveSnap} marketOpen={liveQuotes?.market_open} />}

      {/* EOD result, flat section */}
      {summary && (
        <>
          <SectionHead
            title="End-of-day result"
            eyebrow={
              hasClosedExecution ? (
                <Badge variant="secondary" className="text-[10px]">
                  actual fill
                </Badge>
              ) : null
            }
            right={
              <span className={cn("text-3xl font-bold tabular-nums leading-none", pnl > 0 ? "text-green" : pnl < 0 ? "text-red" : "text-muted-foreground")}>
                {fmtPnlInt(pnl)}
                <span className="ml-2 text-sm font-medium text-muted-foreground">{fmtPctDec(pctChange)}</span>
              </span>
            }
          />
          <StatStrip cols={4}>
            <Stat label="Contract entry" value={fmtMoney(entryPrice)} />
            <Stat label="Contract close" value={fmtMoney(closingPrice)} tone={pnl > 0 ? "positive" : pnl < 0 ? "negative" : "neutral"} />
            <Stat label="Stock open" value={fmtMoney(summary.stock_open)} />
            <Stat
              label="Stock close"
              value={fmtMoney(summary.stock_close)}
              sub={<span className={cn(stockPctChange > 0 ? "text-green" : stockPctChange < 0 ? "text-red" : "")}>{fmtPctDec(stockPctChange)}</span>}
            />
          </StatStrip>
          {summary.notes && <p className="mt-4 rounded-md border border-border/50 bg-muted/30 px-4 py-3 text-sm text-muted-foreground">{summary.notes}</p>}
        </>
      )}
    </div>
  );
}

/*
PositionTag is the inline replacement for the old big "Position taken"
panel. The trade detail page is now identical whether or not Jayce
took the trade, this small pill is the only signal. Two normal
states (Paper / Taken) plus loud warnings for failed / canceled /
close_failed so the operator notices.
*/
function PositionTag({ execution }: { execution: Execution }) {
  const { state, mode } = execution;
  const qty = execution.quantity && execution.quantity > 0 ? execution.quantity : 1;

  if (state === "failed") {
    return (
      <Badge variant="outline" className="border-red-border text-red">
        Open failed
      </Badge>
    );
  }
  if (state === "canceled") {
    return (
      <Badge variant="secondary" className="text-xs">
        Open canceled
      </Badge>
    );
  }
  if (state === "close_failed") {
    return (
      <Badge variant="outline" className="border-red-border text-red">
        Close failed
      </Badge>
    );
  }

  const qtySuffix = qty > 1 ? ` ×${qty}` : "";
  if (mode === "paper") {
    return (
      <Badge variant="outline" className="border-amber-border bg-amber-bg/40 text-amber" title="Paper-trade: no real money committed">
        Paper{qtySuffix}
      </Badge>
    );
  }
  return (
    <Badge variant="outline" className="border-red-border bg-red-bg/40 text-red" title="Real position taken via Schwab">
      Taken{qtySuffix}
    </Badge>
  );
}

/*
LiveSnapshot is the materialized state of the live SSE feed for one
contract, reduced to render-ready fields. Computed once in
TradeDetailBody and shared by LivePnlHero (the giant spotlight number)
and LiveSection (the supporting StatStrip) so the two surfaces never
disagree.

Returns null when no live data has arrived yet, both renderers
should skip in that case.
*/
type LiveSnapshot = {
  liveOption: NonNullable<LiveQuotesResponse["options"]>[string] | null;
  liveStock: NonNullable<LiveQuotesResponse["quotes"]>[string] | null;
  mark: number | null;
  contractDelta: number | null;
  contractDeltaPct: number | null;
  livePnl: number | null;
  stockDelta: number | null;
  stockDeltaPct: number | null;
  entryCaption: string;
};

function computeLiveSnapshot(trade: Trade, liveQuotes: LiveQuotesResponse | null, execution: Execution | null): LiveSnapshot | null {
  if (!liveQuotes) return null;
  /*
  Reconstruct the same option key the server emits (server.go ~846):
  "<SYMBOL>|<CALL|PUT>|<strike .2f>|<expiration>".
  Skip the option lookup entirely for pre-resolution picks: there's no
  strike/expiration to key on yet.
  */
  const optionKey = trade.strike_price !== null && trade.expiration !== null ? `${trade.symbol}|${trade.contract_type}|${trade.strike_price.toFixed(2)}|${trade.expiration}` : null;
  const liveOption = optionKey ? (liveQuotes.options?.[optionKey] ?? null) : null;
  const liveStock = liveQuotes.quotes?.[trade.symbol] ?? null;

  if (!liveOption && !liveStock) return null;

  /*
  P&L baseline: actual broker fill price × quantity when a holding
  execution exists, otherwise the picker's modeled entry × 1 contract.
  Keeps the headline P&L honest for taken trades without changing
  the shape for paper / not-taken picks.
  */
  const useFill = execution?.state === "holding" && execution.open_price > 0;
  const qty = execution?.quantity && execution.quantity > 0 ? execution.quantity : 1;
  const pnlEntry = useFill ? execution.open_price : (trade.estimated_price ?? 0);
  const pnlMultiplier = useFill ? qty : 1;

  const mark = liveOption?.mark ?? null;
  const contractDelta = mark !== null && pnlEntry > 0 ? mark - pnlEntry : null;
  const contractDeltaPct = contractDelta !== null && pnlEntry > 0 ? (contractDelta / pnlEntry) * 100 : null;
  const livePnl = contractDelta !== null ? contractDelta * 100 * pnlMultiplier : null;

  const stockDelta = liveStock && trade.current_price > 0 ? liveStock.last_price - trade.current_price : null;
  const stockDeltaPct = stockDelta !== null && trade.current_price > 0 ? (stockDelta / trade.current_price) * 100 : null;

  const entryCaption = useFill ? `fill ${fmtMoney(pnlEntry)}${qty > 1 ? ` ×${qty}` : ""}` : `entry was ${fmtMoney(pnlEntry)}`;

  return {
    liveOption,
    liveStock,
    mark,
    contractDelta,
    contractDeltaPct,
    livePnl,
    stockDelta,
    stockDeltaPct,
    entryCaption,
  };
}

/*
LivePnlHero is the single highest-priority number on the trade detail
page. Sits right under the ticker header so it's the first metric the
user's eye lands on. The supporting numbers (mark, bid/ask, stock,
volume) still render in the StatStrip below, this block only owns
the headline P&L + percentage.
*/
function LivePnlHero({ snap }: { snap: LiveSnapshot }) {
  const { livePnl, contractDeltaPct, mark, entryCaption } = snap;
  if (livePnl === null) return null;

  return (
    <div className="mt-6 rounded-xl border border-border/60 bg-card/50 px-5 py-5 sm:px-8 sm:py-7">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green opacity-75" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-green" />
            </span>
            <span>Live unrealized P&L</span>
          </div>
          <div className={cn("mt-3 text-5xl font-bold tabular-nums leading-none sm:text-6xl", pnlColor(livePnl))}>{fmtPnlInt(livePnl)}</div>
        </div>
        <div className="flex flex-col items-end gap-1 text-right">
          {contractDeltaPct !== null && <span className={cn("text-2xl font-semibold tabular-nums leading-none sm:text-3xl", pnlColor(livePnl))}>{fmtPctDec(contractDeltaPct)}</span>}
          {mark !== null && (
            <span className="text-xs text-muted-foreground">
              mark {fmtMoney(mark)} · {entryCaption}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

function LiveSection({ snap, marketOpen }: { snap: LiveSnapshot; marketOpen?: boolean }) {
  const { liveOption, liveStock, mark, contractDelta, stockDelta, stockDeltaPct, entryCaption } = snap;

  return (
    <>
      <SectionHead
        title="Live"
        eyebrow={
          <span className="relative flex h-2 w-2">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green opacity-75" />
            <span className="relative inline-flex h-2 w-2 rounded-full bg-green" />
          </span>
        }
        right={
          marketOpen === false ? (
            <Badge variant="secondary" className="text-[10px]">
              Market closed
            </Badge>
          ) : null
        }
      />
      <StatStrip cols={4}>
        <Stat
          label="Contract mark"
          value={
            mark !== null ? (
              <span
                style={{
                  color: contractDelta != null && contractDelta > 0 ? "var(--green)" : contractDelta != null && contractDelta < 0 ? "var(--red)" : undefined,
                }}
              >
                {fmtMoney(mark)}
              </span>
            ) : (
              <Skeleton className="inline-block h-6 w-16 align-middle" />
            )
          }
        />
        <Stat label="Bid / Ask" value={liveOption ? `${fmtMoney(liveOption.bid)} / ${fmtMoney(liveOption.ask)}` : <Skeleton className="inline-block h-6 w-20 align-middle" />} />
        <Stat
          label="Stock"
          value={liveStock ? fmtMoney(liveStock.last_price) : <Skeleton className="inline-block h-6 w-20 align-middle" />}
          sub={
            stockDeltaPct !== null ? <span className={cn(stockDelta && stockDelta > 0 ? "text-green" : stockDelta && stockDelta < 0 ? "text-red" : "")}>{fmtPctDec(stockDeltaPct)}</span> : undefined
          }
        />
        <Stat label="Volume" value={liveOption ? liveOption.volume.toLocaleString() : <Skeleton className="inline-block h-6 w-14 align-middle" />} />
      </StatStrip>
      <p className="mt-3 text-[11px] text-muted-foreground">Live ticks as Schwab pushes them · {entryCaption}.</p>
    </>
  );
}
