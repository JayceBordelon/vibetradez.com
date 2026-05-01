"use client";

import { useMemo } from "react";
import { Bar, BarChart, CartesianGrid, Cell, ReferenceLine, XAxis, YAxis } from "recharts";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { type ChartConfig, ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { formatMonthDay } from "@/lib/date-utils";
import { fmtPnlInt } from "@/lib/format";

const GREEN = "var(--green)";
const RED = "var(--red)";

/**
mondayKey returns the Monday-of-week (YYYY-MM-DD) for a YYYY-MM-DD date,
so a year/all-time series can be aggregated into weekly bars instead of
hundreds of unreadable daily ticks.
*/
function mondayKey(dateStr: string): string {
  // Parse as UTC noon to dodge DST edges; result formatted as YYYY-MM-DD.
  const d = new Date(`${dateStr}T12:00:00Z`);
  const dow = d.getUTCDay(); // 0 = Sun, 1 = Mon, ...
  const offset = dow === 0 ? -6 : 1 - dow; // shift back to Monday
  d.setUTCDate(d.getUTCDate() + offset);
  return d.toISOString().slice(0, 10);
}

function aggregateWeekly(rows: { date: string; pnl: number }[]): { date: string; pnl: number }[] {
  const buckets = new Map<string, number>();
  for (const r of rows) {
    const k = mondayKey(r.date);
    buckets.set(k, (buckets.get(k) ?? 0) + r.pnl);
  }
  return Array.from(buckets.entries())
    .map(([date, pnl]) => ({ date, pnl }))
    .sort((a, b) => a.date.localeCompare(b.date));
}

export function DailyPnlChart({ data, granularity = "daily" }: { data: { date: string; pnl: number }[]; granularity?: "daily" | "weekly" }) {
  const series = useMemo(() => (granularity === "weekly" ? aggregateWeekly(data) : data), [data, granularity]);
  const isWeekly = granularity === "weekly";

  const chartConfig: ChartConfig = {
    pnl: {
      label: isWeekly ? "Weekly P&L" : "Daily P&L",
      color: GREEN,
    },
  };

  return (
    <Card className="lg-card">
      <CardHeader>
        <CardTitle className="text-base">{isWeekly ? "Weekly P&L" : "Daily P&L"}</CardTitle>
        <CardDescription>{isWeekly ? "Net P&L per trading week (Monday–Friday)" : "Net P&L per trading day"}</CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="min-h-[260px] w-full">
          <BarChart data={series} accessibilityLayer>
            <CartesianGrid vertical={false} />
            <XAxis dataKey="date" tickLine={false} axisLine={false} tickMargin={8} tickFormatter={(v: string) => formatMonthDay(v)} />
            <YAxis tickLine={false} axisLine={false} tickMargin={8} tickFormatter={(v: number) => fmtPnlInt(v)} />
            <ReferenceLine y={0} stroke="var(--border)" />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  labelFormatter={(_, payload) => {
                    const item = payload?.[0]?.payload as { date: string } | undefined;
                    if (!item) return "";
                    return isWeekly ? `Week of ${formatMonthDay(item.date)}` : formatMonthDay(item.date);
                  }}
                  formatter={(value) => fmtPnlInt(value as number)}
                />
              }
            />
            <Bar dataKey="pnl" radius={[3, 3, 0, 0]}>
              {series.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.pnl >= 0 ? GREEN : RED} />
              ))}
            </Bar>
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
