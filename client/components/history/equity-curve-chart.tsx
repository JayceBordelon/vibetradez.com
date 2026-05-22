"use client";

import { useMemo } from "react";
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts";

import { type ChartConfig, ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { formatMonthDay } from "@/lib/date-utils";
import { fmtPnlInt } from "@/lib/format";

import type { DayMultiStat } from "./history-shell";
import { TIER_COLORS, TIER_KEYS, TIER_LABELS } from "./tiers";

const chartConfig: ChartConfig = {
  top1: { label: TIER_LABELS.top1, color: TIER_COLORS.top1 },
  top2: { label: TIER_LABELS.top2, color: TIER_COLORS.top2 },
  top3: { label: TIER_LABELS.top3, color: TIER_COLORS.top3 },
};

export function EquityCurveChart({ days }: { days: DayMultiStat[] }) {
  const data = useMemo(
    () =>
      days.map((d) => ({
        date: d.date,
        top1: d.tiers.top1.cumPnl,
        top2: d.tiers.top2.cumPnl,
        top3: d.tiers.top3.cumPnl,
      })),
    [days]
  );

  return (
    <ChartContainer config={chartConfig} className="min-h-[300px] w-full">
      <LineChart data={data} accessibilityLayer margin={{ left: 4, right: 8, top: 4, bottom: 4 }}>
        <CartesianGrid vertical={false} stroke="var(--border)" strokeOpacity={0.5} />
        <XAxis dataKey="date" tickLine={false} axisLine={false} tickMargin={8} tickFormatter={(v: string) => formatMonthDay(v)} />
        <YAxis tickLine={false} axisLine={false} tickMargin={8} tickFormatter={(v: number) => fmtPnlInt(v)} />
        <ChartTooltip
          content={
            <ChartTooltipContent
              labelFormatter={(_, payload) => {
                const item = payload?.[0]?.payload as { date: string } | undefined;
                return item ? formatMonthDay(item.date) : "";
              }}
              formatter={(value, name) => (
                <div className="flex w-full items-center justify-between gap-3">
                  <span className="text-muted-foreground">{TIER_LABELS[name as keyof typeof TIER_LABELS] ?? String(name)}</span>
                  <span className="font-mono font-medium tabular-nums">{fmtPnlInt(Number(value))}</span>
                </div>
              )}
            />
          }
        />
        <ChartLegend content={<ChartLegendContent />} />
        {TIER_KEYS.map((tier) => (
          <Line
            key={tier}
            type="monotone"
            dataKey={tier}
            name={TIER_LABELS[tier]}
            stroke={TIER_COLORS[tier]}
            strokeWidth={tier === "top1" ? 2.5 : tier === "top3" ? 2 : 1.75}
            dot={false}
            isAnimationActive={false}
          />
        ))}
      </LineChart>
    </ChartContainer>
  );
}
