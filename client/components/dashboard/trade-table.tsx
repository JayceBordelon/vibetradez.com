"use client";

import { ChevronRight } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { Badge } from "@/components/ui/badge";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { Card, CardContent } from "@/components/ui/card";
import { Metric } from "@/components/ui/metric";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { calcMoneyness } from "@/lib/calculations";
import { fmtMoney, fmtPctDec, fmtPnlInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade } from "@/types/trade";

interface TradeTableProps {
  trades: DashboardTrade[];
  date: string;
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

function computeRow(dt: DashboardTrade): RowComputed {
  const { trade, summary } = dt;
  const hasSummary = !!summary;
  const pnl = hasSummary ? (summary.closing_price - summary.entry_price) * 100 : 0;
  const pnlPct = hasSummary ? ((summary.closing_price - summary.entry_price) / summary.entry_price) * 100 : 0;
  const stockMove = hasSummary ? ((summary.stock_close - summary.stock_open) / summary.stock_open) * 100 : 0;

  const resultLabel = hasSummary ? (pnl > 0 ? "PROFIT" : pnl < 0 ? "LOSS" : "FLAT") : "OPEN";
  const resultVariant: RowComputed["resultVariant"] = hasSummary ? (pnl > 0 ? "default" : pnl < 0 ? "destructive" : "outline") : "secondary";

  const accentBorder = !hasSummary ? "border-l-transparent" : pnlPct > 1 ? "border-l-green/40" : pnlPct < -1 ? "border-l-red/40" : "border-l-transparent";

  return {
    hasSummary,
    pnl,
    pnlPct,
    stockMove,
    resultLabel,
    resultVariant,
    accentBorder,
    entry: hasSummary ? fmtMoney(summary.entry_price) : fmtMoney(trade.estimated_price),
    close: hasSummary ? fmtMoney(summary.closing_price) : "-",
  };
}

export function TradeTable({ trades, date }: TradeTableProps) {
  return (
    <div className="min-w-0">
      {/* Desktop table */}
      <div className="hidden md:block">
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
              <DesktopTradeRow key={dt.trade.symbol} dt={dt} date={date} />
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Mobile cards */}
      <div className="space-y-3 md:hidden">
        {trades.map((dt) => (
          <TradeRowCard key={dt.trade.symbol} dt={dt} date={date} />
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
function DesktopTradeRow({ dt, date }: { dt: DashboardTrade; date: string }) {
  const router = useRouter();
  const { trade } = dt;
  const moneyness = calcMoneyness(trade);
  const row = computeRow(dt);
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
      <TableCell className={cn("text-right text-base font-semibold tabular-nums", row.hasSummary ? pnlColor(row.pnl) : "text-muted-foreground")}>{row.hasSummary ? fmtPnlInt(row.pnl) : "-"}</TableCell>
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
function TradeRowCard({ dt, date }: { dt: DashboardTrade; date: string }) {
  const { trade } = dt;
  const moneyness = calcMoneyness(trade);
  const row = computeRow(dt);

  return (
    <Link href={tradeHref(trade.symbol, date)} className="block">
      <Card className={cn("animate-in fade-in fill-mode-backwards duration-200 border-l-2 transition-colors hover:bg-muted/40", row.accentBorder)}>
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

          <div className={cn("text-2xl font-semibold tabular-nums", row.hasSummary ? pnlColor(row.pnl) : "text-muted-foreground")}>{row.hasSummary ? fmtPnlInt(row.pnl) : "-"}</div>

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
