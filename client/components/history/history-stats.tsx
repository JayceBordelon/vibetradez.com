import { Activity, ArrowDownRight, ArrowUpRight, Percent, Scale, Sigma, Target, TrendingDown, TrendingUp } from "lucide-react";

import { Card, CardContent } from "@/components/ui/card";
import { StatCard } from "@/components/ui/stat-card";
import { fmt, fmtPctDec, fmtPnlInt, percentHueColor, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";

interface HistoryStatsProps {
  totalPnl: number;
  winRate: number;
  roc: number;
  profitFactor: number;
  avgWin: number;
  avgLoss: number;
  expectancy: number;
  sharpe: number;
  maxDrawdown: number;
  bestPnl: number;
  bestSym: string;
  worstPnl: number;
  worstSym: string;
  totalWinners: number;
  totalLosers: number;
  totalTrades: number;
}

function signTone(v: number): "positive" | "negative" | "neutral" {
  if (v > 0) return "positive";
  if (v < 0) return "negative";
  return "neutral";
}

export function HistoryStats({
  totalPnl,
  winRate,
  roc,
  profitFactor,
  avgWin,
  avgLoss,
  expectancy,
  sharpe,
  maxDrawdown,
  bestPnl,
  bestSym,
  worstPnl,
  worstSym,
  totalWinners,
  totalLosers,
  totalTrades,
}: HistoryStatsProps) {
  const profitFactorValue = profitFactor === Number.POSITIVE_INFINITY ? "\u221E" : `${fmt(profitFactor, 2)}x`;

  return (
    <div>
      <div className="mb-3 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">Performance Snapshot</div>

      {/* Primary stats */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <StatCard index={0} label="Net P&L" value={fmtPnlInt(totalPnl)} sub={`${totalTrades} trades`} tone={signTone(totalPnl)} icon={totalPnl >= 0 ? TrendingUp : TrendingDown} />
        <StatCard index={1} label="Win Rate" value={`${winRate.toFixed(0)}%`} sub={`${totalWinners}W \u00B7 ${totalLosers}L`} valueColor={percentHueColor(winRate)} icon={Target} />
        <StatCard index={2} label="Return on Capital" value={fmtPctDec(roc)} sub="ROC" tone={signTone(roc)} icon={Percent} tooltip="Net P&L / total capital deployed" />
        <StatCard index={3} label="Profit Factor" value={profitFactorValue} tone="neutral" icon={Scale} tooltip="Profit Factor = gross wins / gross losses" />
      </div>

      {/* Secondary stats */}
      <div className="mt-3 grid grid-cols-1 gap-3 sm:mt-4 sm:grid-cols-2 xl:grid-cols-4">
        <DualMetricCard label="Avg Win / Loss" left={{ value: `+$${fmt(avgWin, 0)}`, hint: "Win", positive: true }} right={{ value: `-$${fmt(avgLoss, 0)}`, hint: "Loss", positive: false }} />
        <StatCard
          label="Expectancy"
          value={fmtPnlInt(expectancy)}
          sub="Per trade"
          tone={signTone(expectancy)}
          icon={Sigma}
          tooltip="Expected $ per trade = (winRate × avgWin) − (lossRate × avgLoss)"
        />
        <StatCard
          label="Sharpe / Drawdown"
          value={fmt(sharpe, 2)}
          sub={`Max DD ${fmt(maxDrawdown, 1)}%`}
          tone="neutral"
          icon={Activity}
          tooltip="Sharpe: annualized risk-adjusted return. Max DD: largest peak-to-trough decline"
        />
        <DualMetricCard
          label="Best / Worst Trade"
          left={{ value: fmtPnlInt(bestPnl), hint: bestSym ? `$${bestSym}` : "·", positive: bestPnl >= 0 }}
          right={{ value: fmtPnlInt(worstPnl), hint: worstSym ? `$${worstSym}` : "·", positive: worstPnl >= 0 }}
        />
      </div>
    </div>
  );
}

interface DualMetricCardProps {
  label: string;
  left: { value: string; hint: string; positive: boolean };
  right: { value: string; hint: string; positive: boolean };
}

function DualMetricCard({ label, left, right }: DualMetricCardProps) {
  return (
    <Card className="group gap-0 py-0 transition-all duration-150 hover:-translate-y-0.5 hover:shadow-md">
      <CardContent className="p-5">
        <div className="flex items-center gap-2">
          <span className="h-1.5 w-1.5 rounded-full bg-primary" aria-hidden />
          <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</span>
        </div>
        <div className="mt-2 grid grid-cols-2 gap-3">
          <DualSide value={left.value} hint={left.hint} positive={left.positive} icon={ArrowUpRight} />
          <DualSide value={right.value} hint={right.hint} positive={right.positive} icon={ArrowDownRight} />
        </div>
      </CardContent>
    </Card>
  );
}

function DualSide({ value, hint, positive, icon: Icon }: { value: string; hint: string; positive: boolean; icon: typeof ArrowUpRight }) {
  const pnlValue = Number(value.replace(/[^\d.-]/g, ""));
  return (
    <div>
      <div className={cn("flex items-center gap-1 text-[19px] font-semibold tabular-nums leading-tight sm:text-[22px]", pnlColor(positive ? Math.abs(pnlValue) || 1 : -Math.abs(pnlValue) - 1))}>
        <Icon className="h-3.5 w-3.5 shrink-0 opacity-60" />
        {value}
      </div>
      <div className="mt-0.5 text-[11px] text-muted-foreground">{hint}</div>
    </div>
  );
}
