"use client";

import { ChevronRight } from "lucide-react";
import Link from "next/link";

import { ExecutionBadge } from "@/components/execution-badge";
import { Badge } from "@/components/ui/badge";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { formatDayName, formatMonthDay } from "@/lib/date-utils";
import { fmtMoney, fmtPctDec, fmtPnlInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Execution } from "@/types/trade";

interface TradeDetail {
  symbol: string;
  type: string;
  strike: number;
  entry: number;
  close: number;
  pnl: number;
  pct: number;
  result: string;
}

interface DayStat {
  date: string;
  pnl: number;
  winners: number;
  losers: number;
  trades: number;
  hasSummaries: boolean;
  invested: number;
  returned: number;
  execution: Execution | null;
  details: TradeDetail[];
}

export function DailyBreakdown({ dayStats }: { dayStats: DayStat[] }) {
  const maxAbsPnl = Math.max(...dayStats.map((d) => Math.abs(d.pnl)), 1);

  return (
    <div className="space-y-1">
      {dayStats.map((ds) => (
        <DayRow key={ds.date} ds={ds} maxAbsPnl={maxAbsPnl} />
      ))}
    </div>
  );
}

function DayRow({ ds, maxAbsPnl }: { ds: DayStat; maxAbsPnl: number }) {
  const barWidth = Math.round((Math.abs(ds.pnl) / maxAbsPnl) * 100);
  const isPositive = ds.pnl >= 0;

  return (
    <Collapsible className="animate-in fade-in fill-mode-backwards duration-200">
      <CollapsibleTrigger className={cn("lg-control group flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-muted/50")}>
        <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />

        <div className="w-20 shrink-0">
          <div className="text-sm font-semibold">{formatDayName(ds.date)}</div>
          <div className="text-[11px] text-muted-foreground">{formatMonthDay(ds.date)}</div>
        </div>

        <div className="hidden w-32 shrink-0 text-[11px] text-muted-foreground sm:block">
          {ds.trades} trades
          {ds.hasSummaries ? ` \u00B7 ${ds.winners}W/${ds.losers}L` : ""}
        </div>

        <div className="relative h-8 flex-1 overflow-hidden rounded-md bg-muted/40">
          <div className={cn("h-full", isPositive ? "bg-green/60" : "bg-red/60")} style={{ width: `${barWidth}%` }} />
        </div>

        {ds.execution && (
          <div className="hidden shrink-0 sm:block">
            <ExecutionBadge execution={ds.execution} />
          </div>
        )}

        <div className={cn("w-20 shrink-0 text-right text-sm font-semibold tabular-nums", pnlColor(ds.pnl))}>{fmtPnlInt(ds.pnl)}</div>
      </CollapsibleTrigger>

      <CollapsibleContent className="overflow-hidden data-[state=closed]:animate-collapsible-up data-[state=open]:animate-collapsible-down">
        <div className="mt-1 rounded-md border bg-muted/30 px-4 py-3">
          {ds.details.length > 0 ? (
            <div className="space-y-2">
              {ds.details.map((t, j) => (
                <TradeRow key={`${ds.date}-${j}`} trade={t} date={ds.date} />
              ))}
            </div>
          ) : (
            <div className="py-2 text-center text-[11px] text-muted-foreground">No trade details available for this day.</div>
          )}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

function TradeRow({ trade: t, date }: { trade: TradeDetail; date: string }) {
  const href = `/trade/${encodeURIComponent(t.symbol)}?date=${encodeURIComponent(date)}`;
  return (
    <Link
      href={href}
      className="-mx-2 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-md px-2 py-1.5 text-[13px] transition-colors hover:bg-muted/60 sm:flex-nowrap"
      aria-label={`Open ${t.symbol} ${t.type} ${date} detail`}
    >
      <span className="w-14 shrink-0 font-semibold">${t.symbol}</span>

      <Badge variant="outline" className={cn("w-11 justify-center text-[11px]", t.type === "CALL" ? "border-green/40 text-green" : "border-red/40 text-red")}>
        {t.type}
      </Badge>

      <span className="w-14 shrink-0 text-[11px] text-muted-foreground">${t.strike}</span>

      <span className="hidden w-32 shrink-0 text-[11px] text-muted-foreground sm:inline">
        {fmtMoney(t.entry)} <span className="text-muted-foreground/70">&rarr;</span> {fmtMoney(t.close)}
      </span>

      <span className={cn("ml-auto w-16 shrink-0 text-right text-[13px] tabular-nums", pnlColor(t.pnl))}>{fmtPctDec(t.pct)}</span>

      <span className={cn("w-20 shrink-0 text-right text-sm font-semibold tabular-nums", pnlColor(t.pnl))}>{fmtPnlInt(t.pnl)}</span>

      <ChevronRight className="hidden h-3.5 w-3.5 shrink-0 text-muted-foreground sm:inline" aria-hidden />
    </Link>
  );
}
