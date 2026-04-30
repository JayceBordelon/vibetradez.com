"use client";

import { ArrowRight } from "lucide-react";
import Link from "next/link";

import { ExecutionBadge, matchesTrade } from "@/components/execution-badge";
import { Badge } from "@/components/ui/badge";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { Card, CardContent } from "@/components/ui/card";
import { calcMoneyness } from "@/lib/calculations";
import { fmtMoney, fmtMoneyInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade, Execution, LiveQuotesResponse } from "@/types/trade";

interface MorningCardsProps {
  trades: DashboardTrade[];
  liveQuotes?: LiveQuotesResponse | null;
  date: string;
  execution?: Execution | null;
}

function tradeHref(symbol: string, date: string): string {
  return `/trade/${encodeURIComponent(symbol)}?date=${encodeURIComponent(date)}`;
}

export function MorningCards({ trades, liveQuotes, date, execution }: MorningCardsProps) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {trades.map((dt) => (
        <MorningCard key={dt.trade.symbol} dt={dt} liveQuotes={liveQuotes} date={date} execution={matchesTrade(execution, dt.trade) ? execution : null} />
      ))}
    </div>
  );
}

interface MorningCardProps {
  dt: DashboardTrade;
  liveQuotes?: LiveQuotesResponse | null;
  date: string;
  execution?: Execution | null;
}

function MorningCard({ dt, liveQuotes, date, execution }: MorningCardProps) {
  const { trade } = dt;
  const moneyness = calcMoneyness(trade);

  /**
  Live option mark for "current contract price". Backend keys are
  "<SYMBOL>|<CALL|PUT>|<strike formatted to 2dp>|<expiration>" — see
  server.go:846 — so reconstruct the same key here. Falls back to
  null (em-dash) when Schwab isn't connected or the contract dropped
  off the chain.
  */
  const optionKey = `${trade.symbol}|${trade.contract_type}|${trade.strike_price.toFixed(2)}|${trade.expiration}`;
  const liveOption = liveQuotes?.options?.[optionKey] ?? null;
  const currentContractPrice = liveOption?.mark ?? null;
  const contractDelta = currentContractPrice !== null ? currentContractPrice - trade.estimated_price : null;
  const contractDeltaPct = contractDelta !== null && trade.estimated_price > 0 ? (contractDelta / trade.estimated_price) * 100 : null;

  const currentValue =
    currentContractPrice !== null ? (
      <span className={cn("text-sm font-semibold tabular-nums", pnlColor(contractDelta ?? 0))}>
        {fmtMoney(currentContractPrice)}
        {contractDeltaPct !== null && (
          <span className="ml-1 text-[11px]">
            ({contractDeltaPct > 0 ? "+" : ""}
            {contractDeltaPct.toFixed(1)}%)
          </span>
        )}
      </span>
    ) : (
      <span className="text-sm font-medium text-muted-foreground">—</span>
    );

  const riskBadgeVariant: "destructive" | "outline" | "secondary" = trade.risk_level === "HIGH" ? "destructive" : trade.risk_level === "MEDIUM" ? "outline" : "secondary";
  const showScore = trade.score > 0;

  return (
    <Link href={tradeHref(trade.symbol, date)} className="block">
      <Card className="group h-full animate-in fade-in fill-mode-backwards duration-200 transition-all hover:-translate-y-0.5 hover:border-foreground/30 hover:shadow-md">
        <CardContent className="space-y-4 p-5">
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant="secondary">#{trade.rank}</Badge>
            <span className="text-xl font-bold tracking-tight">${trade.symbol}</span>
            <Badge variant="outline" className={cn(trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
              {trade.contract_type}
            </Badge>
            <Badge variant={moneyness.variant}>{moneyness.label}</Badge>
            <Badge variant={riskBadgeVariant}>{trade.risk_level}</Badge>
            {execution && <ExecutionBadge execution={execution} />}
            {showScore && (
              <div className="ml-auto inline-flex items-center gap-1 rounded-md border bg-muted/40 px-2 py-0.5 text-[11px] font-semibold tabular-nums">
                <ClaudeLogo className="h-3 w-3" />
                <span>{trade.score}/10</span>
              </div>
            )}
          </div>

          <div className="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
            <div>
              <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Buy</div>
              <div className="mt-0.5 text-2xl font-semibold leading-none tabular-nums">{fmtMoney(trade.estimated_price)}</div>
              <div className="mt-0.5 text-[11px] text-muted-foreground">{fmtMoneyInt(trade.estimated_price * 100)} / contract</div>
            </div>
            <div>
              <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Current</div>
              <div className="mt-0.5 text-2xl font-semibold leading-none tabular-nums">{currentValue}</div>
              <div className="mt-0.5 text-[11px] text-muted-foreground">{currentContractPrice !== null ? fmtMoneyInt(currentContractPrice * 100) : "—"} / contract</div>
            </div>
          </div>

          {trade.catalyst && (
            <div className="rounded-md bg-amber-bg px-3 py-2 text-sm">
              <span className="font-semibold text-amber">Catalyst:</span> {trade.catalyst}
            </div>
          )}

          {trade.thesis && <p className="text-sm leading-relaxed text-muted-foreground">{trade.thesis}</p>}

          <div className="flex items-center justify-between border-t pt-3 text-xs font-medium text-muted-foreground transition-colors group-hover:text-foreground">
            <span>View full contract</span>
            <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
