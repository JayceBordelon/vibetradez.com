"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { ExecutionBadge } from "@/components/execution-badge";
import { Badge } from "@/components/ui/badge";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { formatDayName, formatMonthDay } from "@/lib/date-utils";
import { fmtMoney, fmtPctDec, fmtPnlInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";

import type { DayMultiStat, TradeDetail } from "./history-shell";
import { TIER_COLORS, TIER_KEYS, TIER_LABELS, type TierKey } from "./tiers";

const PAGE_SIZE = 25;

export function DailyBreakdown({ days }: { days: DayMultiStat[] }) {
  const maxAbsPnl = useMemo(() => {
    let m = 1;
    for (const d of days) {
      for (const tier of TIER_KEYS) {
        const v = Math.abs(d.tiers[tier].pnl);
        if (v > m) m = v;
      }
    }
    return m;
  }, [days]);

  const totalPages = Math.max(1, Math.ceil(days.length / PAGE_SIZE));
  const [page, setPage] = useState(0);

  useEffect(() => {
    setPage(0);
  }, []);
  useEffect(() => {
    if (page >= totalPages) setPage(0);
  }, [page, totalPages]);

  const start = page * PAGE_SIZE;
  const end = Math.min(start + PAGE_SIZE, days.length);
  const visible = days.slice(start, end);
  const showPagination = days.length > PAGE_SIZE;

  return (
    <div className="space-y-3">
      <div className="space-y-1">
        {visible.map((d) => (
          <DayRow key={d.date} day={d} maxAbsPnl={maxAbsPnl} />
        ))}
      </div>

      {showPagination ? (
        <div className="flex flex-col items-center justify-between gap-2 pt-2 sm:flex-row">
          <div className="text-[11px] text-muted-foreground tabular-nums">
            Showing {start + 1}–{end} of {days.length} days
          </div>
          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              className={cn(
                "lg-control inline-flex h-8 items-center gap-1 rounded-md px-2.5 text-xs font-semibold transition-colors",
                page === 0 ? "cursor-not-allowed text-muted-foreground/40" : "text-foreground hover:bg-muted/50"
              )}
              aria-label="Previous page"
            >
              <ChevronLeft className="h-3.5 w-3.5" aria-hidden />
              Prev
            </button>
            <div className="px-2 font-mono text-xs text-muted-foreground tabular-nums">
              {page + 1} / {totalPages}
            </div>
            <button
              type="button"
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={page >= totalPages - 1}
              className={cn(
                "lg-control inline-flex h-8 items-center gap-1 rounded-md px-2.5 text-xs font-semibold transition-colors",
                page >= totalPages - 1 ? "cursor-not-allowed text-muted-foreground/40" : "text-foreground hover:bg-muted/50"
              )}
              aria-label="Next page"
            >
              Next
              <ChevronRight className="h-3.5 w-3.5" aria-hidden />
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function TierBar({ tier, pnl, maxAbsPnl }: { tier: TierKey; pnl: number; maxAbsPnl: number }) {
  const width = Math.round((Math.abs(pnl) / maxAbsPnl) * 100);
  const color = TIER_COLORS[tier];
  const positive = pnl >= 0;
  return (
    <div className="flex items-center gap-2">
      <span className="hidden w-12 shrink-0 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground sm:inline">{TIER_LABELS[tier]}</span>
      <span className="inline-flex h-2 w-2 shrink-0 rounded-full sm:hidden" style={{ backgroundColor: color }} aria-hidden />
      <div className="relative h-2.5 flex-1 overflow-hidden rounded-sm bg-muted/40">
        <div
          className="h-full"
          style={{
            width: `${width}%`,
            backgroundColor: color,
            opacity: positive ? 0.85 : 0.55,
          }}
        />
      </div>
      <span className={cn("w-16 shrink-0 text-right text-[11px] font-semibold tabular-nums", pnlColor(pnl))}>{fmtPnlInt(pnl)}</span>
    </div>
  );
}

function DayRow({ day, maxAbsPnl }: { day: DayMultiStat; maxAbsPnl: number }) {
  const top10 = day.tiers.top10;

  return (
    <Collapsible className="animate-in fade-in fill-mode-backwards duration-200">
      <CollapsibleTrigger className={cn("group flex w-full items-center gap-3 rounded-md px-3 py-2.5 text-left transition-colors hover:bg-muted/40")}>
        <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />

        <div className="w-20 shrink-0">
          <div className="text-sm font-semibold">{formatDayName(day.date)}</div>
          <div className="text-[11px] text-muted-foreground">{formatMonthDay(day.date)}</div>
        </div>

        <div className="hidden w-32 shrink-0 text-[11px] text-muted-foreground md:block">
          {day.totalTrades} picks
          {top10.hasSummaries ? ` · ${top10.winners}W/${top10.losers}L` : ""}
        </div>

        <div className="flex-1 space-y-1">
          {TIER_KEYS.map((tier) => (
            <TierBar key={tier} tier={tier} pnl={day.tiers[tier].pnl} maxAbsPnl={maxAbsPnl} />
          ))}
        </div>

        {day.executions.length > 0 && (
          // Badges visible on every breakpoint. The previous `hidden sm:flex`
          // dropped them entirely on mobile, which silently hid the
          // "Close failed (verify position)" warning for stranded-at-broker
          // positions — operator browsing /history on a phone wouldn't see
          // it. The flex-wrap keeps the layout sane when many badges land.
          <div className="flex shrink-0 flex-wrap items-center gap-1">
            {day.executions.map((e, i) => (
              <ExecutionBadge key={`${day.date}-exec-${i}`} execution={e} />
            ))}
          </div>
        )}
      </CollapsibleTrigger>

      <CollapsibleContent className="overflow-hidden data-[state=closed]:animate-collapsible-up data-[state=open]:animate-collapsible-down">
        <div className="ml-7 mt-1 border-l border-border/50 pl-4 py-2">
          {top10.details.length > 0 ? <TradeList details={top10.details} date={day.date} /> : <div className="py-2 text-[11px] text-muted-foreground">No trade details available for this day.</div>}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

function tierForRank(rank: number): TierKey {
  if (rank === 1) return "top1";
  if (rank <= 3) return "top3";
  return "top10";
}

function TradeList({ details, date }: { details: TradeDetail[]; date: string }) {
  const sorted = useMemo(() => [...details].sort((a, b) => a.rank - b.rank), [details]);
  return (
    <div className="space-y-1.5">
      {sorted.map((t, j) => (
        <TradeRow key={`${date}-${j}`} trade={t} date={date} />
      ))}
    </div>
  );
}

function TradeRow({ trade: t, date }: { trade: TradeDetail; date: string }) {
  const href = `/trade/${encodeURIComponent(t.symbol)}?date=${encodeURIComponent(date)}`;
  const tier = tierForRank(t.rank);
  const tierColor = TIER_COLORS[tier];
  return (
    <Link
      href={href}
      className="-mx-2 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-md px-2 py-1.5 text-[13px] transition-colors hover:bg-muted/60 sm:flex-nowrap"
      aria-label={`Open ${t.symbol} ${t.type} ${date} detail`}
    >
      <span
        className="inline-flex h-5 w-7 shrink-0 items-center justify-center rounded-sm text-[10px] font-semibold text-white"
        style={{ backgroundColor: tierColor }}
        title={`Rank ${t.rank} · ${TIER_LABELS[tier]}`}
      >
        #{t.rank}
      </span>

      <span className="w-14 shrink-0 font-semibold">${t.symbol}</span>

      <Badge variant="outline" className={cn("w-11 justify-center text-[11px]", t.type === "CALL" ? "border-green/40 text-green" : "border-red/40 text-red")}>
        {t.type}
      </Badge>

      <span className="w-14 shrink-0 text-[11px] text-foreground/80 tabular-nums">${t.strike}</span>

      <span className="hidden w-32 shrink-0 text-[11px] text-foreground/80 tabular-nums sm:inline">
        {fmtMoney(t.entry)} <span className="text-muted-foreground/60">&rarr;</span> {fmtMoney(t.close)}
      </span>

      <span className={cn("ml-auto w-16 shrink-0 text-right text-[13px] tabular-nums", pnlColor(t.pnl))}>{fmtPctDec(t.pct)}</span>

      <span className={cn("w-20 shrink-0 text-right text-sm font-semibold tabular-nums", pnlColor(t.pnl))}>{fmtPnlInt(t.pnl)}</span>

      <ChevronRight className="hidden h-3.5 w-3.5 shrink-0 text-muted-foreground sm:inline" aria-hidden />
    </Link>
  );
}
