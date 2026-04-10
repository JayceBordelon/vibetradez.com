"use client";

import {
	Bar,
	BarChart,
	CartesianGrid,
	Cell,
	ReferenceLine,
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
	ChartTooltip,
	ChartTooltipContent,
	type ChartConfig,
} from "@/components/ui/chart";
import { formatMonthDay } from "@/lib/date-utils";
import { fmtPnlInt } from "@/lib/format";

const GREEN = "var(--green)";
const RED = "var(--red)";

export function DailyPnlChart({
	data,
}: {
	data: { date: string; pnl: number }[];
}) {
	const chartConfig: ChartConfig = {
		pnl: {
			label: "Daily P&L",
			color: GREEN,
		},
	};

	return (
		<Card>
			<CardHeader>
				<CardTitle className="text-base">Daily P&amp;L</CardTitle>
				<CardDescription>Net P&amp;L per trading day</CardDescription>
			</CardHeader>
			<CardContent>
				<ChartContainer config={chartConfig} className="min-h-[260px] w-full">
					<BarChart data={data} accessibilityLayer>
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
						<ReferenceLine y={0} stroke="var(--border)" />
						<ChartTooltip
							content={
								<ChartTooltipContent
									labelFormatter={(_, payload) => {
										const item = payload?.[0]?.payload as
											| { date: string }
											| undefined;
										return item ? formatMonthDay(item.date) : "";
									}}
									formatter={(value) => fmtPnlInt(value as number)}
								/>
							}
						/>
						<Bar dataKey="pnl" radius={[3, 3, 0, 0]}>
							{data.map((entry, index) => (
								<Cell
									key={`cell-${index}`}
									fill={entry.pnl >= 0 ? GREEN : RED}
								/>
							))}
						</Bar>
					</BarChart>
				</ChartContainer>
			</CardContent>
		</Card>
	);
}
