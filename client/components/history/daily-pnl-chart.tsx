"use client";

import { useMemo } from "react";
import { Bar, BarChart, CartesianGrid, ReferenceLine, XAxis, YAxis } from "recharts";

import { type ChartConfig, ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { formatMonthDay } from "@/lib/date-utils";
import { fmtPnlInt } from "@/lib/format";

import type { DayMultiStat } from "./history-shell";
import { TIER_COLORS, TIER_KEYS, TIER_LABELS } from "./tiers";

type Granularity = "daily" | "weekly";

interface SeriesRow {
  date: string;
  top1: number;
  top2: number;
  top3: number;
}

function mondayKey(dateStr: string): string {
  const d = new Date(`${dateStr}T12:00:00Z`);
  const dow = d.getUTCDay();
  const offset = dow === 0 ? -6 : 1 - dow;
  d.setUTCDate(d.getUTCDate() + offset);
  return d.toISOString().slice(0, 10);
}

function buildSeries(days: DayMultiStat[], granularity: Granularity): SeriesRow[] {
  if (granularity === "daily") {
    return days.map((d) => ({
      date: d.date,
      top1: d.tiers.top1.pnl,
      top2: d.tiers.top2.pnl,
      top3: d.tiers.top3.pnl,
    }));
  }
  const buckets = new Map<string, SeriesRow>();
  for (const d of days) {
    const k = mondayKey(d.date);
    const existing = buckets.get(k) ?? { date: k, top1: 0, top2: 0, top3: 0 };
    existing.top1 += d.tiers.top1.pnl;
    existing.top2 += d.tiers.top2.pnl;
    existing.top3 += d.tiers.top3.pnl;
    buckets.set(k, existing);
  }
  return Array.from(buckets.values()).sort((a, b) => a.date.localeCompare(b.date));
}

export function DailyPnlChart({ days, granularity = "daily" }: { days: DayMultiStat[]; granularity?: Granularity }) {
  const series = useMemo(() => buildSeries(days, granularity), [days, granularity]);
  const isWeekly = granularity === "weekly";

  const chartConfig: ChartConfig = {
    top1: { label: TIER_LABELS.top1, color: TIER_COLORS.top1 },
    top2: { label: TIER_LABELS.top2, color: TIER_COLORS.top2 },
    top3: { label: TIER_LABELS.top3, color: TIER_COLORS.top3 },
  };

  return (
    <ChartContainer config={chartConfig} className="min-h-[280px] w-full">
      <BarChart data={series} accessibilityLayer margin={{ left: 4, right: 8, top: 4, bottom: 4 }}>
        <CartesianGrid vertical={false} stroke="var(--border)" strokeOpacity={0.5} />
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
          <Bar key={tier} dataKey={tier} name={TIER_LABELS[tier]} fill={TIER_COLORS[tier]} radius={[3, 3, 0, 0]} isAnimationActive={false} />
        ))}
      </BarChart>
    </ChartContainer>
  );
}
