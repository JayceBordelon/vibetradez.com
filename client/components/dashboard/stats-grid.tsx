"use client";

import { Percent, Target, TrendingDown, TrendingUp, Trophy } from "lucide-react";

import { StatCard } from "@/components/ui/stat-card";
import { useCountUp } from "@/hooks/use-count-up";
import { fmtPnlInt, percentHueColor } from "@/lib/format";

interface StatsGridProps {
	totalPnl: number;
	winRate: number;
	profitFactor: number;
	bestPnl: number;
	bestSym: string;
}

export function StatsGrid({
	totalPnl,
	winRate,
	profitFactor,
	bestPnl,
	bestSym,
}: StatsGridProps) {
	const animatedPnl = useCountUp(totalPnl);

	const pnlTone: "positive" | "negative" | "neutral" =
		totalPnl > 0 ? "positive" : totalPnl < 0 ? "negative" : "neutral";

	const pnlIcon = totalPnl >= 0 ? TrendingUp : TrendingDown;

	const isInfinite = profitFactor === Number.POSITIVE_INFINITY;
	const profitFactorValue = isInfinite ? "\u221E" : `${profitFactor.toFixed(2)}x`;
	const profitFactorSub = isInfinite ? "(no losses yet)" : undefined;

	return (
		<div className="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
			<StatCard
				label="Net P&L"
				value={fmtPnlInt(animatedPnl)}
				tone={pnlTone}
				icon={pnlIcon}
				index={0}
			/>
			<StatCard
				label="Win Rate"
				value={`${winRate.toFixed(0)}%`}
				valueColor={percentHueColor(winRate)}
				icon={Target}
				tooltip="Winning trades / total closed trades"
				index={1}
			/>
			<StatCard
				label="Profit Factor"
				value={profitFactorValue}
				sub={profitFactorSub}
				tone="neutral"
				icon={Percent}
				tooltip="Profit Factor = gross wins / gross losses"
				index={2}
			/>
			<StatCard
				label="Best Trade"
				value={fmtPnlInt(bestPnl)}
				sub={`$${bestSym}`}
				tone={bestPnl > 0 ? "positive" : bestPnl < 0 ? "negative" : "neutral"}
				icon={Trophy}
				index={3}
			/>
		</div>
	);
}
