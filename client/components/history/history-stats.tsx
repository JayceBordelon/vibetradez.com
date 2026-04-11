import { Activity, ArrowDown, ArrowUp, Percent, Scale, Sigma, Target, TrendingDown, TrendingUp, Trophy, Wallet } from "lucide-react";

import { StatCard } from "@/components/ui/stat-card";
import { fmt, fmtMoneyInt, fmtPctDec, fmtPnlInt, percentHueColor } from "@/lib/format";

interface HistoryStatsProps {
  totalPnl: number;
  winRate: number;
  roc: number;
  profitFactor: number;
  totalInvested: number;
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
  totalInvested,
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
      {/* Primary stats */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <StatCard index={0} label="Net P&L" value={fmtPnlInt(totalPnl)} sub={`${totalTrades} trades`} tone={signTone(totalPnl)} icon={totalPnl >= 0 ? TrendingUp : TrendingDown} />
        <StatCard index={1} label="Win Rate" value={`${winRate.toFixed(0)}%`} sub={`${totalWinners}W \u00B7 ${totalLosers}L`} valueColor={percentHueColor(winRate)} icon={Target} />
        <StatCard index={2} label="Return on Capital" value={fmtPctDec(roc)} sub="ROC" tone={signTone(roc)} icon={Percent} tooltip="Net P&L / total capital deployed" />
        <StatCard index={3} label="Profit Factor" value={profitFactorValue} tone="neutral" icon={Scale} tooltip="Profit Factor = gross wins \u00F7 gross losses" />
      </div>

      {/* Secondary stats */}
      <div className="mt-3 grid grid-cols-2 gap-3 sm:mt-4 sm:grid-cols-4 xl:grid-cols-8">
        <StatCard index={0} label="Total Deployed" value={fmtMoneyInt(totalInvested)} tone="neutral" icon={Wallet} />
        <StatCard index={1} label="Avg Win" value={`+$${fmt(avgWin, 0)}`} tone="positive" icon={ArrowUp} />
        <StatCard index={2} label="Avg Loss" value={`-$${fmt(avgLoss, 0)}`} tone="negative" icon={ArrowDown} />
        <StatCard index={3} label="Expectancy" value={fmtPnlInt(expectancy)} tone={signTone(expectancy)} icon={Sigma} tooltip="Expected $ per trade = (winRate × avgWin) − (lossRate × avgLoss)" />
        <StatCard index={4} label="Sharpe Ratio" value={fmt(sharpe, 2)} tone="neutral" icon={Activity} tooltip="Annualized risk-adjusted return = mean daily return / std dev × √252" />
        <StatCard index={5} label="Max Drawdown" value={`${fmt(maxDrawdown, 1)}%`} tone="negative" icon={TrendingDown} tooltip="Largest peak-to-trough decline" />
        <StatCard index={6} label="Best Trade" value={fmtPnlInt(bestPnl)} sub={`$${bestSym}`} tone={signTone(bestPnl)} icon={Trophy} />
        <StatCard index={7} label="Worst Trade" value={fmtPnlInt(worstPnl)} sub={`$${worstSym}`} tone={signTone(worstPnl)} icon={TrendingDown} />
      </div>
    </div>
  );
}
