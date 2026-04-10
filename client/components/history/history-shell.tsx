"use client";

import { BarChart3 } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { TopNFilter } from "@/components/dashboard/top-n-filter";
import { HistorySkeleton } from "@/components/layout/dashboard-skeleton";
import { DataFreshness } from "@/components/layout/data-freshness";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { Section } from "@/components/layout/section";
import { Separator } from "@/components/ui/separator";
import { api } from "@/lib/api";
import {
	getRangeBounds,
	getRangeLabel,
	maxRangeOffset,
} from "@/lib/date-utils";
import type { WeekResponse } from "@/types/trade";

import { CapitalEfficiency } from "./capital-efficiency";
import { DailyBreakdown } from "./daily-breakdown";
import { DailyPnlChart } from "./daily-pnl-chart";
import { DateRangeNav } from "./date-range-nav";
import { EquityCurveChart } from "./equity-curve-chart";
import { ExposureReturnsChart } from "./exposure-returns-chart";
import { HistoryStats } from "./history-stats";
import { MethodologySection } from "./methodology-section";
import { ModeToggle } from "./mode-toggle";

const STORAGE_KEY = "jt_hist_v2";

type DayStat = {
	date: string;
	pnl: number;
	winners: number;
	losers: number;
	trades: number;
	hasSummaries: boolean;
	invested: number;
	returned: number;
	details: {
		symbol: string;
		type: string;
		strike: number;
		entry: number;
		close: number;
		pnl: number;
		pct: number;
		result: string;
	}[];
};

function filterByRank(data: WeekResponse, topFilter: number): WeekResponse {
	// Server should always return an empty array, but be defensive in case
	// an older deployment or a transient error path emits null days.
	const days = data.days ?? [];
	if (topFilter >= 10) return { ...data, days };
	return {
		...data,
		days: days
			.map((day) => ({
				...day,
				trades: (day.trades ?? []).filter(
					(t) => t.trade.rank >= 1 && t.trade.rank <= topFilter,
				),
			}))
			.filter((day) => day.trades.length > 0),
	};
}

function computeAggregates(data: WeekResponse) {
	let totalPnl = 0;
	let totalWinners = 0;
	let totalLosers = 0;
	let totalTrades = 0;
	let totalInvested = 0;
	let totalReturn = 0;
	let grossWins = 0;
	let grossLosses = 0;
	let bestPnl = -Infinity;
	let worstPnl = Infinity;
	let bestSym = "";
	let worstSym = "";
	let peakEquity = 0;
	let maxDrawdown = 0;
	let cumPnl = 0;
	const dailyReturns: number[] = [];
	const equityPoints: { date: string; cumPnl: number }[] = [];
	const dayStats: DayStat[] = [];

	for (const day of data.days ?? []) {
		let dayPnl = 0;
		let dayW = 0;
		let dayL = 0;
		let dayHasSummaries = false;
		let dayInvested = 0;
		let dayReturned = 0;
		const details: DayStat["details"] = [];

		for (const { trade, summary } of day.trades ?? []) {
			totalTrades++;
			if (summary) {
				dayHasSummaries = true;
				const pnl = (summary.closing_price - summary.entry_price) * 100;
				const pct =
					summary.entry_price > 0
						? ((summary.closing_price - summary.entry_price) /
								summary.entry_price) *
							100
						: 0;
				dayPnl += pnl;
				totalPnl += pnl;
				dayInvested += summary.entry_price * 100;
				dayReturned += summary.closing_price * 100;
				totalInvested += summary.entry_price * 100;
				totalReturn += summary.closing_price * 100;

				if (pnl > 0.5) {
					dayW++;
					totalWinners++;
					grossWins += pnl;
				} else if (pnl < -0.5) {
					dayL++;
					totalLosers++;
					grossLosses += Math.abs(pnl);
				}
				if (pnl > bestPnl) {
					bestPnl = pnl;
					bestSym = trade.symbol;
				}
				if (pnl < worstPnl) {
					worstPnl = pnl;
					worstSym = trade.symbol;
				}

				details.push({
					symbol: trade.symbol,
					type: trade.contract_type,
					strike: trade.strike_price,
					entry: summary.entry_price,
					close: summary.closing_price,
					pnl,
					pct,
					result: pnl > 0.5 ? "profit" : pnl < -0.5 ? "loss" : "flat",
				});
			}
		}

		cumPnl += dayPnl;
		if (cumPnl > peakEquity) peakEquity = cumPnl;
		if (peakEquity > 0) {
			const dd = ((peakEquity - cumPnl) / peakEquity) * 100;
			if (dd > maxDrawdown) maxDrawdown = dd;
		}
		if (dayHasSummaries && dayInvested > 0)
			dailyReturns.push(dayPnl / dayInvested);

		equityPoints.push({ date: day.date, cumPnl });
		dayStats.push({
			date: day.date,
			pnl: dayPnl,
			winners: dayW,
			losers: dayL,
			trades: day.trades.length,
			hasSummaries: dayHasSummaries,
			invested: dayInvested,
			returned: dayReturned,
			details,
		});
	}

	const winRate =
		totalWinners + totalLosers > 0
			? (totalWinners / (totalWinners + totalLosers)) * 100
			: 0;
	const profitFactor =
		grossLosses > 0
			? grossWins / grossLosses
			: grossWins > 0
				? Number.POSITIVE_INFINITY
				: 0;
	const avgWin = totalWinners > 0 ? grossWins / totalWinners : 0;
	const avgLoss = totalLosers > 0 ? grossLosses / totalLosers : 0;
	const expectancy =
		(winRate / 100) * avgWin - (1 - winRate / 100) * avgLoss;
	const roc = totalInvested > 0 ? (totalPnl / totalInvested) * 100 : 0;

	let sharpe = 0;
	if (dailyReturns.length > 1) {
		const mean =
			dailyReturns.reduce((a, b) => a + b, 0) / dailyReturns.length;
		const variance =
			dailyReturns.reduce((a, r) => a + (r - mean) ** 2, 0) /
			(dailyReturns.length - 1);
		const stddev = Math.sqrt(variance);
		if (stddev > 0) sharpe = (mean / stddev) * Math.sqrt(252);
	}

	return {
		totalPnl,
		totalWinners,
		totalLosers,
		totalTrades,
		totalInvested,
		totalReturn,
		winRate,
		profitFactor,
		avgWin,
		avgLoss,
		expectancy,
		roc,
		sharpe,
		maxDrawdown,
		bestPnl: bestPnl === -Infinity ? 0 : bestPnl,
		bestSym,
		worstPnl: worstPnl === Infinity ? 0 : worstPnl,
		worstSym,
		equityPoints,
		dayStats,
	};
}

// buildMultiEquityPoints replays the same date range under all four Top-N
// pick-set selections (1 / 3 / 5 / 10) and merges the cumulative P&L series
// into one row per date with one column per series. The Equity Curve always
// renders this multi-line view so the user can compare strategies without
// toggling the toolbar filter.
function buildMultiEquityPoints(
	data: WeekResponse,
): {
	date: string;
	top1: number;
	top3: number;
	top5: number;
	top10: number;
}[] {
	const filters: { key: "top1" | "top3" | "top5" | "top10"; n: number }[] = [
		{ key: "top1", n: 1 },
		{ key: "top3", n: 3 },
		{ key: "top5", n: 5 },
		{ key: "top10", n: 10 },
	];

	const merged = new Map<
		string,
		{
			date: string;
			top1: number;
			top3: number;
			top5: number;
			top10: number;
		}
	>();

	for (const f of filters) {
		const filtered = filterByRank(data, f.n);
		if (!filtered.days?.length) continue;
		const points = computeAggregates(filtered).equityPoints;
		for (const p of points) {
			const row = merged.get(p.date) ?? {
				date: p.date,
				top1: 0,
				top3: 0,
				top5: 0,
				top10: 0,
			};
			row[f.key] = p.cumPnl;
			merged.set(p.date, row);
		}
	}

	return Array.from(merged.values()).sort((a, b) =>
		a.date.localeCompare(b.date),
	);
}

function modeToLabel(mode: string): string {
	switch (mode) {
		case "week":
			return "Weekly";
		case "month":
			return "Monthly";
		case "year":
			return "Yearly";
		case "all":
			return "All-Time";
		default:
			return "Performance";
	}
}

function EmptyState() {
	return (
		<div className="flex flex-col items-center justify-center py-16 text-center">
			<BarChart3 className="h-12 w-12 text-muted-foreground/60" aria-hidden />
			<h3 className="mt-4 text-base font-semibold">
				No trades this period
			</h3>
			<p className="mt-1 text-sm text-muted-foreground">
				Navigate to a period with trading activity.
			</p>
		</div>
	);
}

export function HistoryShell() {
	const [mode, setMode] = useState("week");
	const [rangeOffset, setRangeOffset] = useState(0);
	const [topFilter, setTopFilter] = useState(10);
	const [availableDates, setAvailableDates] = useState<string[]>([]);
	const [rawData, setRawData] = useState<WeekResponse | null>(null);

	// Restore state
	useEffect(() => {
		try {
			const raw = localStorage.getItem(STORAGE_KEY);
			if (raw) {
				const saved = JSON.parse(raw);
				if (saved.mode) setMode(saved.mode);
				if (saved.rangeOffset != null) setRangeOffset(saved.rangeOffset);
				if (saved.topFilter && [1, 3, 5, 10].includes(saved.topFilter))
					setTopFilter(saved.topFilter);
			}
		} catch {}
	}, []);

	// Save state
	useEffect(() => {
		try {
			localStorage.setItem(
				STORAGE_KEY,
				JSON.stringify({ mode, rangeOffset, topFilter }),
			);
		} catch {}
	}, [mode, rangeOffset, topFilter]);

	// Load dates
	useEffect(() => {
		api.getTradeDates(365).then((res) => {
			if (res.dates?.length) setAvailableDates(res.dates);
		});
	}, []);

	// Load range data
	const loadRange = useCallback(() => {
		const b = getRangeBounds(mode, rangeOffset);
		api.getWeekTrades(b.start, b.end).then(setRawData);
	}, [mode, rangeOffset]);

	useEffect(() => {
		loadRange();
	}, [loadRange]);

	const filtered = rawData ? filterByRank(rawData, topFilter) : null;
	const agg = filtered?.days?.length ? computeAggregates(filtered) : null;

	// Equity curve always shows all four Top-N strategies overlaid, regardless
	// of which one is currently selected by the toolbar's TopNFilter. This
	// lets the user compare how each pick-set would have performed at a
	// glance without having to toggle the filter.
	const multiEquityPoints = rawData
		? buildMultiEquityPoints(rawData)
		: [];

	const maxOffset = maxRangeOffset(mode, availableDates);
	const label = getRangeLabel(mode, rangeOffset);
	const modeLabel = modeToLabel(mode);

	const daysWithPnl = agg?.dayStats.filter((d) => d.hasSummaries) ?? [];

	const subtitle =
		filtered?.days?.length && agg
			? `${agg.totalTrades} trades across ${filtered.days.length} trading days${
					topFilter < 10 ? ` \u00B7 Top ${topFilter}` : ""
				}`
			: "Loading\u2026";

	return (
		<div className="animate-in fade-in duration-300">
			<PageToolbar
				title={`${modeLabel} Performance`}
				subtitle={subtitle}
				primaryControls={
					<TopNFilter value={topFilter} onChange={setTopFilter} />
				}
				secondaryControls={
					<div className="flex flex-wrap items-center gap-2">
						<ModeToggle
							mode={mode}
							onChange={(m) => {
								setMode(m);
								setRangeOffset(0);
							}}
						/>
						<DateRangeNav
							label={label}
							canPrev={mode !== "all" && rangeOffset < maxOffset}
							canNext={mode !== "all" && rangeOffset > 0}
							onPrev={() => setRangeOffset((o) => o + 1)}
							onNext={() => setRangeOffset((o) => o - 1)}
						/>
					</div>
				}
				rightSlot={<DataFreshness state="market-closed" asOf={undefined} />}
			/>

			<div className="mx-auto max-w-[1200px] px-4 py-6 sm:px-7">
				{!rawData ? (
					<HistorySkeleton />
				) : !filtered?.days?.length ? (
					<EmptyState />
				) : agg ? (
					<>
						{agg.totalWinners + agg.totalLosers > 0 && (
							<>
								<HistoryStats {...agg} />

								{multiEquityPoints.length > 1 && (
									<Section
										title="Equity Curve"
										subtitle="Cumulative P&L over time"
										className="mt-8"
									>
										<EquityCurveChart data={multiEquityPoints} />
									</Section>
								)}

								{daysWithPnl.length > 1 && (
									<Section
										title="Daily P&L"
										subtitle="Net result per trading day"
									>
										<DailyPnlChart data={daysWithPnl} />
									</Section>
								)}

								{daysWithPnl.length > 1 && (
									<Section
										title="Capital Deployment"
										subtitle="Invested vs returned per day"
									>
										<ExposureReturnsChart data={daysWithPnl} />
									</Section>
								)}

								<Section
									title="Capital Efficiency"
									subtitle="Period summary"
								>
									<CapitalEfficiency
										totalInvested={agg.totalInvested}
										totalReturn={agg.totalReturn}
										totalPnl={agg.totalPnl}
										roc={agg.roc}
									/>
								</Section>
							</>
						)}

						<Separator className="my-6" />

						<Section
							title="Daily Breakdown"
							subtitle="Click any day to see individual trades"
						>
							<DailyBreakdown dayStats={agg.dayStats} />
						</Section>

						<Section title="Methodology" className="mt-8">
							<MethodologySection />
						</Section>
					</>
				) : null}
			</div>
		</div>
	);
}
