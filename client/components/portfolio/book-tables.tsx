"use client";

import { ChevronRight } from "lucide-react";
import { useRouter } from "next/navigation";
import Link from "next/link";

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { AnimatedNumber } from "@/components/ui/animated-number";
import { dayParts } from "@/lib/day-split";
import { fmtMoney, fmtPrice, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { ClosedTrade, Holding } from "@/types/portfolio";
import { KindChip, tradeHref } from "./trade-cards";

/*
Ledger tables for the book surfaces: holdings (split into an options
table and a stocks table, each with the columns that actually matter for
that instrument) and closed round trips. Editorial treatment, not data-
grid chrome: letter-spaced small-caps headers, hairline rules, mono
tabular numerals right-aligned, the whole row a navigation surface into
the trade detail. Columns that don't fit a phone collapse away (the
symbol, value, and P&L always survive).
*/

const headCls = "h-10 px-2.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-muted-foreground first:pl-0 last:pr-0";
const cellCls = "px-2.5 py-4 first:pl-0 last:pr-0";
const numCls = "text-right tabular-nums";

function useRowNav() {
  const router = useRouter();
  return (href: string) => ({
    onClick: () => router.push(href),
    className: "cursor-pointer border-border/50 transition-colors hover:bg-foreground/[0.03]",
  });
}

// PnlCell stacks the signed dollars over the signed percent, both
// animated, colored by sign.
function PnlCell({ pnl, pct }: { pnl: number; pct: number }) {
  return (
    <div className={cn("flex flex-col items-end gap-0.5", pnlColor(pnl))}>
      <span className="font-semibold tabular-nums">
        <AnimatedNumber value={pnl} kind="pnlInt" />
      </span>
      <span className="text-xs tabular-nums">
        <AnimatedNumber value={pct} kind="pctSigned1" />
      </span>
    </div>
  );
}

/*
DayCell stacks the session move over the overnight gap, each colored by
its own sign, with the honest fallbacks: an unsplittable change since
yesterday's close shows as one "day" figure, a position opened today
shows its whole life as the session.
*/
function DayCell({ h }: { h: Holding }) {
  const parts = dayParts(h);
  if (parts.day !== null) {
    return (
      <div className={cn("flex flex-col items-end gap-0.5 tabular-nums", pnlColor(parts.day))}>
        <span className="text-sm font-medium">
          <AnimatedNumber value={parts.day} kind="pnlInt" />
        </span>
        <span className="text-[10px] tracking-wider text-muted-foreground uppercase">day</span>
      </div>
    );
  }
  if (parts.today === null) return <span className="text-muted-foreground">-</span>;
  return (
    <div className="flex flex-col items-end gap-0.5 tabular-nums">
      <span className={cn("text-sm font-medium", pnlColor(parts.today))}>
        <AnimatedNumber value={parts.today} kind="pnlInt" />
        <span className="ml-1 text-[10px] font-normal tracking-wider text-muted-foreground uppercase">today</span>
      </span>
      <span className={cn("text-xs", (parts.overnight ?? 0) === 0 ? "text-muted-foreground" : pnlColor(parts.overnight ?? 0))}>
        <AnimatedNumber value={parts.overnight ?? 0} kind="pnlInt" />
        <span className="ml-1 text-[10px] tracking-wider text-muted-foreground uppercase">o/n</span>
      </span>
    </div>
  );
}

// SymbolCell carries the row's identity + the real link (keyboard / SEO),
// while the row itself is the pointer target.
function SymbolCell({ href, label, chip }: { href: string; label: string; chip: React.ReactNode }) {
  return (
    <div className="flex items-center gap-2">
      <Link href={href} className="font-semibold hover:underline" onClick={(e) => e.stopPropagation()}>
        {label}
      </Link>
      {chip}
    </div>
  );
}

export function OptionsTable({ items, total }: { items: Holding[]; total?: number }) {
  const rowNav = useRowNav();
  if (items.length === 0) return null;
  return (
    <section className="mt-8">
      <h2 className="mb-2.5 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">Options ({total ?? items.length})</h2>
      <Table>
        <TableHeader>
          <TableRow className="border-border hover:bg-transparent">
            <TableHead className={headCls}>Contract</TableHead>
            <TableHead className={cn(headCls, "text-right")}>Qty</TableHead>
            <TableHead className={cn(headCls, "text-right max-sm:hidden")}>Strike</TableHead>
            <TableHead className={cn(headCls, "max-sm:hidden")}>Expiry</TableHead>
            <TableHead className={cn(headCls, "text-right max-md:hidden")}>Day</TableHead>
            <TableHead className={cn(headCls, "text-right")}>Value</TableHead>
            <TableHead className={cn(headCls, "text-right")}>Unrealized</TableHead>
            <TableHead className={cn(headCls, "w-6")} aria-hidden />
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((h) => {
            const href = tradeHref("holdings", h.id);
            const pct = h.cost_basis > 0 ? (h.unrealized_pnl / h.cost_basis) * 100 : 0;
            return (
              <TableRow key={h.id} {...rowNav(href)}>
                <TableCell className={cellCls}>
                  <SymbolCell href={href} label={h.underlying} chip={<KindChip kind={h.kind} contractType={h.contract_type} />} />
                </TableCell>
                <TableCell className={cn(cellCls, numCls)}>{h.quantity}</TableCell>
                <TableCell className={cn(cellCls, numCls, "max-sm:hidden")}>{h.strike ? fmtPrice(h.strike) : "-"}</TableCell>
                <TableCell className={cn(cellCls, "max-sm:hidden")}>
                  <span className="whitespace-nowrap">{h.expiration || "-"}</span>
                  {h.dte != null && <span className="ml-2 text-xs text-muted-foreground">{h.dte}d left</span>}
                </TableCell>
                <TableCell className={cn(cellCls, numCls, "max-md:hidden")}>
                  <DayCell h={h} />
                </TableCell>
                <TableCell className={cn(cellCls, numCls, "font-semibold")}>
                  <AnimatedNumber value={h.market_value} kind="money" />
                </TableCell>
                <TableCell className={cn(cellCls, numCls)}>
                  <PnlCell pnl={h.unrealized_pnl} pct={pct} />
                </TableCell>
                <TableCell className={cn(cellCls, "w-6 pr-0")}>
                  <ChevronRight className="h-4 w-4 text-muted-foreground" aria-hidden />
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </section>
  );
}

export function StocksTable({ items, total }: { items: Holding[]; total?: number }) {
  const rowNav = useRowNav();
  if (items.length === 0) return null;
  return (
    <section className="mt-8">
      <h2 className="mb-2.5 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">Stocks ({total ?? items.length})</h2>
      <Table>
        <TableHeader>
          <TableRow className="border-border hover:bg-transparent">
            <TableHead className={headCls}>Symbol</TableHead>
            <TableHead className={cn(headCls, "text-right")}>Shares</TableHead>
            <TableHead className={cn(headCls, "text-right max-sm:hidden")}>Avg cost</TableHead>
            <TableHead className={cn(headCls, "max-sm:hidden")}>Opened</TableHead>
            <TableHead className={cn(headCls, "text-right max-md:hidden")}>Day</TableHead>
            <TableHead className={cn(headCls, "text-right")}>Value</TableHead>
            <TableHead className={cn(headCls, "text-right")}>Unrealized</TableHead>
            <TableHead className={cn(headCls, "w-6")} aria-hidden />
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((h) => {
            const href = tradeHref("holdings", h.id);
            const pct = h.cost_basis > 0 ? (h.unrealized_pnl / h.cost_basis) * 100 : 0;
            return (
              <TableRow key={h.id} {...rowNav(href)}>
                <TableCell className={cellCls}>
                  <SymbolCell href={href} label={h.label} chip={<KindChip kind={h.kind} contractType={h.contract_type} />} />
                </TableCell>
                <TableCell className={cn(cellCls, numCls)}>{h.quantity}</TableCell>
                <TableCell className={cn(cellCls, numCls, "max-sm:hidden")}>{h.quantity > 0 ? fmtMoney(h.cost_basis / h.quantity) : "-"}</TableCell>
                <TableCell className={cn(cellCls, "whitespace-nowrap max-sm:hidden")}>{h.opened_date || "-"}</TableCell>
                <TableCell className={cn(cellCls, numCls, "max-md:hidden")}>
                  <DayCell h={h} />
                </TableCell>
                <TableCell className={cn(cellCls, numCls, "font-semibold")}>
                  <AnimatedNumber value={h.market_value} kind="money" />
                </TableCell>
                <TableCell className={cn(cellCls, numCls)}>
                  <PnlCell pnl={h.unrealized_pnl} pct={pct} />
                </TableCell>
                <TableCell className={cn(cellCls, "w-6 pr-0")}>
                  <ChevronRight className="h-4 w-4 text-muted-foreground" aria-hidden />
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </section>
  );
}

export function ClosedTable({ items }: { items: ClosedTrade[] }) {
  const rowNav = useRowNav();
  if (items.length === 0) return null;
  return (
    <Table>
      <TableHeader>
        <TableRow className="border-border hover:bg-transparent">
          <TableHead className={headCls}>Trade</TableHead>
          <TableHead className={cn(headCls, "max-sm:hidden")}>Held</TableHead>
          <TableHead className={cn(headCls, "text-right max-sm:hidden")}>Entry</TableHead>
          <TableHead className={cn(headCls, "text-right max-sm:hidden")}>Exit</TableHead>
          <TableHead className={cn(headCls, "text-right")}>Realized</TableHead>
          <TableHead className={cn(headCls, "text-right")}>Return</TableHead>
          <TableHead className={cn(headCls, "w-6")} aria-hidden />
        </TableRow>
      </TableHeader>
      <TableBody>
        {items.map((t) => {
          const href = tradeHref("closed", t.id);
          // Win = strictly profitable; a $0 scratch counts as a loss, matching
          // the win-rate stats (server trades.Summarize + dashboard strip).
          const win = t.realized_pnl > 0;
          return (
            <TableRow key={t.id} {...rowNav(href)}>
              <TableCell className={cellCls}>
                <SymbolCell
                  href={href}
                  label={t.label}
                  chip={
                    <span className="flex items-center gap-1.5">
                      <KindChip kind={t.kind} contractType={t.contract_type} />
                      <span className={cn("rounded-full border px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider", win ? "border-green-border bg-green-bg text-green" : "border-red-border bg-red-bg text-red")}>
                        {win ? "Win" : "Loss"}
                      </span>
                    </span>
                  }
                />
              </TableCell>
              <TableCell className={cn(cellCls, "max-sm:hidden")}>
                <span className="whitespace-nowrap">
                  {t.opened_date} to {t.closed_date}
                </span>
                <span className="ml-2 text-xs text-muted-foreground">{t.hold_days}d</span>
              </TableCell>
              <TableCell className={cn(cellCls, numCls, "max-sm:hidden")}>{t.entry_price > 0 ? fmtMoney(t.entry_price) : "-"}</TableCell>
              <TableCell className={cn(cellCls, numCls, "max-sm:hidden")}>{t.exit_price > 0 ? fmtMoney(t.exit_price) : "-"}</TableCell>
              <TableCell className={cn(cellCls, numCls, "font-semibold", pnlColor(t.realized_pnl))}>
                <AnimatedNumber value={t.realized_pnl} kind="pnlInt" />
              </TableCell>
              <TableCell className={cn(cellCls, numCls, pnlColor(t.realized_pnl))}>
                <AnimatedNumber value={t.realized_pct} kind="pctSigned1" />
              </TableCell>
              <TableCell className={cn(cellCls, "w-6 pr-0")}>
                <ChevronRight className="h-4 w-4 text-muted-foreground" aria-hidden />
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}
