import { findClosedExecutionForTrade, findExecutionForTrade } from "@/components/execution-badge";
import type { DashboardTrade, Execution, Trade } from "@/types/trade";

export function calcMoneyness(trade: Trade) {
  const s = trade.current_price;
  const k = trade.strike_price;
  if (s === 0 || k === 0) return { pct: 0, label: "ATM", variant: "outline" as const };

  const pct = trade.contract_type === "CALL" ? ((s - k) / k) * 100 : ((k - s) / k) * 100;

  if (Math.abs(pct) < 1) return { pct, label: "ATM", variant: "outline" as const };
  if (pct > 0)
    return {
      pct,
      label: `${Math.abs(pct).toFixed(1)}% ITM`,
      variant: "default" as const,
    };
  return {
    pct,
    label: `${Math.abs(pct).toFixed(1)}% OTM`,
    variant: "destructive" as const,
  };
}

export function calcBreakeven(trade: Trade): number {
  return trade.contract_type === "CALL" ? trade.strike_price + trade.estimated_price : trade.strike_price - trade.estimated_price;
}

export function calcMaxLoss(trade: Trade): number {
  return trade.estimated_price * 100;
}

export function sentimentLabel(score: number): string {
  if (score > 0.3) return "Bullish";
  if (score < -0.3) return "Bearish";
  return "Neutral";
}

export function sentimentColor(score: number): string {
  if (score > 0.3) return "text-green";
  if (score < -0.3) return "text-red";
  return "text-muted-foreground";
}

/**
TradePnl bundles the entry/close prices, $ P&L, % change, and the
source flag the caller can use to badge "actual fill" vs "modeled".
hasData is true when a P&L is computable at all (either a settled
summary or a closed live execution exists).
*/
export interface TradePnl {
  entryPrice: number;
  closingPrice: number;
  pnl: number;
  pctChange: number;
  source: "execution" | "modeled";
  hasData: boolean;
}

/**
computeTradePnl resolves which P&L number a trade card / row should
display. Prefers a matching closed live execution (broker truth)
over the modeled EOD summary (Claude's morning estimate + EOD re-
quote). The summary uses Claude's modeled prices, which can drift
from the real broker fill by 10-20% on liquid options — once an
execution exists, we trust the broker number.

Returns hasData=false when neither source is available; callers
render "—" or skip the row.
*/
export function computeTradePnl(dt: DashboardTrade, executions: Execution[] | null | undefined): TradePnl {
  const live = findClosedExecutionForTrade(executions, dt.trade);
  if (live) {
    const pctChange = live.open_price > 0 ? ((live.close_price - live.open_price) / live.open_price) * 100 : 0;
    return {
      entryPrice: live.open_price,
      closingPrice: live.close_price,
      pnl: live.realized_pnl,
      pctChange,
      source: "execution",
      hasData: true,
    };
  }
  /*
  Audit re-audit lens 4 #2: a close_failed execution means the
  position is STILL OPEN at the broker (the auto-close retry window
  exhausted). The modeled summary path below would silently roll up
  Claude's modeled close price into stats, masking the operational
  problem. Return hasData=false instead so StatsGrid / PnlChart /
  ExposurePanel exclude these from aggregates — the trade table and
  badges separately surface the "verify position" warning.
  */
  const anyExec = findExecutionForTrade(executions, dt.trade);
  if (anyExec?.state === "close_failed") {
    return { entryPrice: anyExec.open_price, closingPrice: 0, pnl: 0, pctChange: 0, source: "execution", hasData: false };
  }
  if (dt.summary) {
    const pnl = (dt.summary.closing_price - dt.summary.entry_price) * 100;
    const pctChange = dt.summary.entry_price > 0 ? ((dt.summary.closing_price - dt.summary.entry_price) / dt.summary.entry_price) * 100 : 0;
    return {
      entryPrice: dt.summary.entry_price,
      closingPrice: dt.summary.closing_price,
      pnl,
      pctChange,
      source: "modeled",
      hasData: true,
    };
  }
  return { entryPrice: 0, closingPrice: 0, pnl: 0, pctChange: 0, source: "modeled", hasData: false };
}
