/**
ExecutionBadge surfaces a position Jayce took on a trade. Two
variants:
  - compact: small inline pill for trade cards / table rows
  - full: 4-stat panel for the trade detail page (entry, close,
    realized P&L, mode label)

Color rules:
  - amber background = paper (clearly distinct from real positions)
  - red-orange background = live (real money on the line)
  - green background = live + holding + currently profitable (live mark known)
  - red background = live + holding + currently losing (live mark known)
State rules:
  - submitted: order placed at broker, awaiting fill confirmation
  - holding: shows entry + live unrealized P&L when mark known
  - closed: shows P&L with green/red color
  - failed: shows "open failed" with neutral styling
  - canceled: 9:35 ET dangling-LIMIT cancel, pick never filled, no position
*/

import { Minus } from "lucide-react";

import { fmtMoney } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Execution, LiveQuotesResponse, Trade } from "@/types/trade";

interface ExecutionBadgeProps {
  execution: Execution;
  variant?: "compact" | "full";
  /**
  Current option mark for the contract this execution is on. When
  provided AND the execution is in 'holding' state in live mode, the
  badge derives unrealized P&L = (liveMark - open_price) * 100 and
  recolors itself green/red on that. Optional, surfaces without live
  data (history view) pass null/undefined and the badge falls back to
  the neutral mode-based styling.
  */
  liveMark?: number | null;
  className?: string;
}

export function ExecutionBadge({ execution, variant = "compact", liveMark, className }: ExecutionBadgeProps) {
  if (variant === "full") {
    return <ExecutionPanel execution={execution} liveMark={liveMark} className={className} />;
  }
  return <ExecutionPill execution={execution} liveMark={liveMark} className={className} />;
}

function ExecutionPill({ execution, liveMark, className }: { execution: Execution; liveMark?: number | null; className?: string }) {
  const { mode, state } = execution;
  const isLive = mode === "live";
  const qty = execution.quantity || 1;

  const holdingPnl = computeHoldingPnl(execution, liveMark);

  let modeColors: string;
  if (state === "skipped") {
    // Skipped picks aren't positions, they're decisions. Neutral
    // slate so they don't shout for attention the way real fills or
    // failures do.
    modeColors = "border-border bg-muted/40 text-muted-foreground";
  } else if (holdingPnl !== null && holdingPnl > 0) {
    modeColors = "border-green-border bg-green-bg text-green";
  } else if (holdingPnl !== null && holdingPnl < 0) {
    modeColors = "border-red-border bg-red-bg text-red";
  } else {
    modeColors = isLive ? "border-red-border bg-red-bg text-red" : "border-amber-border bg-amber-bg text-amber";
  }

  /*
  qtyPrefix surfaces multi-contract positions ("Holding 3 @ $1.50",
  "Closed 2× +$120"). Single-contract pills omit the prefix so the
  legacy one-contract baseline reads identically to the pre-rewrite UI.
  */
  const qtyPrefix = qty > 1 ? `${qty} ` : "";

  let label: React.ReactNode;
  if (state === "skipped") {
    // Skipped at the open by the agent. The reason text rides on
    // the badge's title attribute so the full justification is one
    // hover away without bloating the inline pill.
    label = <>Skipped at open</>;
  } else if (state === "failed") {
    label = <>{qty > 1 ? `Open failed (${qty} contracts)` : "Open failed"}</>;
  } else if (state === "canceled") {
    label = <>Open canceled (stale quote)</>;
  } else if (state === "close_failed") {
    // Position is filled at the broker but auto-close attempt
    // terminated in a non-filled state. Operator needs to verify and
    // close manually. Surface this loudly rather than masking as "Holding".
    label = <>Close failed (verify position)</>;
  } else if (state === "submitted") {
    label = <>{qty > 1 ? `${qty} contracts working at broker` : "Order working at broker"}</>;
  } else if (state === "closed") {
    const pnl = execution.realized_pnl;
    const pnlColor = pnl > 0 ? "text-green" : pnl < 0 ? "text-red" : "text-foreground";
    label = (
      <>
        Closed{qty > 1 ? ` ${qty}×` : ""}{" "}
        <span className={cn("font-semibold", pnlColor)}>
          {pnl > 0 ? "+" : ""}
          {fmtMoney(pnl)}
        </span>
      </>
    );
  } else if (holdingPnl !== null) {
    label = (
      <>
        Holding {qtyPrefix}@ <span className="font-semibold">{fmtMoney(execution.open_price)}</span>
        <span className="ml-1 font-semibold">
          ({holdingPnl > 0 ? "+" : ""}
          {fmtMoney(holdingPnl)})
        </span>
      </>
    );
  } else {
    label = (
      <>
        Holding {qtyPrefix}@ <span className="font-semibold">{fmtMoney(execution.open_price)}</span>
      </>
    );
  }

  const title =
    state === "skipped" ? execution.note || "Skipped at open by the at-open agent (no reason provided)" : isLive ? "Real position taken via Schwab" : "Paper-trade, no real money committed";

  return (
    <span className={cn("inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-[11px] font-medium", modeColors, className)} title={title}>
      <span className="inline-block h-1.5 w-1.5 rounded-full bg-current" />
      <span>{label}</span>
      {state !== "skipped" && <span className="ml-0.5 text-[10px] font-bold uppercase tracking-wider opacity-80">{mode}</span>}
    </span>
  );
}

function ExecutionPanel({ execution, liveMark, className }: { execution: Execution; liveMark?: number | null; className?: string }) {
  const { mode, state, open_price, close_price, realized_pnl, executed_at, closed_at } = execution;
  const isLive = mode === "live";

  // Skipped picks get their own minimal panel since none of the
  // Entry / Close / P&L columns mean anything for a position that
  // was never taken. The reason text is the entire payload.
  if (state === "skipped") {
    return (
      <div className={cn("rounded-lg border border-border bg-muted/30", className)}>
        <div className="border-b border-border px-4 py-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">Skipped at open by the at-open agent</div>
        <div className="px-4 py-3 text-sm leading-relaxed text-foreground/85">{execution.note?.trim() ? execution.note : "No reason provided."}</div>
      </div>
    );
  }

  const holdingPnl = computeHoldingPnl(execution, liveMark);

  let headerColors: string;
  if (state === "close_failed") {
    headerColors = "border-red-border bg-red-bg text-red";
  } else if (holdingPnl !== null && holdingPnl > 0) {
    headerColors = "border-green-border bg-green-bg text-green";
  } else if (holdingPnl !== null && holdingPnl < 0) {
    headerColors = "border-red-border bg-red-bg text-red";
  } else {
    headerColors = isLive ? "border-red-border bg-red-bg text-red" : "border-amber-border bg-amber-bg text-amber";
  }

  // close_failed surfaces loudly in the header: the auto-close cron
  // exhausted its retry window and the position is still open at the
  // broker. Without this branch the panel reads "Position taken" with
  // no warning, operator wouldn't know to check Schwab.
  let headerLabel: string;
  switch (state) {
    case "submitted":
      headerLabel = "Order placed";
      break;
    case "close_failed":
      headerLabel = "Close failed, verify at broker";
      break;
    case "canceled":
      headerLabel = "Open canceled";
      break;
    case "failed":
      headerLabel = "Open failed";
      break;
    default:
      headerLabel = "Position taken";
  }
  const closeSub = closed_at
    ? formatTime(closed_at)
    : state === "holding"
      ? "auto-closes 3:55pm ET"
      : state === "submitted"
        ? "pending open fill"
        : state === "close_failed"
          ? "auto-close exhausted retries"
          : state === "canceled"
            ? "limit went stale 5min post-open"
            : null;
  const entrySub = executed_at ? formatTime(executed_at) : state === "submitted" ? "awaiting broker fill" : null;

  /*
  Third stat slot is dual-purpose: realized P&L when closed, live
  unrealized P&L when holding-with-mark, NoValue placeholder
  otherwise. Using the same column instead of a separate 4th stat
  keeps the panel layout identical across states.
  */
  let pnlLabel = "Realized P&L";
  let pnlValue: React.ReactNode = <NoValue />;
  let pnlValueClass = "";
  if (state === "closed") {
    pnlValue = `${realized_pnl > 0 ? "+" : ""}${fmtMoney(realized_pnl)}`;
    pnlValueClass = realized_pnl > 0 ? "text-green" : realized_pnl < 0 ? "text-red" : "";
  } else if (holdingPnl !== null) {
    pnlLabel = "Unrealized P&L";
    pnlValue = `${holdingPnl > 0 ? "+" : ""}${fmtMoney(holdingPnl)}`;
    pnlValueClass = holdingPnl > 0 ? "text-green" : holdingPnl < 0 ? "text-red" : "";
  }

  /*
  Close column shows live mark on holding-with-mark so the user can
  see the contract is moving. Otherwise renders the NoValue
  placeholder until the 3:55pm cron lands the real close.
  */
  let closeValue: React.ReactNode = <NoValue />;
  if (close_price > 0) {
    closeValue = fmtMoney(close_price);
  } else if (state === "holding" && liveMark != null && liveMark > 0) {
    closeValue = fmtMoney(liveMark);
  }
  const closeSubResolved = state === "holding" && liveMark != null && liveMark > 0 && !closed_at ? "live mark" : closeSub;

  const qty = execution.quantity || 1;

  return (
    <div className={cn("rounded-lg border", className)}>
      <div className={cn("flex items-center justify-between border-b px-4 py-2 text-xs font-semibold uppercase tracking-wider", headerColors)}>
        <span className="inline-flex items-center gap-2">
          <span className="inline-block h-2 w-2 rounded-full bg-current" />
          {headerLabel} · {mode}
          {qty > 1 && <span className="ml-1 rounded-md border border-current/40 px-1.5 py-0.5 text-[10px] font-bold tabular-nums normal-case tracking-normal">{qty}× contracts</span>}
        </span>
        <span>{state}</span>
      </div>
      <div className="grid grid-cols-3 gap-4 p-4">
        <Stat label="Entry" value={open_price > 0 ? fmtMoney(open_price) : <NoValue />} sub={entrySub} />
        <Stat label="Close" value={closeValue} sub={closeSubResolved} />
        <Stat label={pnlLabel} value={pnlValue} valueClassName={pnlValueClass} />
      </div>
      {!isLive && (
        <div className="border-t px-4 py-2 text-[11px] text-muted-foreground">
          Paper-mode position: no real capital is committed. Fill prices use the live Schwab option mark. Real-money fills would include slippage and bid/ask spread.
        </div>
      )}
    </div>
  );
}

/**
NoValue renders a small low-opacity minus icon as the placeholder for
"not applicable yet" execution states (submitted but unfilled, holding
without a live mark, etc.). Distinct from the dashboard's Skeleton
pulse, which means "live data is loading and will arrive shortly".
*/
function NoValue() {
  return <Minus className="inline-block h-4 w-4 align-middle text-muted-foreground/40" aria-hidden />;
}

function Stat({ label, value, sub, valueClassName }: { label: string; value: React.ReactNode; sub?: string | null; valueClassName?: string }) {
  return (
    <div>
      <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</div>
      <div className={cn("mt-0.5 text-lg font-semibold tabular-nums", valueClassName)}>{value}</div>
      {sub && <div className="mt-0.5 text-[10px] text-muted-foreground">{sub}</div>}
    </div>
  );
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return (
    d.toLocaleTimeString("en-US", {
      hour: "numeric",
      minute: "2-digit",
      hour12: true,
      timeZone: "America/New_York",
    }) + " ET"
  );
}

/**
matchesTrade returns true when the execution is for the given trade
row (same symbol + contract_type + strike). Used by the dashboard /
history surfaces to find which card or row to render the badge on.
*/
export function matchesTrade(execution: Execution | null | undefined, trade: { symbol: string; contract_type: string; strike_price: number | null }): boolean {
  if (!execution) return false;
  if (execution.symbol !== trade.symbol || execution.contract_type !== trade.contract_type) return false;
  /*
  Skipped picks never resolved a strike: trade.strike_price stays
  NULL forever and the execution row carries strike=0 (the SQL
  COALESCE default for the same NULL). Match by symbol + contract
  type alone in that case — the top-3 selector guarantees
  uniqueness per (symbol, contract_type) within a day.
  */
  if (trade.strike_price === null) return execution.state === "skipped";
  return Math.abs(execution.strike_price - trade.strike_price) < 0.005;
}

/**
findExecutionForTrade scans a list of executions and returns the one
matching the given trade by symbol + contract_type + strike. Returns
null when no execution matches, caller should fall back to the
modeled summary numbers in that case. Returns null for pre-resolution
picks (strike_price still nil) since the executor only creates rows
after resolution.
*/
export function findExecutionForTrade(executions: Execution[] | null | undefined, trade: { symbol: string; contract_type: string; strike_price: number | null }): Execution | null {
  if (!executions || executions.length === 0) return null;
  for (const e of executions) {
    if (matchesTrade(e, trade)) return e;
  }
  return null;
}

/**
findClosedExecutionForTrade returns the execution for the trade ONLY
when it has a closed state with both fill prices populated. This is
the gate for sourcing realized P&L from broker truth instead of from
the modeled EOD summary, pre-bugfix close rows recorded fill_price=0,
so a closed execution with close_price=0 must fall back to summary.
*/
export function findClosedExecutionForTrade(executions: Execution[] | null | undefined, trade: { symbol: string; contract_type: string; strike_price: number | null }): Execution | null {
  const e = findExecutionForTrade(executions, trade);
  if (!e) return null;
  if (e.state !== "closed") return null;
  if (e.open_price <= 0 || e.close_price <= 0) return null;
  return e;
}

/**
liveMarkForTrade looks up the current option mark for a trade from
the dashboard live-quotes payload. The server keys the options map
as "<SYMBOL>|<CALL|PUT>|<strike to 2dp>|<expiration>" (see
server.go:846 / morning-layout:32), so we mirror that key here.
Returns null when the live feed isn't connected, the contract dropped
off the chain, or the mark is non-positive.
*/
/**
executionQuantity returns the contract count for an execution row,
defaulting to 1 when the field is missing or zero. Centralizes the
"legacy single-contract rows have quantity unset" defensive fallback
so callers (exposure panel, P&L math) don't each reimplement it.
*/
export function executionQuantity(execution: Execution | null | undefined): number {
  if (!execution || !execution.quantity || execution.quantity < 1) return 1;
  return execution.quantity;
}

export function liveMarkForTrade(liveQuotes: LiveQuotesResponse | null | undefined, trade: Pick<Trade, "symbol" | "contract_type" | "strike_price" | "expiration">): number | null {
  if (!liveQuotes?.options) return null;
  if (trade.strike_price === null || trade.expiration === null) return null;
  const key = `${trade.symbol}|${trade.contract_type}|${trade.strike_price.toFixed(2)}|${trade.expiration}`;
  const mark = liveQuotes.options[key]?.mark;
  if (mark == null || mark <= 0) return null;
  return mark;
}

/**
computeHoldingPnl returns the unrealized P&L (in dollars per single
contract) for a live execution currently in 'holding' state, given
the contract's live mark. Returns null when:
  - the execution isn't a live holding (paper mode, or state ≠ holding)
  - no live mark is available (Schwab disconnected, contract delisted)
  - the open fill price wasn't recorded (defensive, shouldn't happen)
Never returns 0 by accident: we explicitly distinguish "no mark known"
(null → neutral styling) from "mark known and break-even" (0 →
neutral styling but caller can still display "+$0.00").
*/
function computeHoldingPnl(execution: Execution, liveMark: number | null | undefined): number | null {
  if (execution.state !== "holding") return null;
  if (execution.mode !== "live") return null;
  if (liveMark == null || liveMark <= 0) return null;
  if (execution.open_price <= 0) return null;
  const qty = execution.quantity || 1;
  return (liveMark - execution.open_price) * 100 * qty;
}
