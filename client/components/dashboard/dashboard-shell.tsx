"use client";

// NavBar moved to app/(app)/layout.tsx
import { CalendarX } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { DashboardSkeleton } from "@/components/layout/dashboard-skeleton";
import { DataFreshness } from "@/components/layout/data-freshness";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { Section } from "@/components/layout/section";
import { Separator } from "@/components/ui/separator";
import { api } from "@/lib/api";
import { usePicker } from "@/lib/picker-context";
import type {
	DashboardResponse,
	DashboardTrade,
	LiveQuotesResponse,
} from "@/types/trade";

import { DateNavigator } from "./date-navigator";
import { ExposurePanel } from "./exposure-panel";
import { MorningCards } from "./morning-cards";
import { PnlChart } from "./pnl-chart";
import { StatsGrid } from "./stats-grid";
import { StockChart } from "./stock-chart";
import { SymbolTabs } from "./symbol-tabs";
import { TopNFilter } from "./top-n-filter";
import { TradeTable } from "./trade-table";

const STORAGE_KEY = "jt_dash_v3";
const REFRESH_SECONDS = 60;
const LIVE_POLL_SECONDS = 15;

function filterByRank(
	data: DashboardResponse,
	topFilter: number,
): DashboardResponse {
	// Server should always return an empty array, but be defensive in
	// case any deployment regresses to nil-slice → JSON null. Mirrors
	// the same defensive coalescing the history page already does.
	const trades = data.trades ?? [];
	if (topFilter >= 10) return { ...data, trades };
	return {
		...data,
		trades: trades.filter(
			(t) => t.trade.rank >= 1 && t.trade.rank <= topFilter,
		),
	};
}

function computeStats(trades: DashboardTrade[]) {
	let winners = 0;
	let losers = 0;
	let totalPnl = 0;
	let bestPnl = -Infinity;
	let bestSym = "";
	let grossWins = 0;
	let grossLosses = 0;
	let hasSummaries = false;

	for (const { trade, summary } of trades ?? []) {
		if (summary) {
			hasSummaries = true;
			const pnl = (summary.closing_price - summary.entry_price) * 100;
			totalPnl += pnl;
			if (pnl > 0.5) {
				winners++;
				grossWins += pnl;
			} else if (pnl < -0.5) {
				losers++;
				grossLosses += Math.abs(pnl);
			}
			if (pnl > bestPnl) {
				bestPnl = pnl;
				bestSym = trade.symbol;
			}
		}
	}

	const winRate =
		winners + losers > 0 ? (winners / (winners + losers)) * 100 : 0;
	const profitFactor =
		grossLosses > 0
			? grossWins / grossLosses
			: grossWins > 0
				? Number.POSITIVE_INFINITY
				: 0;

	return { hasSummaries, totalPnl, winRate, profitFactor, bestPnl, bestSym };
}

export function DashboardShell() {
	const { picker } = usePicker();
	const [dates, setDates] = useState<string[]>([]);
	const [dayIndex, setDayIndex] = useState(0);
	const [topFilter, setTopFilter] = useState(10);
	const [rawData, setRawData] = useState<DashboardResponse | null>(null);
	const [liveQuotes, setLiveQuotes] = useState<LiveQuotesResponse | null>(
		null,
	);
	const [activeSymbol, setActiveSymbol] = useState("");
	const [chartTimeframe] = useState({
		period: 5,
		ptype: "day",
		ftype: "minute",
		freq: 5,
	});

	// Restore state from localStorage
	useEffect(() => {
		try {
			const raw = localStorage.getItem(STORAGE_KEY);
			if (raw) {
				const saved = JSON.parse(raw);
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
				JSON.stringify({
					topFilter,
					date: dates[dayIndex],
				}),
			);
		} catch {}
	}, [topFilter, dayIndex, dates]);

	// Load dates. If the API returns an empty list (fresh deploy with
	// no historical trades yet), we still kick off loadDay() so the
	// shell renders the EmptyState instead of an infinite skeleton.
	const [datesFetched, setDatesFetched] = useState(false);
	useEffect(() => {
		api.getTradeDates()
			.then((res) => {
				if (res.dates?.length) {
					setDates(res.dates);
					try {
						const raw = localStorage.getItem(STORAGE_KEY);
						if (raw) {
							const saved = JSON.parse(raw);
							if (saved.date) {
								const idx = res.dates.indexOf(saved.date);
								if (idx >= 0) setDayIndex(idx);
							}
						}
					} catch {}
				}
			})
			.finally(() => setDatesFetched(true));
	}, []);

	// Load day data
	const loadDay = useCallback(() => {
		const date = dates[dayIndex];
		api.getTrades(date, picker).then(setRawData);
	}, [dates, dayIndex, picker]);

	useEffect(() => {
		// Fire loadDay on every dates change OR once dates have been
		// fetched (even if the result was empty). The empty-dates branch
		// still calls api.getTrades(undefined) which the server handles
		// by returning an empty dashboardResponse{}, letting the shell
		// render its EmptyState branch instead of the loading skeleton.
		if (dates.length > 0 || datesFetched) loadDay();
	}, [loadDay, dates, datesFetched]);

	// Auto-refresh
	useEffect(() => {
		const interval = setInterval(loadDay, REFRESH_SECONDS * 1000);
		return () => clearInterval(interval);
	}, [loadDay]);

	// Live quotes polling
	useEffect(() => {
		if (!rawData?.trades?.length) return;
		const hasSummaries = rawData.trades.some((t) => t.summary);
		if (hasSummaries) return;

		const poll = () => api.getLiveQuotes().then(setLiveQuotes);
		poll();
		const interval = setInterval(poll, LIVE_POLL_SECONDS * 1000);
		return () => clearInterval(interval);
	}, [rawData]);

	const filtered = rawData ? filterByRank(rawData, topFilter) : null;
	const stats = filtered?.trades ? computeStats(filtered.trades) : null;

	// Set first symbol when data loads
	useEffect(() => {
		if (filtered?.trades?.length && !activeSymbol) {
			setActiveSymbol(filtered.trades[0].trade.symbol);
		}
	}, [filtered, activeSymbol]);

	const freshnessState: "loading" | "live" | "market-closed" | "pre-market" =
		!rawData
			? "loading"
			: !liveQuotes
				? stats?.hasSummaries
					? "market-closed"
					: "pre-market"
				: liveQuotes.market_open
					? "live"
					: "market-closed";

	const title = stats?.hasSummaries ? "End of Day Results" : "Today's Plays";
	const subtitle = filtered?.trades?.length
		? `${filtered.trades.length} options picks${
				topFilter < 10 ? ` · Top ${topFilter}` : ""
			}${stats?.hasSummaries ? "" : " · 0–7 DTE · Under $200/contract"}`
		: "Loading…";

	return (
		<div className="animate-in fade-in duration-300">
			<PageToolbar
				title={title}
				subtitle={subtitle}
				primaryControls={
					<TopNFilter value={topFilter} onChange={setTopFilter} />
				}
				secondaryControls={
					<DateNavigator
						dates={dates}
						index={dayIndex}
						onChange={setDayIndex}
					/>
				}
				rightSlot={
					<DataFreshness
						state={freshnessState}
						asOf={liveQuotes?.as_of}
					/>
				}
			/>

			<div className="mx-auto max-w-[1200px] px-4 py-6 sm:px-7">
				{!rawData ? (
					<DashboardSkeleton />
				) : !filtered?.trades?.length ? (
					<EmptyState />
				) : stats?.hasSummaries ? (
					<>
						<StatsGrid
							totalPnl={stats.totalPnl}
							winRate={stats.winRate}
							profitFactor={stats.profitFactor}
							bestPnl={stats.bestPnl}
							bestSym={stats.bestSym}
						/>
						<Section title="Price Chart" className="mt-8">
							<SymbolTabs
								trades={filtered.trades}
								activeSymbol={activeSymbol}
								onSelect={setActiveSymbol}
							/>
							<div className="mt-3 h-[280px] overflow-hidden rounded-lg border bg-card sm:h-[360px] lg:h-[420px]">
								{activeSymbol && (() => {
									const dt = filtered.trades.find((t) => t.trade.symbol === activeSymbol);
									return (
										<StockChart
											symbol={activeSymbol}
											timeframe={chartTimeframe}
											strikePrice={dt?.trade.strike_price}
											trade={dt?.trade}
											summary={dt?.summary ?? undefined}
										/>
									);
								})()}
							</div>
						</Section>
						<Section
							title="Exposure Analysis"
							subtitle="How capital was deployed today"
						>
							<ExposurePanel
								trades={filtered.trades}
								hasSummaries
							/>
						</Section>
						<Section
							title="P&L by Trade"
							subtitle="Per-contract performance, sorted"
						>
							<PnlChart trades={filtered.trades} />
						</Section>
						<Separator />
						<Section
							title="Trade Details"
							subtitle="Click any row to expand"
						>
							<TradeTable trades={filtered.trades} />
						</Section>
					</>
				) : (
					<>
						<Section title="Price Chart">
							<SymbolTabs
								trades={filtered.trades}
								activeSymbol={activeSymbol}
								onSelect={setActiveSymbol}
							/>
							<div className="mt-3 h-[280px] overflow-hidden rounded-lg border bg-card sm:h-[360px] lg:h-[420px]">
								{activeSymbol && (() => {
									const dt = filtered.trades.find((t) => t.trade.symbol === activeSymbol);
									return (
										<StockChart
											symbol={activeSymbol}
											timeframe={chartTimeframe}
											strikePrice={dt?.trade.strike_price}
											trade={dt?.trade}
											summary={dt?.summary ?? undefined}
										/>
									);
								})()}
							</div>
						</Section>
						<Section
							title="Exposure"
							subtitle="Capital at risk for today's picks"
						>
							<ExposurePanel
								trades={filtered.trades}
								hasSummaries={false}
							/>
						</Section>
						<Section
							title="Today's Picks"
							subtitle={`${filtered.trades.length} ranked plays`}
						>
							<MorningCards
								trades={filtered.trades}
								liveQuotes={liveQuotes}
							/>
						</Section>
					</>
				)}
			</div>
		</div>
	);
}

function EmptyState() {
	return (
		<div className="flex flex-col items-center justify-center py-16 text-center">
			<CalendarX className="h-12 w-12 text-muted-foreground/50" />
			<h3 className="mt-4 text-base font-semibold">
				No trades for this date
			</h3>
			<p className="mt-1 text-sm text-muted-foreground">
				Trades publish at 9:25 AM ET on market days.
			</p>
		</div>
	);
}
