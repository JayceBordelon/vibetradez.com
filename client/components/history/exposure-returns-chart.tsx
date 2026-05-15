"use client";

import { useMemo } from "react";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";

import { type ChartConfig, ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { formatMonthDay } from "@/lib/date-utils";
import { fmtMoneyInt } from "@/lib/format";

import type { DayMultiStat } from "./history-shell";
import { TIER_COLORS, TIER_KEYS, TIER_LABELS, TIER_SUBLABELS, type TierKey } from "./tiers";

interface DayPair {
  date: string;
  invested: number;
  returned: number;
}

function buildSeries(days: DayMultiStat[], tier: TierKey): DayPair[] {
  return days.map((d) => ({ date: d.date, invested: d.tiers[tier].invested, returned: d.tiers[tier].returned })).filter((d) => d.invested > 0 || d.returned > 0);
}

function TierColumn({ tier, days }: { tier: TierKey; days: DayMultiStat[] }) {
  const series = useMemo(() => buildSeries(days, tier), [days, tier]);
  const accent = TIER_COLORS[tier];

  const chartConfig: ChartConfig = {
    invested: { label: "Invested", color: "var(--muted-foreground)" },
    returned: { label: "Returned", color: accent },
  };

  return (
    <div className="min-w-0">
      <div className="mb-2 flex items-center gap-2">
        <span className="h-2 w-2 rounded-full" style={{ backgroundColor: accent }} aria-hidden />
        <span className="text-sm font-semibold">{TIER_LABELS[tier]}</span>
        <span className="text-[11px] text-muted-foreground">{TIER_SUBLABELS[tier]}</span>
      </div>
      <ChartContainer config={chartConfig} className="min-h-[180px] w-full">
        <BarChart data={series} accessibilityLayer margin={{ left: 4, right: 8, top: 4, bottom: 4 }}>
          <CartesianGrid vertical={false} stroke="var(--border)" strokeOpacity={0.5} />
          <XAxis dataKey="date" tickLine={false} axisLine={false} tickMargin={8} tickFormatter={(v: string) => formatMonthDay(v)} />
          <YAxis tickLine={false} axisLine={false} tickMargin={8} tickFormatter={(v: number) => fmtMoneyInt(v)} />
          <ChartTooltip
            content={
              <ChartTooltipContent
                labelFormatter={(_, payload) => {
                  const item = payload?.[0]?.payload as { date: string } | undefined;
                  return item ? formatMonthDay(item.date) : "";
                }}
                formatter={(value) => fmtMoneyInt(value as number)}
              />
            }
          />
          <ChartLegend content={<ChartLegendContent />} />
          <Bar dataKey="invested" name="Invested" fill="var(--color-invested)" radius={[3, 3, 0, 0]} isAnimationActive={false} />
          <Bar dataKey="returned" name="Returned" fill="var(--color-returned)" radius={[3, 3, 0, 0]} isAnimationActive={false} />
        </BarChart>
      </ChartContainer>
    </div>
  );
}

export function ExposureReturnsChart({ days }: { days: DayMultiStat[] }) {
  return (
    <div className="grid grid-cols-1 gap-6 md:grid-cols-3 md:gap-x-8">
      {TIER_KEYS.map((tier) => (
        <TierColumn key={tier} tier={tier} days={days} />
      ))}
    </div>
  );
}
