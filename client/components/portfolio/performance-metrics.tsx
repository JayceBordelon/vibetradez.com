"use client";

import { useMemo } from "react";
import { Activity, Clock, Percent, TrendingDown } from "lucide-react";

import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { AnimatedNumber } from "@/components/ui/animated-number";
import { fmtMoneyInt } from "@/lib/format";
import type { ClosedTrade, EquityCurvePoint } from "@/types/portfolio";

interface PerformanceMetricsProps {
  points: EquityCurvePoint[];
  trades: ClosedTrade[];
  /** Same live overlay contract as the chart: applied to the curve's
   *  TODAY point only, so drawdown stays current between polls. */
  today?: string;
  liveEquity?: number | null;
}

const TICK_MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

// "2026-06-03" reads as "Jun 3" in the sub lines. Parsed from the parts so
// it never shifts a day across timezones.
function fmtDay(iso: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!m) return iso;
  return `${TICK_MONTHS[Number(m[2]) - 1] ?? m[2]} ${Number(m[3])}`;
}

/*
PerformanceMetrics is the risk-and-quality strip under the dashboard
chart: the figures the chart's shape implies but never states. Drawdown
comes from the same daily equity series the chart plots (with the live
overlay on today's point); the round-trip stats come from the closed
trades log. Everything renders honestly when young: an account with no
closed trades yet shows quiet placeholders, not fabricated zeros.
*/
export function PerformanceMetrics({ points, trades, today, liveEquity = null }: PerformanceMetricsProps) {
  // Max drawdown: deepest peak-to-trough decline of account equity across
  // the window, as a negative percent, with the trough date for the sub.
  const dd = useMemo(() => {
    let peak = 0;
    let maxDd = 0;
    let troughDate: string | null = null;
    for (const p of points) {
      let v = p.account_equity;
      if (today && p.date === today && liveEquity !== null && liveEquity > 0) v = liveEquity;
      if (v <= 0) continue;
      if (v > peak) peak = v;
      if (peak > 0) {
        const d = ((v - peak) / peak) * 100;
        if (d < maxDd) {
          maxDd = d;
          troughDate = p.date;
        }
      }
    }
    return { maxDd, troughDate };
  }, [points, today, liveEquity]);

  const wins = trades.filter((t) => t.realized_pnl > 0);
  const grossWin = wins.reduce((s, t) => s + t.realized_pnl, 0);
  const grossLoss = trades.reduce((s, t) => s + (t.realized_pnl < 0 ? -t.realized_pnl : 0), 0);
  const winRate = trades.length > 0 ? (wins.length / trades.length) * 100 : null;
  const profitFactor = grossLoss > 0 ? grossWin / grossLoss : null;
  const avgHold = trades.length > 0 ? trades.reduce((s, t) => s + t.hold_days, 0) / trades.length : null;

  return (
    <StatStrip cols={4} className="mt-6">
      <Stat
        label="Max drawdown"
        value={dd.maxDd < 0 ? <AnimatedNumber value={dd.maxDd} kind="pct1" /> : "0.0%"}
        tone={dd.maxDd < 0 ? "negative" : "neutral"}
        icon={TrendingDown}
        sub={dd.troughDate ? `bottomed ${fmtDay(dd.troughDate)}` : "no drawdown yet"}
      />
      <Stat
        label="Win rate"
        value={winRate !== null ? `${winRate.toFixed(0)}%` : "--"}
        icon={Percent}
        sub={trades.length > 0 ? `${wins.length} of ${trades.length} round trips` : "no closed trades yet"}
      />
      <Stat
        label="Profit factor"
        value={profitFactor !== null ? profitFactor.toFixed(2) : "--"}
        tone={profitFactor !== null ? (profitFactor >= 1 ? "positive" : "negative") : "neutral"}
        icon={Activity}
        sub={trades.length === 0 ? "no closed trades yet" : grossLoss > 0 ? `${fmtMoneyInt(grossWin)} won / ${fmtMoneyInt(grossLoss)} lost` : "no losing trades yet"}
      />
      <Stat label="Avg hold" value={avgHold !== null ? `${avgHold.toFixed(1)}d` : "--"} icon={Clock} sub={trades.length > 0 ? "per round trip" : "no closed trades yet"} />
    </StatStrip>
  );
}
