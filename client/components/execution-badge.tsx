/**
ExecutionBadge surfaces a position Jayce took on a trade. Two
variants:
  - compact: small inline pill for trade cards / table rows
  - full: 4-stat panel for the trade detail page (entry, close,
    realized P&L, mode label)

Color rules:
  - amber background = paper (clearly distinct from real positions)
  - red-orange background = live (real money on the line)
State rules:
  - holding: shows entry + "live position" indicator
  - closed: shows P&L with green/red color
  - failed: shows "open failed" with neutral styling
*/

import { fmtMoney } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Execution } from "@/types/trade";

interface ExecutionBadgeProps {
  execution: Execution;
  variant?: "compact" | "full";
  className?: string;
}

export function ExecutionBadge({ execution, variant = "compact", className }: ExecutionBadgeProps) {
  if (variant === "full") {
    return <ExecutionPanel execution={execution} className={className} />;
  }
  return <ExecutionPill execution={execution} className={className} />;
}

function ExecutionPill({ execution, className }: { execution: Execution; className?: string }) {
  const { mode, state } = execution;
  const isLive = mode === "live";
  const modeColors = isLive ? "border-red-border bg-red-bg text-red" : "border-amber-border bg-amber-bg text-amber";

  let label: React.ReactNode;
  if (state === "failed") {
    label = <>Open failed</>;
  } else if (state === "closed") {
    const pnl = execution.realized_pnl;
    const pnlColor = pnl > 0 ? "text-green" : pnl < 0 ? "text-red" : "text-foreground";
    label = (
      <>
        Closed{" "}
        <span className={cn("font-semibold", pnlColor)}>
          {pnl > 0 ? "+" : ""}
          {fmtMoney(pnl)}
        </span>
      </>
    );
  } else {
    label = (
      <>
        Holding @ <span className="font-semibold">{fmtMoney(execution.open_price)}</span>
      </>
    );
  }

  return (
    <span
      className={cn("inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-[11px] font-medium", modeColors, className)}
      title={isLive ? "Real position taken via Schwab" : "Paper-trade — no real money committed"}
    >
      <span className="inline-block h-1.5 w-1.5 rounded-full bg-current" />
      <span>{label}</span>
      <span className="ml-0.5 text-[10px] font-bold uppercase tracking-wider opacity-80">{mode}</span>
    </span>
  );
}

function ExecutionPanel({ execution, className }: { execution: Execution; className?: string }) {
  const { mode, state, open_price, close_price, realized_pnl, executed_at, closed_at } = execution;
  const isLive = mode === "live";
  const headerColors = isLive ? "border-red-border bg-red-bg text-red" : "border-amber-border bg-amber-bg text-amber";

  return (
    <div className={cn("rounded-lg border", className)}>
      <div className={cn("flex items-center justify-between border-b px-4 py-2 text-xs font-semibold uppercase tracking-wider", headerColors)}>
        <span className="inline-flex items-center gap-2">
          <span className="inline-block h-2 w-2 rounded-full bg-current" />
          Position taken — {mode}
        </span>
        <span>{state}</span>
      </div>
      <div className="grid grid-cols-3 gap-4 p-4">
        <Stat label="Entry" value={open_price > 0 ? fmtMoney(open_price) : "—"} sub={executed_at ? formatTime(executed_at) : null} />
        <Stat label="Close" value={close_price > 0 ? fmtMoney(close_price) : "—"} sub={closed_at ? formatTime(closed_at) : state === "holding" ? "auto-closes 3:55pm ET" : null} />
        <Stat
          label="Realized P&L"
          value={state === "closed" ? `${realized_pnl > 0 ? "+" : ""}${fmtMoney(realized_pnl)}` : "—"}
          valueClassName={state === "closed" ? (realized_pnl > 0 ? "text-green" : realized_pnl < 0 ? "text-red" : "") : ""}
        />
      </div>
      {!isLive && (
        <div className="border-t px-4 py-2 text-[11px] text-muted-foreground">
          Paper-mode position — no real capital is committed. Fill prices use the live Schwab option mark; real-money fills would include slippage and bid/ask spread.
        </div>
      )}
    </div>
  );
}

function Stat({ label, value, sub, valueClassName }: { label: string; value: string; sub?: string | null; valueClassName?: string }) {
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
  return d.toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit", hour12: true, timeZone: "America/New_York" }) + " ET";
}

/**
matchesTrade returns true when the execution is for the given trade
row (same symbol + contract_type + strike). Used by the dashboard /
history surfaces to find which card or row to render the badge on.
*/
export function matchesTrade(execution: Execution | null | undefined, trade: { symbol: string; contract_type: string; strike_price: number }): boolean {
  if (!execution) return false;
  return execution.symbol === trade.symbol && execution.contract_type === trade.contract_type && Math.abs(execution.strike_price - trade.strike_price) < 0.005;
}
