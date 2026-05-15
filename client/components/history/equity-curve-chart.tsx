"use client";

import { CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { type ChartConfig, ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { formatMonthDay } from "@/lib/date-utils";
import { fmtPnlInt } from "@/lib/format";

export interface EquityPoint {
  date: string;
  cumPnl: number;
}

const chartConfig: ChartConfig = {
  cumPnl: { label: "Cumulative P&L", color: "var(--chart-3)" },
};

export function EquityCurveChart({ data }: { data: EquityPoint[] }) {
  return (
    <Card className="lg-card">
      <CardHeader>
        <CardTitle className="text-base">Equity Curve</CardTitle>
        <CardDescription>Cumulative P&amp;L over time</CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="min-h-[280px] w-full">
          <LineChart data={data} accessibilityLayer>
            <CartesianGrid vertical={false} />
            <XAxis dataKey="date" tickLine={false} axisLine={false} tickMargin={8} tickFormatter={(v: string) => formatMonthDay(v)} />
            <YAxis tickLine={false} axisLine={false} tickMargin={8} tickFormatter={(v: number) => fmtPnlInt(v)} />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  labelFormatter={(_, payload) => {
                    const item = payload?.[0]?.payload as { date: string } | undefined;
                    return item ? formatMonthDay(item.date) : "";
                  }}
                  formatter={(value) => fmtPnlInt(Number(value))}
                />
              }
            />
            <Line type="monotone" dataKey="cumPnl" name="Cumulative P&L" stroke="var(--chart-3)" strokeWidth={2.5} dot={false} />
          </LineChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
