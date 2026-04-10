"use client";

import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";

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

export function EquityCurveChart({
	data,
}: {
	data: { date: string; cumPnl: number }[];
}) {
	const final = data.length > 0 ? data[data.length - 1].cumPnl : 0;
	const positive = final >= 0;
	const strokeColor = positive ? "var(--green)" : "var(--red)";
	const gradientId = "equityGradient";

	const chartConfig: ChartConfig = {
		cumPnl: {
			label: "Cumulative P&L",
			color: strokeColor,
		},
	};

	return (
		<Card>
			<CardHeader>
				<CardTitle className="text-base">Equity Curve</CardTitle>
				<CardDescription>Cumulative P&amp;L over time</CardDescription>
			</CardHeader>
			<CardContent>
				<ChartContainer config={chartConfig} className="min-h-[260px] w-full">
					<AreaChart data={data} accessibilityLayer>
						<defs>
							<linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
								<stop
									offset="0%"
									stopColor={strokeColor}
									stopOpacity={0.3}
								/>
								<stop
									offset="100%"
									stopColor={strokeColor}
									stopOpacity={0.02}
								/>
							</linearGradient>
						</defs>
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
									formatter={(value) => fmtPnlInt(value as number)}
								/>
							}
						/>
						<Area
							dataKey="cumPnl"
							type="monotone"
							stroke={strokeColor}
							strokeWidth={2}
							fill={`url(#${gradientId})`}
						/>
					</AreaChart>
				</ChartContainer>
			</CardContent>
		</Card>
	);
}
