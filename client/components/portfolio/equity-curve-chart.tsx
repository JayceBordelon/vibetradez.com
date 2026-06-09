"use client";

import { useMemo } from "react";
import { Area, AreaChart, CartesianGrid, ReferenceLine, XAxis, YAxis } from "recharts";

import { AnimatedNumber } from "@/components/ui/animated-number";
import { type ChartConfig, ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { cn } from "@/lib/utils";
import type { EquityCurvePoint } from "@/types/portfolio";

interface EquityCurveChartProps {
  points: EquityCurvePoint[];
}

/*
The dashboard chart is a dollar P&L decomposition over the window, three
series on one axis:

  realized    cumulative P&L of round trips the model has closed (booked)
  unrealized  the open book's mark-to-market on each EOD snapshot
  spy         what buy-and-hold SPY would have earned on the same starting
              equity (the dashed benchmark ghost)

Series colors double as the legend: the header stats above the plot carry
matching color keys, so the chart needs no separate legend block. Realized
wears the semantic green and a soft fill anchored to the zero baseline;
unrealized wears the clay accent the site uses for the model's own voice
(its open bets); SPY stays the muted dashed reference.
*/
const chartConfig: ChartConfig = {
  realized: { label: "Realized", color: "var(--green)" },
  unrealized: { label: "Unrealized", color: "var(--claude)" },
  spy: { label: "SPY", color: "var(--muted-foreground)" },
};

function buildSeries(points: EquityCurvePoint[]) {
  const firstEquity = points.find((p) => p.account_equity > 0)?.account_equity ?? 0;
  const firstSpy = points.find((p) => p.spy_close > 0)?.spy_close ?? 0;
  return points.map((p) => ({
    date: p.date,
    realized: p.realized_cum,
    unrealized: p.unrealized,
    // Hypothetical buy-and-hold: the window's starting equity riding SPY.
    spy: firstEquity > 0 && firstSpy > 0 && p.spy_close > 0 ? firstEquity * (p.spy_close / firstSpy - 1) : null,
  }));
}

function lastValue(series: Array<number | null>): number {
  for (let i = series.length - 1; i >= 0; i--) {
    const v = series[i];
    if (v != null) return v;
  }
  return 0;
}

// Signed compact dollars for the headline stats ("+$1,204", "-$86").
function fmtUsdSigned(n: number): string {
  const body = Math.abs(n).toLocaleString("en-US", { maximumFractionDigits: 0 });
  return `${n < 0 ? "-" : "+"}$${body}`;
}

// Axis ticks: compact unsigned dollars, negative as -$120.
function fmtUsdTick(n: number): string {
  const body = Math.abs(n).toLocaleString("en-US", { maximumFractionDigits: 0 });
  return `${n < 0 ? "-" : ""}$${body}`;
}

const TICK_MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

// fmtTick abbreviates an ISO date for the x-axis ("Jun 9", or "Jun 27 '25"
// when the point is from an earlier year than the curve's last point), so a
// year-long window doesn't render ten full YYYY-MM-DD strings. Parsed from
// the parts so it never shifts a day across timezones.
function fmtTick(iso: string, lastYear: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!m) return iso;
  const month = TICK_MONTHS[Number(m[2]) - 1] ?? m[2];
  const base = `${month} ${Number(m[3])}`;
  return m[1] === lastYear ? base : `${base} '${m[1].slice(2)}`;
}

/*
HeadlineStat is one color-keyed figure in the strip above the plot. The
2px key bar matches its series stroke exactly, so the strip IS the legend.
Values tween on live refresh (AnimatedNumber), so a poll tick reads as the
number moving rather than flickering.
*/
function HeadlineStat({ label, value, pctOfStart, colorVar, dashed }: { label: string; value: number; pctOfStart: number | null; colorVar: string; dashed?: boolean }) {
  return (
    <div>
      <div className="flex items-center gap-1.5">
        <span
          aria-hidden
          className="h-0.5 w-4 rounded-full"
          style={dashed ? { backgroundImage: `repeating-linear-gradient(90deg, ${colorVar} 0 3px, transparent 3px 5px)` } : { backgroundColor: colorVar }}
        />
        <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</span>
      </div>
      <div className="mt-0.5 text-2xl font-bold tabular-nums" style={{ color: colorVar }}>
        <AnimatedNumber value={value} kind="usdSigned" />
      </div>
      {pctOfStart !== null && (
        <div className="text-[11px] text-muted-foreground">
          <AnimatedNumber value={pctOfStart} kind="pct1" /> of starting equity
        </div>
      )}
    </div>
  );
}

export function EquityCurveChart({ points }: EquityCurveChartProps) {
  const data = useMemo(() => buildSeries(points), [points]);

  if (data.length < 2) {
    return <div className="flex h-56 items-center justify-center text-sm text-muted-foreground">Not enough history yet to chart the curve. It fills in once the daily snapshot has run for a few sessions.</div>;
  }

  const realized = lastValue(data.map((d) => d.realized));
  const unrealized = lastValue(data.map((d) => d.unrealized));
  const spyPnl = lastValue(data.map((d) => d.spy));
  const total = realized + unrealized;
  const edge = total - spyPnl;
  const firstEquity = points.find((p) => p.account_equity > 0)?.account_equity ?? 0;
  const pctOfStart = (n: number) => (firstEquity > 0 ? (n / firstEquity) * 100 : null);
  const lastYear = data[data.length - 1].date.slice(0, 4);

  return (
    <div>
      {/* Headline strip: the color keys ARE the plot legend. */}
      <div className="mb-5 flex flex-wrap items-end gap-x-8 gap-y-3">
        <HeadlineStat label="Realized" value={realized} pctOfStart={pctOfStart(realized)} colorVar="var(--green)" />
        <HeadlineStat label="Unrealized" value={unrealized} pctOfStart={pctOfStart(unrealized)} colorVar="var(--claude)" />
        <HeadlineStat label="SPY buy & hold" value={spyPnl} pctOfStart={pctOfStart(spyPnl)} colorVar="var(--muted-foreground)" dashed />
        <div className={cn("mb-1 rounded-full border px-2.5 py-1 text-xs font-semibold", edge >= 0 ? "border-green-border bg-green-bg text-green" : "border-red-border bg-red-bg text-red")}>
          {edge >= 0 ? "Beating SPY by " : "Trailing SPY by "}
          <AnimatedNumber value={Math.abs(edge)} kind="moneyInt" />
        </div>
      </div>

      <ChartContainer config={chartConfig} className="h-64 w-full">
        <AreaChart data={data} margin={{ top: 8, right: 12, left: 4, bottom: 4 }}>
          <defs>
            <linearGradient id="realizedFill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--green)" stopOpacity={0.22} />
              <stop offset="92%" stopColor="var(--green)" stopOpacity={0} />
            </linearGradient>
          </defs>
          {/* --chart-grid is the token tuned per theme for in-plot reference
              lines; var(--border) at half opacity disappeared in light mode. */}
          <CartesianGrid vertical={false} stroke="var(--chart-grid)" />
          {/* P&L is signed, so the zero baseline is the anchor the eye
              measures everything against: slightly stronger than the grid. */}
          <ReferenceLine y={0} stroke="var(--chart-text)" strokeOpacity={0.45} />
          <XAxis dataKey="date" tickLine={false} axisLine={false} tickMargin={8} fontSize={11} minTickGap={36} tickFormatter={(v: string) => fmtTick(v, lastYear)} />
          <YAxis tickLine={false} axisLine={false} tickMargin={8} fontSize={11} width={52} domain={["auto", "auto"]} tickFormatter={fmtUsdTick} />
          <ChartTooltip
            content={
              <ChartTooltipContent
                formatter={(value, name) => {
                  const label = name === "realized" ? "Realized" : name === "unrealized" ? "Unrealized" : "SPY buy & hold";
                  return `${label}: ${fmtUsdSigned(Number(value))}`;
                }}
              />
            }
          />
          {/* SPY: dashed reference ghost, no fill, rendered first so the
              account's own series sit above it (SVG z-order = JSX order). */}
          <Area type="monotone" dataKey="spy" stroke="var(--muted-foreground)" strokeWidth={1.5} strokeDasharray="4 4" fill="none" dot={false} connectNulls />
          {/* Unrealized: the open book, clay, no fill. */}
          <Area type="monotone" dataKey="unrealized" stroke="var(--claude)" strokeWidth={2} fill="none" dot={false} connectNulls />
          {/* Realized: booked P&L, on top with the zero-anchored wash. */}
          <Area type="monotone" dataKey="realized" stroke="var(--green)" strokeWidth={2.5} fill="url(#realizedFill)" dot={false} connectNulls />
        </AreaChart>
      </ChartContainer>
    </div>
  );
}
