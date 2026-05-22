"use client";

import { ChevronRight } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { findExecutionForTrade, liveMarkForTrade } from "@/components/execution-badge";
import { Badge } from "@/components/ui/badge";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { Card, CardContent } from "@/components/ui/card";
import { Metric } from "@/components/ui/metric";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { calcMoneyness, computeTradePnl } from "@/lib/calculations";
import { fmtMoney, fmtPctDec, fmtPnlInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade, Execution, LiveQuotesResponse } from "@/types/trade";

interface TradeTableProps {
  trades: DashboardTrade[];
  date: string;
  executions?: Execution[] | null;
  liveQuotes?: LiveQuotesResponse | null;
}

function tradeHref(symbol: string, date: string): string {
  return `/trade/${encodeURIComponent(symbol)}?date=${encodeURIComponent(date)}`;
}

// ---- Computed values for a single DashboardTrade ----
interface RowComputed {
  hasSummary: boolean;
  pnl: number;
  pnlPct: number;
  stockMove: number;
  resultLabel: string;
  resultVariant: "default" | "destructive" | "outline" | "secondary";
  accentBorder: string;
  entry: string;
  close: string;
}

function computeRow(dt: DashboardTrade, executions: Execution[] | null | undefined, liveQuotes: LiveQuotesResponse | null | undefined): RowComputed {
  const { trade, summary } = dt;
  const hasSummary = !!summary;
  const result = computeTradePnl(dt, executions);
  const stockMove = hasSummary && summary.stock_open > 0 ? ((summary.stock_close - summary.stock_open) / summary.stock_open) * 100 : 0;

  /*
  Holding-with-live-mark gives the row a real-time unrealized P&L so
  the table matches what ExecutionBadge already shows on the morning
  cards. Only triggers for a live execution still in holding state
  with a positive mark; closed and paper rows fall through to the
  pre-existing summary/estimate path below.
  */
  const execution = findExecutionForTrade(executions, trade);
  const liveMark = liveMarkForTrade(liveQuotes, trade);
  const isLiveHolding = !hasSummary && execution?.state === "holding" && execution.mode === "live" && liveMark != null && execution.open_price > 0;
  const holdingPnl = isLiveHolding ? (liveMark - execution.open_price) * 100 : 0;
  const holdingPct = isLiveHolding ? ((liveMark - execution.open_price) / execution.open_price) * 100 : 0;

  let pnl: number;
  let pnlPct: number;
  let resultLabel: string;
  let resultVariant: RowComputed["resultVariant"];
  let accentBorder: string;
  let entry: string;
  let close: string;

  if (hasSummary) {
    pnl = result.pnl;
    pnlPct = result.pctChange;
    resultLabel = pnl > 0 ? "PROFIT" : pnl < 0 ? "LOSS" : "FLAT";
    resultVariant = pnl > 0 ? "default" : pnl < 0 ? "destructive" : "outline";
    accentBorder = pnlPct > 1 ? "border-l-green/40" : pnlPct < -1 ? "border-l-red/40" : "border-l-transparent";
    entry = fmtMoney(result.entryPrice);
    close = fmtMoney(result.closingPrice);
  } else if (isLiveHolding) {
    pnl = holdingPnl;
    pnlPct = holdingPct;
    resultLabel = holdingPnl > 0 ? "HOLDING ↑" : holdingPnl < 0 ? "HOLDING ↓" : "HOLDING";
    resultVariant = holdingPnl > 0 ? "default" : holdingPnl < 0 ? "destructive" : "secondary";
    accentBorder = holdingPnl > 0 ? "border-l-green/60" : holdingPnl < 0 ? "border-l-red/60" : "border-l-transparent";
    entry = fmtMoney(execution.open_price);
    close = fmtMoney(liveMark);
  } else if (execution?.state === "close_failed") {
    /*
    The auto-close cron exhausted its retry-cancel-replace window
    and the position is still open at the broker. Surface that
    loudly here, the prior shape rendered "OPEN" with no warning,
    making a stranded-at-broker position look identical to a
    never-executed pick.
    */
    pnl = 0;
    pnlPct = 0;
    resultLabel = "CLOSE FAILED";
    resultVariant = "destructive";
    accentBorder = "border-l-red";
    entry = fmtMoney(execution.open_price);
    close = "verify";
  } else if (execution?.state === "canceled") {
    /*
    9:35 ET cancel-dangling cron killed the LIMIT before it filled
    (stale quote, no second-chance retry). No position taken; render
    neutrally distinct from "OPEN" so the row reads as "pick fired
    but the open never happened" instead of a still-pending fill.
    */
    pnl = 0;
    pnlPct = 0;
    resultLabel = "CANCELED";
    resultVariant = "secondary";
    accentBorder = "border-l-transparent";
    entry = "-";
    close = "-";
  } else {
    pnl = result.pnl;
    pnlPct = result.pctChange;
    resultLabel = "OPEN";
    resultVariant = "secondary";
    accentBorder = "border-l-transparent";
    entry = trade.estimated_price !== null ? fmtMoney(trade.estimated_price) : "...";
    close = "-";
  }

  return {
    hasSummary,
    pnl,
    pnlPct,
    stockMove,
    resultLabel,
    resultVariant,
    accentBorder,
    entry,
    close,
  };
}

export function TradeTable({ trades, date, executions, liveQuotes }: TradeTableProps) {
  return (
    <div className="min-w-0">
      {/* Desktop table */}
      <div className="hidden overflow-hidden md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10 text-center">#</TableHead>
              <TableHead>Trade</TableHead>
              <TableHead className="text-center">Score</TableHead>
              <TableHead className="text-right">Entry</TableHead>
              <TableHead className="text-right">Close</TableHead>
              <TableHead className="text-right">Stock</TableHead>
              <TableHead className="text-right">P&amp;L</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {trades.map((dt) => (
              <DesktopTradeRow key={dt.trade.symbol} dt={dt} date={date} executions={executions} liveQuotes={liveQuotes} />
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Mobile cards */}
      <div className="space-y-3 md:hidden">
        {trades.map((dt) => (
          <TradeRowCard key={dt.trade.symbol} dt={dt} date={date} executions={executions} liveQuotes={liveQuotes} />
        ))}
      </div>
    </div>
  );
}

/**
---- Desktop row ----
Each row is a navigation surface to /trade/<symbol>?date=<date>; we render
the link inside a regular cell rather than wrapping the entire <tr> so we
keep valid table semantics (no <a> wrapping <tr>).
*/
function DesktopTradeRow({ dt, date, executions, liveQuotes }: { dt: DashboardTrade; date: string; executions?: Execution[] | null; liveQuotes?: LiveQuotesResponse | null }) {
  const router = useRouter();
  const { trade } = dt;
  const moneyness = calcMoneyness(trade);
  const row = computeRow(dt, executions, liveQuotes);
  const href = tradeHref(trade.symbol, date);

  return (
    <TableRow className={cn("cursor-pointer border-l-2 transition-colors hover:bg-muted/50", row.accentBorder)} onClick={() => router.push(href)}>
      <TableCell className="text-center text-sm tabular-nums text-muted-foreground">{trade.rank}</TableCell>
      <TableCell>
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="font-mono text-sm font-semibold">${trade.symbol}</span>
          <Badge variant="outline" className={cn(trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
            {trade.contract_type}
          </Badge>
          <Badge variant={row.resultVariant}>{row.resultLabel}</Badge>
          <Badge variant={moneyness.variant}>{moneyness.label}</Badge>
        </div>
      </TableCell>
      <TableCell className="text-center text-xs tabular-nums">
        <ScorePill score={trade.score} />
      </TableCell>
      <TableCell className="text-right font-mono text-sm tabular-nums">{row.entry}</TableCell>
      <TableCell className="text-right font-mono text-sm tabular-nums">{row.close}</TableCell>
      <TableCell className={cn("text-right font-mono text-sm tabular-nums", row.hasSummary ? pnlColor(row.stockMove) : "text-muted-foreground")}>
        {row.hasSummary ? fmtPctDec(row.stockMove) : "-"}
      </TableCell>
      <TableCell className={cn("text-right text-base font-semibold tabular-nums", row.hasSummary || row.pnl !== 0 || row.pnlPct !== 0 ? pnlColor(row.pnl) : "text-muted-foreground")}>
        {row.hasSummary || row.pnl !== 0 ? fmtPnlInt(row.pnl) : "-"}
      </TableCell>
      <TableCell className="w-10 text-right">
        <Link
          href={href}
          aria-label={`Open ${trade.symbol} detail`}
          onClick={(e) => e.stopPropagation()}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <ChevronRight className="h-4 w-4" />
        </Link>
      </TableCell>
    </TableRow>
  );
}

function ScorePill({ score }: { score: number }) {
  if (score === 0) {
    return <span className="text-muted-foreground">-</span>;
  }
  return (
    <span className="inline-flex items-center gap-1 rounded-md border bg-muted/40 px-1.5 py-0.5 font-semibold">
      <ClaudeLogo className="h-3 w-3" />
      <span>{score}/10</span>
    </span>
  );
}

// ---- Mobile card ----
function TradeRowCard({ dt, date, executions, liveQuotes }: { dt: DashboardTrade; date: string; executions?: Execution[] | null; liveQuotes?: LiveQuotesResponse | null }) {
  const { trade } = dt;
  const moneyness = calcMoneyness(trade);
  const row = computeRow(dt, executions, liveQuotes);
  const showPnl = row.hasSummary || row.pnl !== 0;

  return (
    <Link href={tradeHref(trade.symbol, date)} className="block">
      <Card
        className={cn(
          "animate-in fade-in fill-mode-backwards rounded-lg border border-border/60 bg-card/30 shadow-none duration-200 border-l-2 transition-colors hover:border-foreground/30 hover:bg-card/60",
          row.accentBorder
        )}
      >
        <CardContent className="space-y-3 p-4">
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant="secondary">#{trade.rank}</Badge>
            <span className="font-mono text-base font-semibold">${trade.symbol}</span>
            <Badge variant="outline" className={cn(trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
              {trade.contract_type}
            </Badge>
            <Badge variant={row.resultVariant}>{row.resultLabel}</Badge>
            <Badge variant={moneyness.variant}>{moneyness.label}</Badge>
            <ChevronRight className="ml-auto h-4 w-4 text-muted-foreground" aria-hidden />
          </div>

          <div className={cn("text-2xl font-semibold tabular-nums", showPnl ? pnlColor(row.pnl) : "text-muted-foreground")}>{showPnl ? fmtPnlInt(row.pnl) : "-"}</div>

          <div className="grid grid-cols-3 gap-3 text-sm">
            <Metric label="Entry" value={row.entry} />
            <Metric label="Close" value={row.close} />
            <Metric label="DTE" value={`${trade.dte}d`} />
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
