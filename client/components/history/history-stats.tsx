import { Activity, Percent, Scale, Sigma, Target, TrendingDown, TrendingUp } from "lucide-react";
import { motion, type Variants } from "motion/react";

import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { fmt, fmtPctDec, fmtPnlInt, percentHueColor } from "@/lib/format";
import { cn } from "@/lib/utils";

const containerVariants: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.04 } },
};

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 8 },
  show: { opacity: 1, y: 0, transition: { duration: 0.32, ease: [0.22, 1, 0.36, 1] } },
};

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
  const profitFactorValue = profitFactor === Number.POSITIVE_INFINITY ? "∞" : `${fmt(profitFactor, 2)}x`;

  return (
    <div>
      <div className="mb-3 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">Performance Snapshot</div>

      <motion.div variants={containerVariants} initial="hidden" animate="show">
        <StatStrip cols={4}>
          <motion.div variants={itemVariants}>
            <Stat label="Net P&L" value={fmtPnlInt(totalPnl)} sub={`${totalTrades} trades`} tone={signTone(totalPnl)} icon={totalPnl >= 0 ? TrendingUp : TrendingDown} />
          </motion.div>
          <motion.div variants={itemVariants}>
            <Stat label="Win Rate" value={`${winRate.toFixed(0)}%`} sub={`${totalWinners}W · ${totalLosers}L`} valueColor={percentHueColor(winRate)} icon={Target} />
          </motion.div>
          <motion.div variants={itemVariants}>
            <Stat label="Return on Capital" value={fmtPctDec(roc)} sub="Net P&L / capital deployed" tone={signTone(roc)} icon={Percent} />
          </motion.div>
          <motion.div variants={itemVariants}>
            <Stat label="Profit Factor" value={profitFactorValue} sub="gross wins / gross losses" icon={Scale} />
          </motion.div>
        </StatStrip>

        <StatStrip cols={4} className="mt-3 sm:mt-4">
          <motion.div variants={itemVariants}>
            <Stat label="Avg Win" value={`+$${fmt(avgWin, 0)}`} tone="positive" />
          </motion.div>
          <motion.div variants={itemVariants}>
            <Stat label="Avg Loss" value={`-$${fmt(avgLoss, 0)}`} tone="negative" />
          </motion.div>
          <motion.div variants={itemVariants}>
            <Stat label="Expectancy" value={fmtPnlInt(expectancy)} sub="per trade" tone={signTone(expectancy)} icon={Sigma} />
          </motion.div>
          <motion.div variants={itemVariants}>
            <Stat label="Sharpe" value={fmt(sharpe, 2)} sub={`Max DD ${fmt(maxDrawdown, 1)}%`} icon={Activity} />
          </motion.div>
        </StatStrip>
      </motion.div>

      <div className="mt-4 flex flex-wrap items-baseline gap-x-6 gap-y-1 text-[12px] text-muted-foreground">
        <span>
          <span className="text-[10px] font-semibold uppercase tracking-wider">Best</span>{" "}
          <span className={cn("font-semibold tabular-nums", bestPnl >= 0 ? "text-green" : "text-foreground")}>{fmtPnlInt(bestPnl)}</span>
          {bestSym && <span className="ml-1 font-mono">${bestSym}</span>}
        </span>
        <span>
          <span className="text-[10px] font-semibold uppercase tracking-wider">Worst</span>{" "}
          <span className={cn("font-semibold tabular-nums", worstPnl < 0 ? "text-red" : "text-foreground")}>{fmtPnlInt(worstPnl)}</span>
          {worstSym && <span className="ml-1 font-mono">${worstSym}</span>}
        </span>
      </div>
    </div>
  );
}
