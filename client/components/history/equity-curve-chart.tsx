"use client";

import {
	CartesianGrid,
	Line,
	LineChart,
	XAxis,
	YAxis,
} from "recharts";

import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import {
	ChartContainer,
	ChartLegend,
	ChartLegendContent,
	ChartTooltip,
	ChartTooltipContent,
	type ChartConfig,
} from "@/components/ui/chart";
import { formatMonthDay } from "@/lib/date-utils";
import { fmtPnlInt } from "@/lib/format";

export interface EquityPoint {
	date: string;
	top1: number;
	top3: number;
	top5: number;
	top10: number;
}

const SERIES = [
	{ key: "top1", label: "Top 1", color: "var(--chart-1)" },
	{ key: "top3", label: "Top 3", color: "var(--green)" },
	{ key: "top5", label: "Top 5", color: "var(--amber)" },
	{ key: "top10", label: "Top 10", color: "var(--chart-3)" },
] as const;

export function EquityCurveChart({ data }: { data: EquityPoint[] }) {
	const chartConfig: ChartConfig = Object.fromEntries(
		SERIES.map((s) => [s.key, { label: s.label, color: s.color }]),
	);

	return (
		<Card>
			<CardHeader>
				<CardTitle className="text-base">Equity Curve</CardTitle>
				<CardDescription>
					Cumulative P&amp;L over time, replayed under each Top-N pick
					selection
				</CardDescription>
			</CardHeader>
			<CardContent>
				<ChartContainer config={chartConfig} className="min-h-[280px] w-full">
					<LineChart data={data} accessibilityLayer>
						<CartesianGrid vertical={false} />
						<XAxis
							dataKey="date"
							tickLine={false}
							axisLine={false}
							tickMargin={8}
							tickFormatter={(v: string) => formatMonthDay(v)}
						/>
						<YAxis
							tickLine={false}
							axisLine={false}
							tickMargin={8}
							tickFormatter={(v: number) => fmtPnlInt(v)}
						/>
						<ChartTooltip
							content={
								<ChartTooltipContent
									labelFormatter={(_, payload) => {
										const item = payload?.[0]?.payload as
											| { date: string }
											| undefined;
										return item ? formatMonthDay(item.date) : "";
									}}
									formatter={(value) => fmtPnlInt(Number(value))}
								/>
							}
						/>
						<ChartLegend content={<ChartLegendContent />} />
						{SERIES.map((s) => (
							<Line
								key={s.key}
								type="monotone"
								dataKey={s.key}
								name={s.label}
								stroke={s.color}
								strokeWidth={2}
								dot={false}
							/>
						))}
					</LineChart>
				</ChartContainer>
			</CardContent>
		</Card>
	);
}
