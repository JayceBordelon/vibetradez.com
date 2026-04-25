"use client";

import { ArrowRight } from "lucide-react";
import Link from "next/link";

import { Badge } from "@/components/ui/badge";
import { ClaudeLogo, OpenAILogo } from "@/components/ui/brand-icons";
import { Card, CardContent } from "@/components/ui/card";
import { Metric } from "@/components/ui/metric";
import { calcMoneyness } from "@/lib/calculations";
import { fmt, fmtMoney, fmtMoneyInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade, LiveQuotesResponse } from "@/types/trade";

interface MorningCardsProps {
  trades: DashboardTrade[];
  liveQuotes?: LiveQuotesResponse | null;
  date: string;
}

function tradeHref(symbol: string, date: string): string {
  return `/trade/${encodeURIComponent(symbol)}?date=${encodeURIComponent(date)}`;
}

export function MorningCards({ trades, liveQuotes, date }: MorningCardsProps) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {trades.map((dt) => (
        <MorningCard key={dt.trade.symbol} dt={dt} liveQuotes={liveQuotes} date={date} />
      ))}
    </div>
  );
}

interface MorningCardProps {
  dt: DashboardTrade;
  liveQuotes?: LiveQuotesResponse | null;
  date: string;
}

function MorningCard({ dt, liveQuotes, date }: MorningCardProps) {
  const { trade } = dt;
  const moneyness = calcMoneyness(trade);

  const liveStock = liveQuotes?.quotes?.[trade.symbol];
  const liveStockChangeColor = liveStock ? pnlColor(liveStock.net_change) : "";

  const stockPriceValue = liveStock ? (
    <span className={cn("text-sm font-semibold tabular-nums", liveStockChangeColor)}>
      {fmtMoney(liveStock.last_price)}
      {liveStock.net_change !== 0 && <span className="ml-1 text-xs">{liveStock.net_change > 0 ? "↑" : "↓"}</span>}
    </span>
  ) : (
    fmtMoney(trade.current_price)
  );

  const riskBadgeVariant: "destructive" | "outline" | "secondary" = trade.risk_level === "HIGH" ? "destructive" : trade.risk_level === "MEDIUM" ? "outline" : "secondary";
  const hasDualScore = trade.gpt_score > 0 && trade.claude_score > 0;

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
            {hasDualScore && (
              <div className="ml-auto flex items-center gap-1.5 rounded-md border bg-muted/40 px-2 py-0.5 text-[11px] font-semibold tabular-nums">
                <OpenAILogo className="h-3 w-3" />
                <span>{trade.gpt_score}</span>
                <span className="text-muted-foreground">·</span>
                <ClaudeLogo className="h-3 w-3" />
                <span>{trade.claude_score}</span>
              </div>
            )}
          </div>

          <div>
            <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">Est. Premium</div>
            <div className="mt-1 text-[32px] font-semibold leading-none tabular-nums">{fmtMoney(trade.estimated_price)}</div>
            <div className="mt-1 text-xs text-muted-foreground">{fmtMoneyInt(trade.estimated_price * 100)} per contract</div>
          </div>

          <div className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
            <Metric label="Strike" value={fmtMoney(trade.strike_price)} />
            <Metric label="Expiration" value={`${trade.expiration} (${trade.dte}d)`} />
            <Metric label="Stock Price" value={stockPriceValue} />
            <Metric label="Target" value={fmtMoney(trade.target_price)} />
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
