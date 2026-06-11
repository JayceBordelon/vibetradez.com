"use client";

import { CheckCircle2, ChevronRight, XCircle } from "lucide-react";
import { useRouter } from "next/navigation";
import Link from "next/link";

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { AnimatedNumber } from "@/components/ui/animated-number";
import { fmtPrice } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { PortfolioDecision } from "@/types/portfolio";

/*
TodaysExecutions is the day's tape as a ledger table: one row per move,
action ink on the left, mono numerals right-aligned, the rationale as a
truncating middle column (full text on hover), and quiet status marks.
Money moves lead; holds settle to the bottom as the quiet coda. A row
whose move belongs to a trade navigates to that holding or closed round
trip.
*/

type StatusKind = "filled" | "working" | "canceled" | "";

// The decision log stores the broker's RAW status vocabulary (reconciled
// by the risk-sweep cron), so classify by the terminal sets and treat any
// other non-empty status as in-flight rather than allow-listing the
// broker's dozen AWAITING_*/PENDING_* variants.
function statusKind(status?: string): StatusKind {
  const s = (status ?? "").toUpperCase();
  if (s === "") return "";
  if (s === "FILLED") return "filled";
  if (s === "CANCELED" || s === "REJECTED" || s === "EXPIRED" || s === "REPLACED") return "canceled";
  return "working";
}

const ACTION_INK: Record<string, { label: string; text: string }> = {
  buy_equity: { label: "Buy", text: "text-green" },
  buy_option: { label: "Buy to open", text: "text-green" },
  sell_equity: { label: "Sell", text: "text-red" },
  sell_option: { label: "Sell to close", text: "text-red" },
  cancel_order: { label: "Cancel", text: "text-amber" },
  hold: { label: "Hold", text: "text-muted-foreground" },
};

function StatusMark({ kind }: { kind: StatusKind }) {
  if (kind === "filled") {
    return (
      <span className="inline-flex items-center gap-1 text-[10px] font-bold uppercase tracking-wider text-green">
        <CheckCircle2 className="h-3 w-3" aria-hidden />
        Filled
      </span>
    );
  }
  if (kind === "working") {
    return (
      <span className="inline-flex items-center gap-1.5 text-[10px] font-bold uppercase tracking-wider text-amber">
        <span aria-hidden className="h-1.5 w-1.5 animate-pulse rounded-full bg-amber" />
        Working
      </span>
    );
  }
  if (kind === "canceled") {
    return (
      <span className="inline-flex items-center gap-1 text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
        <XCircle className="h-3 w-3" aria-hidden />
        Canceled
      </span>
    );
  }
  return null;
}

const headCls = "h-9 px-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground first:pl-0 last:pr-0";
const cellCls = "px-2 py-3 first:pl-0 last:pr-0";

function ExecutionRow({ d }: { d: PortfolioDecision }) {
  const router = useRouter();
  const ink = ACTION_INK[d.action] ?? { label: d.action, text: "text-foreground/90" };
  const isHold = d.action === "hold";
  const contract = d.contract_type ? [d.contract_type, d.strike ? `${fmtPrice(d.strike)} strike` : null, d.expiration ? `exp ${d.expiration}` : null].filter(Boolean).join(" · ") : null;
  const qty = d.quantity ?? 0;
  const showNumbers = !isHold && qty > 0 && (d.limit_price ?? 0) > 0;
  const href = d.trade_id ? `/${d.trade_kind === "closed" ? "closed" : "holdings"}?trade=${encodeURIComponent(d.trade_id)}` : null;

  return (
    <TableRow
      className={cn("border-border/50 transition-colors", href ? "cursor-pointer hover:bg-foreground/[0.03]" : "hover:bg-transparent")}
      onClick={href ? () => router.push(href) : undefined}
    >
      <TableCell className={cellCls}>
        <span className={cn("font-mono text-[11px] font-bold uppercase tracking-wider whitespace-nowrap", ink.text)}>{ink.label}</span>
      </TableCell>
      <TableCell className={cellCls}>
        <div className="flex flex-col">
          {href ? (
            <Link href={href} className="font-semibold hover:underline" onClick={(e) => e.stopPropagation()}>
              {d.underlying || d.symbol}
            </Link>
          ) : (
            <span className="font-semibold">{d.underlying || d.symbol}</span>
          )}
          {contract && <span className="text-xs whitespace-nowrap text-muted-foreground">{contract}</span>}
        </div>
      </TableCell>
      <TableCell className={cn(cellCls, "w-full max-w-0 max-md:hidden")}>
        {d.rationale && (
          <p className="truncate text-xs text-muted-foreground" title={d.rationale}>
            {d.rationale}
          </p>
        )}
      </TableCell>
      <TableCell className={cn(cellCls, "text-right tabular-nums")}>
        {showNumbers ? (
          <span className="font-mono text-sm whitespace-nowrap">
            {qty} × <AnimatedNumber value={d.limit_price ?? 0} kind="money" />
          </span>
        ) : (
          <span className="text-muted-foreground">-</span>
        )}
      </TableCell>
      <TableCell className={cn(cellCls, "text-right tabular-nums max-sm:hidden")}>
        {showNumbers && (d.notional ?? 0) > 0 ? (
          <span className="text-xs text-muted-foreground">
            <AnimatedNumber value={d.notional ?? 0} kind="money" />
          </span>
        ) : (
          <span className="text-muted-foreground">-</span>
        )}
      </TableCell>
      <TableCell className={cn(cellCls, "text-right")}>
        <StatusMark kind={statusKind(d.status)} />
      </TableCell>
      <TableCell className={cn(cellCls, "w-6 pr-0")}>{href && <ChevronRight className="h-4 w-4 text-muted-foreground" aria-hidden />}</TableCell>
    </TableRow>
  );
}

export function TodaysExecutions({ decisions }: { decisions: PortfolioDecision[] }) {
  if (decisions.length === 0) {
    return <p className="py-6 text-sm text-muted-foreground">No executions today. Either the midday session (~12:30 PM ET) hasn&apos;t run yet, or it looked at the book and chose to leave everything alone.</p>;
  }
  // Money moves lead the tape; holds settle to the bottom as the quiet
  // "and everything else stayed put" coda.
  const moves = decisions.filter((d) => d.action !== "hold");
  const holds = decisions.filter((d) => d.action === "hold");
  return (
    <Table>
      <TableHeader>
        <TableRow className="border-border/50 hover:bg-transparent">
          <TableHead className={headCls}>Action</TableHead>
          <TableHead className={headCls}>Symbol</TableHead>
          <TableHead className={cn(headCls, "max-md:hidden")}>Rationale</TableHead>
          <TableHead className={cn(headCls, "text-right")}>Qty × Limit</TableHead>
          <TableHead className={cn(headCls, "text-right max-sm:hidden")}>Notional</TableHead>
          <TableHead className={cn(headCls, "text-right")}>Status</TableHead>
          <TableHead className={cn(headCls, "w-6")} aria-hidden />
        </TableRow>
      </TableHeader>
      <TableBody>
        {[...moves, ...holds].map((d, i) => (
          <ExecutionRow key={`${d.action}-${d.symbol ?? "?"}-${i}`} d={d} />
        ))}
      </TableBody>
    </Table>
  );
}
