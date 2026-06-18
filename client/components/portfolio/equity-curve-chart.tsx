"use client";

import { useMemo, useState } from "react";
import { Area, AreaChart, CartesianGrid, ReferenceLine, XAxis, YAxis } from "recharts";

import { AnimatedNumber, type AnimatedNumberKind } from "@/components/ui/animated-number";
import { type ChartConfig, ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { fmtMoneyInt } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { EquityCurvePoint } from "@/types/portfolio";

interface EquityCurveChartProps {
  points: EquityCurvePoint[];
  /**
  Live tick overrides for the curve's TODAY point (the server's synthetic
  live point refreshes per poll; these keep it current per streamed tick
  between polls). Only applied when the last point's date equals `today`,
  so a stale curve never gets yesterday's history falsified.
  */
  today?: string;
  liveEquity?: number | null;
  liveUnrealized?: number | null;
  liveSpyMark?: number | null;
}

/*
The dashboard chart has two views over the same daily series, switched by
the segmented control in the headline strip:

  value  the account's FULL value (positions + cash) at each close, the
         number the user actually has, against what the same starting
         equity would be worth riding buy-and-hold SPY. A faint dashed
         rule marks the window's peak so distance-from-peak reads off
         the plot directly.
  pnl    the dollar P&L decomposition over the window: cumulative
         realized round trips, the open book's EOD mark, and the SPY
         ghost on the same starting equity.

Series colors double as the legend in both views: the headline stats
above the plot carry matching color keys, so the chart needs no separate
legend block. The account line wears the clay accent the site uses for
the model's own voice; realized wears the semantic green; SPY stays the
muted dashed reference everywhere.
*/
type ChartView = "value" | "pnl";

const valueChartConfig: ChartConfig = {
  equity: { label: "Account value", color: "var(--claude)" },
  spyValue: { label: "SPY", color: "var(--muted-foreground)" },
};

const pnlChartConfig: ChartConfig = {
  realized: { label: "Realized", color: "var(--green)" },
  unrealized: { label: "Unrealized", color: "var(--claude)" },
  spy: { label: "SPY", color: "var(--muted-foreground)" },
};

function buildSeries(points: EquityCurvePoint[]) {
  // Anchor the SPY counterfactual to the first day BOTH the account and SPY
  // have a value, and base it on the account's equity THAT day. Picking the
  // first non-zero equity and the first non-zero SPY close independently can
  // resolve to DIFFERENT dates (e.g. early days where the EOD SPY fetch
  // failed), which silently rebases the ghost line to a later start than the
  // account line and falsifies the "Beating/Trailing SPY" verdict. Tying both
  // to one anchor day makes the ghost start level with the account on the
  // same date; days before that anchor gap out (we can't compare without SPY).
  const anchor = points.find((p) => p.account_equity > 0 && p.spy_close > 0);
  const baseEquity = anchor?.account_equity ?? 0;
  const baseSpy = anchor?.spy_close ?? 0;
  return points.map((p) => ({
    date: p.date,
    realized: p.realized_cum,
    unrealized: p.unrealized,
    // Hypothetical buy-and-hold: the anchor day's equity riding SPY, as P&L
    // (`spy`) for the decomposition view and as account value (`spyValue`)
    // for the value view.
    spy: baseEquity > 0 && baseSpy > 0 && p.spy_close > 0 ? baseEquity * (p.spy_close / baseSpy - 1) : null,
    spyValue: baseEquity > 0 && baseSpy > 0 && p.spy_close > 0 ? baseEquity * (p.spy_close / baseSpy) : null,
    // A zero-equity day is a missing snapshot, not a wiped account: null
    // it so the line gaps (connectNulls bridges it) instead of cratering.
    equity: p.account_equity > 0 ? p.account_equity : null,
  }));
}

function lastValue(series: Array<number | null>): number {
  for (let i = series.length - 1; i >= 0; i--) {
    const v = series[i];
    if (v != null) return v;
  }
  return 0;
}

// Signed compact dollars for the headline stats ("+$1,204", "-$86"). The sign
// is taken from the rounded magnitude, so a value that rounds to $0 reads as a
// neutral "$0" rather than a self-contradicting "-$0".
function fmtUsdSigned(n: number): string {
  const r = Math.round(n);
  if (r === 0) return "$0";
  const body = Math.abs(r).toLocaleString("en-US");
  return `${r < 0 ? "-" : "+"}$${body}`;
}

// signColor picks the P&L color for a headline stat, but stays neutral when
// the value rounds to $0 at the displayed (whole-dollar) precision — so a
// figure shown as "$0" is never tinted red or green as if it had direction.
function signColor(n: number): string {
  const r = Math.round(n);
  if (r < 0) return "text-red";
  if (r > 0) return "text-green";
  return "text-foreground";
}

// Axis ticks: compact unsigned dollars, negative as -$120.
function fmtUsdTick(n: number): string {
  const body = Math.abs(n).toLocaleString("en-US", { maximumFractionDigits: 0 });
  return `${n < 0 ? "-" : ""}$${body}`;
}

// Value-view axis ticks: account values run 5 figures, so $12,500 ticks
// crowd the 52px gutter. Compact to $12.5k past $10k; smaller magnitudes
// keep full dollars (P&L-sized numbers stay readable as-is).
function fmtUsdCompactTick(n: number): string {
  const abs = Math.abs(n);
  if (abs < 10_000) return fmtUsdTick(n);
  const k = abs / 1000;
  const body = k >= 100 ? Math.round(k).toLocaleString("en-US") : k.toFixed(1).replace(/\.0$/, "");
  return `${n < 0 ? "-" : ""}$${body}k`;
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
HeadlineStat is one figure in the strip above the plot. When it carries a
colorVar it doubles as a legend entry: the 2px key bar matches its series
stroke exactly, so the strip IS the legend (a stat with no series, like
Total return, simply renders without a key). Signed stats color the VALUE
by its sign like every other P&L figure: a negative realized number must
read red even when its series line is green, or the strip claims a win
the account didn't have. Always-positive levels (the account value, the
SPY counterfactual) opt out via signed={false} so they don't all glow
green. Values tween on live refresh (AnimatedNumber), so a poll tick
reads as the number moving rather than flickering.
*/
function HeadlineStat({
  label,
  value,
  kind = "usdSigned",
  signed = true,
  pctOfStart = null,
  sub,
  colorVar,
  dashed,
}: {
  label: string;
  value: number;
  kind?: AnimatedNumberKind;
  signed?: boolean;
  pctOfStart?: number | null;
  sub?: string;
  colorVar?: string;
  dashed?: boolean;
}) {
  return (
    <div>
      <div className="flex items-center gap-1.5">
        {colorVar && (
          <span
            aria-hidden
            className="h-0.5 w-4 rounded-full"
            style={dashed ? { backgroundImage: `repeating-linear-gradient(90deg, ${colorVar} 0 3px, transparent 3px 5px)` } : { backgroundColor: colorVar }}
          />
        )}
        <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</span>
      </div>
      <div className={cn("mt-0.5 text-2xl font-bold tabular-nums", signed ? signColor(value) : "text-foreground")}>
        <AnimatedNumber value={value} kind={kind} />
      </div>
      {pctOfStart !== null && (
        <div className="text-[11px] text-muted-foreground">
          <AnimatedNumber value={pctOfStart} kind="pct1" /> of starting equity
        </div>
      )}
      {sub && <div className="text-[11px] text-muted-foreground">{sub}</div>}
    </div>
  );
}

// EdgePill is the beating/trailing SPY verdict, shared by both views (the
// value view measures it level vs level, the P&L view sum vs ghost; the
// two agree by construction up to snapshot drift).
function EdgePill({ edge }: { edge: number }) {
  // Decide the verb from the SAME rounded magnitude the pill displays. An edge
  // that rounds to under a dollar (or a dead tie) reads "Even with SPY" rather
  // than asserting a "Beating/Trailing SPY by $0" win the account never had.
  const rounded = Math.round(edge);
  if (rounded === 0) {
    return <div className="mb-1 rounded-full border border-border bg-muted/40 px-2.5 py-1 text-xs font-semibold text-muted-foreground">Even with SPY</div>;
  }
  const beating = rounded > 0;
  return (
    <div className={cn("mb-1 rounded-full border px-2.5 py-1 text-xs font-semibold", beating ? "border-green-border bg-green-bg text-green" : "border-red-border bg-red-bg text-red")}>
      {beating ? "Beating SPY by " : "Trailing SPY by "}
      <AnimatedNumber value={Math.abs(edge)} kind="moneyInt" />
    </div>
  );
}

// ViewToggle is the segmented control that switches the plot. Inverted
// active cell (foreground on background) so the selected view reads at a
// glance; 36px minimum hit target on touch widths.
function ViewToggle({ view, onChange }: { view: ChartView; onChange: (v: ChartView) => void }) {
  const tabs: Array<{ id: ChartView; label: string }> = [
    { id: "value", label: "Account value" },
    { id: "pnl", label: "P&L vs SPY" },
  ];
  return (
    <div role="tablist" aria-label="Chart view" className="inline-flex border border-border">
      {tabs.map((t) => (
        <button
          key={t.id}
          type="button"
          role="tab"
          aria-selected={view === t.id}
          onClick={() => onChange(t.id)}
          className={cn(
            "min-h-9 px-2.5 py-1 font-mono text-[10px] font-bold uppercase tracking-wider transition-colors sm:min-h-0",
            view === t.id ? "bg-foreground text-background" : "text-muted-foreground hover:text-foreground",
          )}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

export function EquityCurveChart({ points, today, liveEquity = null, liveUnrealized = null, liveSpyMark = null }: EquityCurveChartProps) {
  const [view, setView] = useState<ChartView>("value");
  // Overlay the live tick values onto today's point before building the
  // series, so the headline stats AND the last chart segment move in real
  // time with the strip above them.
  const livePoints = useMemo(() => {
    if (points.length === 0) return points;
    const last = points[points.length - 1];
    if (!today || last.date !== today) return points;
    const next = { ...last };
    if (liveEquity !== null && liveEquity > 0) next.account_equity = liveEquity;
    if (liveUnrealized !== null) next.unrealized = liveUnrealized;
    if (liveSpyMark !== null && liveSpyMark > 0) next.spy_close = liveSpyMark;
    return [...points.slice(0, -1), next];
  }, [points, today, liveEquity, liveUnrealized, liveSpyMark]);
  const data = useMemo(() => buildSeries(livePoints), [livePoints]);

  if (data.length < 2) {
    return <div className="flex h-56 items-center justify-center text-sm text-muted-foreground">Not enough history yet to chart the curve. It fills in once the daily snapshot has run for a few sessions.</div>;
  }

  const firstEquity = livePoints.find((p) => p.account_equity > 0)?.account_equity ?? 0;
  const pctOfStart = (n: number) => (firstEquity > 0 ? (n / firstEquity) * 100 : null);
  const lastYear = data[data.length - 1].date.slice(0, 4);

  // P&L view headline figures.
  const realized = lastValue(data.map((d) => d.realized));
  const unrealized = lastValue(data.map((d) => d.unrealized));
  const spyPnl = lastValue(data.map((d) => d.spy));
  const pnlEdge = realized + unrealized - spyPnl;

  // Value view headline figures: levels, not P&L.
  const equityNow = lastValue(data.map((d) => d.equity));
  const spyValueNow = lastValue(data.map((d) => d.spyValue));
  const totalReturn = firstEquity > 0 ? equityNow - firstEquity : 0;
  const valueEdge = equityNow - spyValueNow;
  const peakEquity = data.reduce((m, d) => Math.max(m, d.equity ?? 0), 0);

  return (
    <div>
      {/* Headline strip: the color keys ARE the plot legend. */}
      <div className="mb-5 flex flex-wrap items-end gap-x-8 gap-y-3">
        {view === "value" ? (
          <>
            <HeadlineStat label="Account value" value={equityNow} kind="moneyInt" signed={false} sub={firstEquity > 0 ? `started at ${fmtMoneyInt(firstEquity)}` : undefined} colorVar="var(--claude)" />
            <HeadlineStat label="Total return" value={totalReturn} pctOfStart={pctOfStart(totalReturn)} />
            <HeadlineStat label="SPY buy & hold" value={spyValueNow} kind="moneyInt" signed={false} sub="starting equity riding SPY" colorVar="var(--muted-foreground)" dashed />
            {spyValueNow > 0 && <EdgePill edge={valueEdge} />}
          </>
        ) : (
          <>
            <HeadlineStat label="Realized" value={realized} pctOfStart={pctOfStart(realized)} colorVar="var(--green)" />
            <HeadlineStat label="Unrealized" value={unrealized} pctOfStart={pctOfStart(unrealized)} colorVar="var(--claude)" />
            <HeadlineStat label="SPY buy & hold" value={spyPnl} pctOfStart={pctOfStart(spyPnl)} colorVar="var(--muted-foreground)" dashed />
            <EdgePill edge={pnlEdge} />
          </>
        )}
        <div className="ml-auto self-start">
          <ViewToggle view={view} onChange={setView} />
        </div>
      </div>

      <ChartContainer config={view === "value" ? valueChartConfig : pnlChartConfig} className="h-64 w-full">
        <AreaChart data={data} margin={{ top: 8, right: 12, left: 4, bottom: 4 }}>
          <defs>
            <linearGradient id="realizedFill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--green)" stopOpacity={0.22} />
              <stop offset="92%" stopColor="var(--green)" stopOpacity={0} />
            </linearGradient>
            <linearGradient id="equityFill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--claude)" stopOpacity={0.18} />
              <stop offset="92%" stopColor="var(--claude)" stopOpacity={0} />
            </linearGradient>
          </defs>
          {/* --chart-grid is the token tuned per theme for in-plot reference
              lines; var(--border) at half opacity disappeared in light mode. */}
          <CartesianGrid vertical={false} stroke="var(--chart-grid)" />
          {/* P&L is signed, so the zero baseline is the anchor the eye
              measures everything against: slightly stronger than the grid.
              The value view's anchor is the window peak instead, fainter,
              so distance-from-peak reads straight off the plot. */}
          {view === "pnl" && <ReferenceLine y={0} stroke="var(--chart-text)" strokeOpacity={0.45} />}
          {view === "value" && peakEquity > 0 && <ReferenceLine y={peakEquity} stroke="var(--chart-text)" strokeOpacity={0.3} strokeDasharray="3 3" />}
          <XAxis dataKey="date" tickLine={false} axisLine={false} tickMargin={8} fontSize={11} minTickGap={36} tickFormatter={(v: string) => fmtTick(v, lastYear)} />
          <YAxis tickLine={false} axisLine={false} tickMargin={8} fontSize={11} width={52} domain={["auto", "auto"]} tickFormatter={view === "value" ? fmtUsdCompactTick : fmtUsdTick} />
          <ChartTooltip
            content={
              <ChartTooltipContent
                formatter={(value, name) => {
                  // Levels (the value view) read unsigned; P&L reads signed.
                  if (name === "equity") return `Account value: ${fmtUsdTick(Number(value))}`;
                  if (name === "spyValue") return `SPY buy & hold: ${fmtUsdTick(Number(value))}`;
                  const label = name === "realized" ? "Realized" : name === "unrealized" ? "Unrealized" : "SPY buy & hold";
                  return `${label}: ${fmtUsdSigned(Number(value))}`;
                }}
              />
            }
          />
          {/* SPY ghosts: dashed reference, no fill, rendered first so the
              account's own series sit above it (SVG z-order = JSX order). */}
          {view === "value" && <Area type="monotone" dataKey="spyValue" stroke="var(--muted-foreground)" strokeWidth={1.5} strokeDasharray="4 4" fill="none" dot={false} connectNulls />}
          {/* The account's full value, clay, with the zero-anchored wash. */}
          {view === "value" && <Area type="monotone" dataKey="equity" stroke="var(--claude)" strokeWidth={2.5} fill="url(#equityFill)" dot={false} connectNulls />}
          {view === "pnl" && <Area type="monotone" dataKey="spy" stroke="var(--muted-foreground)" strokeWidth={1.5} strokeDasharray="4 4" fill="none" dot={false} connectNulls />}
          {/* Unrealized: the open book, clay, no fill. */}
          {view === "pnl" && <Area type="monotone" dataKey="unrealized" stroke="var(--claude)" strokeWidth={2} fill="none" dot={false} connectNulls />}
          {/* Realized: booked P&L, on top with the zero-anchored wash. */}
          {view === "pnl" && <Area type="monotone" dataKey="realized" stroke="var(--green)" strokeWidth={2.5} fill="url(#realizedFill)" dot={false} connectNulls />}
        </AreaChart>
      </ChartContainer>

      {/* The legend explainer reads better AFTER the numbers: leading
          with prose pushed the actual figures below the fold on mobile. */}
      <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
        {view === "value"
          ? "The account's full value (positions plus cash) at each daily close. The dashed line is what the same starting equity would be worth in buy-and-hold SPY, and the faint rule marks the window's peak."
          : "Realized is booked round trips, unrealized is the open book's mark, and the dashed line is what buy-and-hold SPY would have earned on the same starting equity."}
      </p>
    </div>
  );
}
