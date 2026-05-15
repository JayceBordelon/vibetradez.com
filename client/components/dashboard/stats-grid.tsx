"use client";

import { Percent, Target, TrendingDown, TrendingUp, Trophy } from "lucide-react";
import { motion, type Variants } from "motion/react";

import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { useCountUp } from "@/hooks/use-count-up";
import { fmtPnlInt, percentHueColor } from "@/lib/format";

interface StatsGridProps {
  totalPnl: number;
  winRate: number;
  profitFactor: number;
  bestPnl: number;
  bestSym: string;
}

const containerVariants: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.06 } },
};

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 8 },
  show: { opacity: 1, y: 0, transition: { duration: 0.32, ease: [0.22, 1, 0.36, 1] } },
};

export function StatsGrid({ totalPnl, winRate, profitFactor, bestPnl, bestSym }: StatsGridProps) {
  const animatedPnl = useCountUp(totalPnl);

  const pnlTone: "positive" | "negative" | "neutral" = totalPnl > 0 ? "positive" : totalPnl < 0 ? "negative" : "neutral";
  const pnlIcon = totalPnl >= 0 ? TrendingUp : TrendingDown;

  const isInfinite = profitFactor === Number.POSITIVE_INFINITY;
  const profitFactorValue = isInfinite ? "∞" : `${profitFactor.toFixed(2)}x`;
  const profitFactorSub = isInfinite ? "no losses yet" : "gross wins / gross losses";

  return (
    <motion.div variants={containerVariants} initial="hidden" animate="show">
      <StatStrip cols={4}>
        <motion.div variants={itemVariants}>
          <Stat label="Net P&L" value={fmtPnlInt(animatedPnl)} tone={pnlTone} icon={pnlIcon} />
        </motion.div>
        <motion.div variants={itemVariants}>
          <Stat label="Win Rate" value={`${winRate.toFixed(0)}%`} sub="winners / closed" valueColor={percentHueColor(winRate)} icon={Target} />
        </motion.div>
        <motion.div variants={itemVariants}>
          <Stat label="Profit Factor" value={profitFactorValue} sub={profitFactorSub} icon={Percent} />
        </motion.div>
        <motion.div variants={itemVariants}>
          <Stat label="Best Trade" value={fmtPnlInt(bestPnl)} sub={bestSym ? `$${bestSym}` : undefined} tone={bestPnl > 0 ? "positive" : bestPnl < 0 ? "negative" : "neutral"} icon={Trophy} />
        </motion.div>
      </StatStrip>
    </motion.div>
  );
}
