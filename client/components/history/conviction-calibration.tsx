"use client";

import { useMemo } from "react";
import { Bar, BarChart, CartesianGrid, LabelList, XAxis, YAxis } from "recharts";

import { type ChartConfig, ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";

import type { DayMultiStat } from "./history-shell";

interface ScoreBucket {
  /** Conviction score 1..10 as Claudia assigned it on the morning pick. */
  score: number;
  /** Trades that closed at least $0.50 in the green. */
  winners: number;
  /** Trades that closed at least $0.50 in the red. */
  losers: number;
  /** Trades that closed within ±$0.50 (treated as flat, neither bucket). */
  flat: number;
  /** Sample size: winners + losers + flat. */
  total: number;
  /**
  Win rate measured across decided trades only (winners / (winners + losers)).
  Flats are excluded from the denominator so a row of 50% pure-flat outcomes
  doesn't poison the rate.
  */
  winRatePct: number;
  /**
  Pre-formatted bar label. Pre-computing keeps the LabelList formatter
  typing simple (LabelFormatter expects a single LabelValue arg in
  Recharts v3) and handles the zero-decided edge case at the data
  layer instead of in the formatter.
  */
  winRateLabel: string;
}

function bucketByScore(days: DayMultiStat[]): ScoreBucket[] {
  const map = new Map<number, ScoreBucket>();
  for (const day of days) {
    // Source from the full top-3 basket so we get the maximum sample
    // size, the calibration question is "across every pick Claudia
    // made", not "across the hero pick only".
    for (const td of day.tiers.top3.details) {
      if (!td.score || td.score < 1) continue;
      const bucket = map.get(td.score) ?? {
        score: td.score,
        winners: 0,
        losers: 0,
        flat: 0,
        total: 0,
        winRatePct: 0,
        winRateLabel: "",
      };
      bucket.total++;
      if (td.result === "profit") bucket.winners++;
      else if (td.result === "loss") bucket.losers++;
      else bucket.flat++;
      map.set(td.score, bucket);
    }
  }
  const out = Array.from(map.values()).sort((a, b) => a.score - b.score);
  for (const b of out) {
    const decided = b.winners + b.losers;
    b.winRatePct = decided > 0 ? Math.round((b.winners / decided) * 100) : 0;
    b.winRateLabel = decided > 0 ? `${b.winRatePct}%` : "-";
  }
  return out;
}

const chartConfig: ChartConfig = {
  winners: { label: "Winners", color: "var(--brand-green)" },
  losers: { label: "Losers", color: "var(--brand-red)" },
};

export function ConvictionCalibration({ days }: { days: DayMultiStat[] }) {
  const data = useMemo(() => bucketByScore(days), [days]);
  if (data.length === 0) return null;

  const totalTrades = data.reduce((sum, b) => sum + b.total, 0);
  const totalDecided = data.reduce((sum, b) => sum + b.winners + b.losers, 0);
  const totalWinners = data.reduce((sum, b) => sum + b.winners, 0);
  const overallWinRate = totalDecided > 0 ? Math.round((totalWinners / totalDecided) * 100) : 0;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 text-[11px] text-muted-foreground">
        <span>
          <span className="font-semibold text-foreground tabular-nums">{totalTrades}</span> settled trades · overall win rate{" "}
          <span className="font-semibold text-foreground tabular-nums">{overallWinRate}%</span>
        </span>
        <span>Calibration: higher scores should win more often</span>
      </div>

      <ChartContainer config={chartConfig} className="min-h-[280px] w-full">
        <BarChart data={data} accessibilityLayer margin={{ left: 4, right: 8, top: 24, bottom: 4 }}>
          <CartesianGrid vertical={false} stroke="var(--border)" strokeOpacity={0.5} />
          <XAxis
            dataKey="score"
            tickLine={false}
            axisLine={false}
            tickMargin={8}
            tickFormatter={(v: number) => `${v}/10`}
            label={{
              value: "Conviction score",
              position: "insideBottom",
              offset: -2,
              style: { fontSize: 10, fill: "var(--muted-foreground)" },
            }}
          />
          <YAxis tickLine={false} axisLine={false} tickMargin={8} allowDecimals={false} />
          <ChartTooltip
            content={
              <ChartTooltipContent
                labelFormatter={(label) => `Score ${label}/10`}
                formatter={(value, name, item) => {
                  const bucket = item?.payload as ScoreBucket | undefined;
                  if (!bucket) return null;
                  return (
                    <div className="flex w-full items-center justify-between gap-3">
                      <span className="text-muted-foreground">
                        {name === "winners" ? "Winners" : "Losers"}
                        {bucket.flat > 0 && name === "losers" ? ` · ${bucket.flat} flat` : ""}
                      </span>
                      <span className="font-mono font-medium tabular-nums">{Number(value)}</span>
                    </div>
                  );
                }}
              />
            }
          />
          <ChartLegend content={<ChartLegendContent />} />
          <Bar dataKey="winners" name="winners" fill="var(--brand-green)" stackId="result" radius={[0, 0, 0, 0]} isAnimationActive={false} />
          <Bar dataKey="losers" name="losers" fill="var(--claude)" stackId="result" radius={[3, 3, 0, 0]} isAnimationActive={false}>
            <LabelList
              dataKey="winRateLabel"
              position="top"
              style={{
                fontSize: 10,
                fontWeight: 600,
                fill: "var(--foreground)",
              }}
            />
          </Bar>
        </BarChart>
      </ChartContainer>

      <p className="text-[11px] leading-relaxed text-muted-foreground">
        Each bar is the number of settled trades at that conviction score, split into winners and losers per the legend. The number above each bar is the win rate among decided trades. If
        Claudia&apos;s scores are well-calibrated, win rate should climb monotonically left-to-right.
      </p>
    </div>
  );
}
