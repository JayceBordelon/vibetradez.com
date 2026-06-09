import { CheckCircle2, XCircle } from "lucide-react";

import { fmtMoney, fmtPrice } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { PortfolioDecision } from "@/types/portfolio";

/*
TodaysExecutions is the day's tape: every move the model committed today,
rendered as a flowing ledger in the site's hairline-divided editorial
style (no cards). Each entry carries a 2px action-colored tick on its
left edge, the same border language the transcript uses, so a green tick
means money in, red means money out, and holds stay quiet and gray.
*/

type StatusKind = "filled" | "working" | "canceled" | "";

function statusKind(status?: string): StatusKind {
  switch ((status ?? "").toUpperCase()) {
    case "FILLED":
      return "filled";
    case "WORKING":
    case "SUBMITTED":
    case "QUEUED":
    case "ACCEPTED":
    case "PENDING_ACTIVATION":
      return "working";
    case "CANCELED":
    case "REJECTED":
    case "EXPIRED":
      return "canceled";
    default:
      return "";
  }
}

const ACTION_INK: Record<string, { label: string; tick: string; text: string }> = {
  buy_equity: { label: "Buy", tick: "bg-green", text: "text-green" },
  buy_option: { label: "Buy to open", tick: "bg-green", text: "text-green" },
  sell_equity: { label: "Sell", tick: "bg-red", text: "text-red" },
  sell_option: { label: "Sell to close", tick: "bg-red", text: "text-red" },
  cancel_order: { label: "Cancel", tick: "bg-amber", text: "text-amber" },
  hold: { label: "Hold", tick: "bg-muted-foreground/40", text: "text-muted-foreground" },
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

function ExecutionRow({ d }: { d: PortfolioDecision }) {
  const ink = ACTION_INK[d.action] ?? { label: d.action, tick: "bg-muted-foreground/40", text: "text-foreground/90" };
  const isHold = d.action === "hold";
  const contract = d.contract_type ? [d.contract_type, d.strike ? `${fmtPrice(d.strike)} strike` : null, d.expiration ? `exp ${d.expiration}` : null].filter(Boolean).join(" · ") : null;
  const qty = d.quantity ?? 0;
  const showNumbers = !isHold && qty > 0 && (d.limit_price ?? 0) > 0;

  return (
    <li className="relative flex items-start justify-between gap-4 py-3.5 pl-4">
      {/* Action tick: the row's left edge carries the move's color. */}
      <span aria-hidden className={cn("absolute top-3 bottom-3 left-0 w-0.5 rounded-full", ink.tick)} />
      <div className="min-w-0">
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
          <span className={cn("font-mono text-[11px] font-bold uppercase tracking-wider", ink.text)}>{ink.label}</span>
          <span className="text-base font-semibold">{d.underlying || d.symbol}</span>
          {contract && <span className="text-xs text-muted-foreground">{contract}</span>}
        </div>
        {d.rationale && <p className="mt-1 max-w-[60ch] text-xs leading-relaxed text-muted-foreground sm:truncate">{d.rationale}</p>}
      </div>
      <div className="flex shrink-0 flex-col items-end gap-1 text-right">
        {showNumbers && (
          <>
            <span className="font-mono text-sm font-medium tabular-nums">
              {qty} × {fmtMoney(d.limit_price ?? 0)}
            </span>
            {(d.notional ?? 0) > 0 && <span className="text-xs text-muted-foreground tabular-nums">{fmtMoney(d.notional ?? 0)} notional</span>}
          </>
        )}
        <StatusMark kind={statusKind(d.status)} />
      </div>
    </li>
  );
}

export function TodaysExecutions({ decisions }: { decisions: PortfolioDecision[] }) {
  if (decisions.length === 0) {
    return <p className="py-6 text-sm text-muted-foreground">No executions today. The model held the book as-is, or the market hasn&apos;t opened yet.</p>;
  }
  // Money moves lead the tape; holds settle to the bottom as the quiet
  // "and everything else stayed put" coda.
  const moves = decisions.filter((d) => d.action !== "hold");
  const holds = decisions.filter((d) => d.action === "hold");
  return (
    <ol className="divide-y divide-border/60">
      {[...moves, ...holds].map((d, i) => (
        <ExecutionRow key={`${d.action}-${d.symbol ?? "?"}-${i}`} d={d} />
      ))}
    </ol>
  );
}
