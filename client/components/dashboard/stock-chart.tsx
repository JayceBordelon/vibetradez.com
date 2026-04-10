"use client";

import { useEffect, useRef, useState } from "react";
import { createChart, CrosshairMode } from "lightweight-charts";
import type { IChartApi, Time } from "lightweight-charts";
import { useTheme } from "next-themes";
import { api } from "@/lib/api";
import type { ChartParams } from "@/types/trade";

interface StockChartProps {
	symbol: string;
	timeframe: ChartParams;
}

const GREEN = "#10b981";
const RED = "#ef4444";

export function StockChart({ symbol, timeframe }: StockChartProps) {
	const containerRef = useRef<HTMLDivElement>(null);
	const chartRef = useRef<IChartApi | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const { resolvedTheme } = useTheme();

	useEffect(() => {
		if (!containerRef.current) return;

		const isDark = resolvedTheme === "dark";

		const bgColor = isDark ? "#0a0a0a" : "#ffffff";
		const textColor = isDark ? "#a1a1aa" : "#71717a";
		const gridColor = isDark
			? "rgba(255,255,255,0.04)"
			: "rgba(0,0,0,0.06)";

		const chart = createChart(containerRef.current, {
			width: containerRef.current.clientWidth,
			height: 400,
			layout: {
				background: { color: bgColor },
				textColor,
			},
			grid: {
				vertLines: { color: gridColor },
				horzLines: { color: gridColor },
			},
			crosshair: {
				mode: CrosshairMode.Normal,
			},
			rightPriceScale: {
				borderColor: gridColor,
			},
			timeScale: {
				borderColor: gridColor,
				timeVisible: true,
			},
		});

		chartRef.current = chart;

		const candleSeries = chart.addCandlestickSeries({
			upColor: GREEN,
			downColor: RED,
			borderUpColor: GREEN,
			borderDownColor: RED,
			wickUpColor: GREEN,
			wickDownColor: RED,
		});

		const volumeSeries = chart.addHistogramSeries({
			priceFormat: { type: "volume" },
			priceScaleId: "volume",
		});

		chart.priceScale("volume").applyOptions({
			scaleMargins: { top: 0.8, bottom: 0 },
		});

		setLoading(true);
		setError(null);

		api
			.getChartData(symbol, timeframe)
			.then((data) => {
				if (!data?.candles?.length) {
					setError("No chart data available");
					setLoading(false);
					return;
				}

				const candles = data.candles.map((c) => ({
					time: c.time as Time,
					open: c.open,
					high: c.high,
					low: c.low,
					close: c.close,
				}));

				const volumes = data.candles.map((c) => ({
					time: c.time as Time,
					value: c.volume,
					color:
						c.close >= c.open
							? `${GREEN}40`
							: `${RED}40`,
				}));

				candleSeries.setData(candles);
				volumeSeries.setData(volumes);
				chart.timeScale().fitContent();
				setLoading(false);
			})
			.catch(() => {
				setError("Chart unavailable");
				setLoading(false);
			});

		const resizeObserver = new ResizeObserver((entries) => {
			for (const entry of entries) {
				const { width } = entry.contentRect;
				chart.applyOptions({ width });
			}
		});

		resizeObserver.observe(containerRef.current);

		return () => {
			resizeObserver.disconnect();
			chart.remove();
			chartRef.current = null;
		};
	}, [symbol, timeframe, resolvedTheme]);

	return (
		<div className="relative">
			{loading && !error && (
				<div className="absolute inset-0 z-10 flex items-center justify-center rounded-lg bg-card/80">
					<span className="text-sm text-muted-foreground">
						Loading chart...
					</span>
				</div>
			)}
			{error && (
				<div className="absolute inset-0 z-10 flex items-center justify-center rounded-lg bg-card/80">
					<span className="text-sm text-muted-foreground">
						{error}
					</span>
				</div>
			)}
			<div ref={containerRef} className="h-[400px] w-full rounded-lg" />
		</div>
	);
}
