"use client";

import { useEffect, useMemo, useState } from "react";
import { Area, AreaChart, CartesianGrid, ReferenceLine, XAxis, YAxis } from "recharts";

import { type ChartConfig, ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { api } from "@/lib/api";
import { fmtPrice } from "@/lib/format";
import type { PositionValuePoint, PricePoint } from "@/types/portfolio";

/*
TradeHistoryChart draws a single trade's life so far (or its whole life,
for a closed trade): for equities, the underlying's price over the holding
period; for options, two series on paired axes: the underlying's price on
the left and the position's per-contract value on the right, with the
strike as a dashed reference on the price axis.

Sources, in trust order: the underlying line prefers live market-data
closes and falls back to the per-share value implied by the EOD book
snapshots; the option-value line comes from the snapshots alone (there is
no public history for an OCC symbol). Either series may be empty: a trade
opened this morning has no snapshot yet, and the market-data source can be
unreachable: the chart renders whatever exists and says so instead of
faking a flat line.

Duration handling: the window is open date through close date (or today).
Daily points serve every span; the x-axis abbreviates ticks and lets
minTickGap thin a year down, while a days-old trade simply shows its few
points with visible dots so two datapoints read as data, not as a line
pretending to be continuous.
*/

interface TradeHistoryChartProps {
  kind: "stock" | "option";
  /** Broker symbol: ticker, or the OCC option symbol. */
  symbol: string;
  underlying: string;
  openedDate?: string;
  closedDate?: string;
  strike?: number;
  /** Per-share avg cost (equity) or per-contract avg premium (option). */
  entryPrice?: number;
}

const TICK_MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

function fmtTick(iso: string, lastYear: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!m) return iso;
  const month = TICK_MONTHS[Number(m[2]) - 1] ?? m[2];
  const base = `${month} ${Number(m[3])}`;
  return m[1] === lastYear ? base : `${base} '${m[1].slice(2)}`;
}

// Axis dollars adapt their precision to the magnitude: a $4.20 to $5.10
// option premium needs cents or every tick collapses to "$5", while a
// $200 stock reads cleanest in whole dollars.
function fmtUsdTick(n: number): string {
  const abs = Math.abs(n);
  const digits = abs >= 100 ? 0 : abs >= 10 ? 1 : 2;
  const body = abs.toLocaleString("en-US", { minimumFractionDigits: digits, maximumFractionDigits: digits });
  return `${n < 0 ? "-" : ""}$${body}`;
}

type Row = { date: string; price: number | null; value: number | null };

/*
mergeSeries outer-joins the two day-keyed series so Recharts gets one
data array; either side may be missing on any given day (weekends exist
only in snapshots, snapshot gaps exist only in closes) and connectNulls
bridges the gaps.
*/
function mergeSeries(prices: PricePoint[], snapshots: PositionValuePoint[], perContract: boolean): Row[] {
  const byDate = new Map<string, Row>();
  for (const p of prices) {
    byDate.set(p.date, { date: p.date, price: p.close, value: null });
  }
  for (const s of snapshots) {
    const row = byDate.get(s.date) ?? { date: s.date, price: null, value: null };
    row.value = s.quantity > 0 ? s.market_value / (perContract ? s.quantity * 100 : s.quantity) : null;
    byDate.set(s.date, row);
  }
  return [...byDate.values()].sort((a, b) => (a.date < b.date ? -1 : 1));
}

// First/last non-null reading of a series, for the win/loss line color.
function endpoint(data: Row[], key: "price" | "value", fromEnd: boolean): number | null {
  const idx = fromEnd ? [...data].reverse() : data;
  for (const row of idx) {
    const v = row[key];
    if (v != null) return v;
  }
  return null;
}

export function TradeHistoryChart({ kind, symbol, underlying, openedDate, closedDate, strike, entryPrice }: TradeHistoryChartProps) {
  const [prices, setPrices] = useState<PricePoint[] | null>(null);
  const [snapshots, setSnapshots] = useState<PositionValuePoint[] | null>(null);

  useEffect(() => {
    let alive = true;
    api
      .getPriceHistory(underlying, openedDate, closedDate)
      .then((r) => alive && setPrices(r.points ?? []))
      .catch(() => alive && setPrices([]));
    api
      .getPositionHistory(symbol)
      .then((r) => alive && setSnapshots(r.points ?? []))
      .catch(() => alive && setSnapshots([]));
    return () => {
      alive = false;
    };
  }, [underlying, symbol, openedDate, closedDate]);

  const isOption = kind === "option";
  const data = useMemo(() => {
    if (prices === null || snapshots === null) return null;
    // Clamp both series to THIS trade's window: position history is keyed
    // by symbol, and a ticker the model has traded more than once carries
    // every round trip's snapshots, which would stretch a 20-day hold's
    // chart across months of unrelated history.
    const within = (d: string) => (!openedDate || d >= openedDate) && (!closedDate || d <= closedDate);
    const ps = prices.filter((p) => within(p.date));
    const ss = snapshots.filter((s) => within(s.date));
    // Equity: the stock IS the position, so one line. Prefer market closes;
    // fall back to the snapshot-implied per-share value.
    if (!isOption) {
      if (ps.length > 0) return ps.map((p) => ({ date: p.date, price: p.close, value: null }) as Row);
      return mergeSeries([], ss, false).map((r) => ({ ...r, price: r.value, value: null }));
    }
    return mergeSeries(ps, ss, true);
  }, [prices, snapshots, isOption, openedDate, closedDate]);

  if (data === null) {
    return <div className="h-60 w-full animate-pulse rounded-md bg-muted/40" aria-busy="true" aria-label="Loading trade history" />;
  }

  const havePrice = data.some((d) => d.price != null);
  const haveValue = isOption && data.some((d) => d.value != null);
  const pointCount = data.filter((d) => d.price != null || d.value != null).length;

  if (pointCount === 0) {
    return (
      <p className="py-4 text-sm text-muted-foreground">
        {closedDate
          ? "No price series is available for this trade right now: market data is unreachable and no daily book snapshots cover the holding period."
          : "No price history yet. A position opened today charts after its first end-of-day snapshot."}
      </p>
    );
  }

  // A handful of points reads as dots-with-a-line; a long window reads as
  // a continuous line with thinned ticks.
  const sparse = pointCount <= 12;
  const lastYear = data[data.length - 1].date.slice(0, 4);

  // The POSITION line is win/loss colored, like every other P&L surface:
  // green when the latest mark sits at or above the entry, red below it.
  // Entry prefers the recorded avg cost / premium; a missing entry falls
  // back to the series' own first reading so the line still takes a side.
  const posKey = isOption ? "value" : "price";
  const lastMark = endpoint(data, posKey, true);
  const baseline = entryPrice != null && entryPrice > 0 ? entryPrice : endpoint(data, posKey, false);
  const posColor = lastMark != null && baseline != null && lastMark < baseline ? "var(--red)" : "var(--green)";
  const chartConfig: ChartConfig = {
    price: { label: "Underlying", color: isOption ? "var(--muted-foreground)" : posColor },
    value: { label: "Position", color: posColor },
  };

  return (
    <div>
      <ChartContainer config={chartConfig} className="h-60 w-full">
        <AreaChart data={data} margin={{ top: 8, right: isOption ? 4 : 12, left: 4, bottom: 4 }}>
          <CartesianGrid vertical={false} stroke="var(--chart-grid)" xAxisId="x" yAxisId="price" />
          <XAxis xAxisId="x" dataKey="date" tickLine={false} axisLine={false} tickMargin={8} fontSize={11} minTickGap={36} tickFormatter={(v: string) => fmtTick(v, lastYear)} />
          {/* yAxisId names are alphabetical on purpose: Recharts v3 renders
              multiple y-axes in alphabetical id order, so "price" (left)
              paints before "value" (right). */}
          <YAxis yAxisId="price" orientation="left" tickLine={false} axisLine={false} tickMargin={8} fontSize={11} width={52} domain={["auto", "auto"]} tickFormatter={fmtUsdTick} hide={!havePrice} />
          {isOption && <YAxis yAxisId="value" orientation="right" tickLine={false} axisLine={false} tickMargin={8} fontSize={11} width={52} domain={["auto", "auto"]} tickFormatter={fmtUsdTick} hide={!haveValue} />}
          {strike != null && strike > 0 && havePrice && (
            <ReferenceLine xAxisId="x" yAxisId="price" y={strike} stroke="var(--chart-text)" strokeOpacity={0.5} strokeDasharray="3 3" label={{ value: `strike ${fmtPrice(strike)}`, position: "insideTopRight", fontSize: 10, fill: "var(--chart-text)" }} />
          )}
          {!isOption && entryPrice != null && entryPrice > 0 && (
            <ReferenceLine xAxisId="x" yAxisId="price" y={entryPrice} stroke="var(--chart-text)" strokeOpacity={0.5} strokeDasharray="3 3" label={{ value: `entry ${fmtPrice(entryPrice)}`, position: "insideTopRight", fontSize: 10, fill: "var(--chart-text)" }} />
          )}
          <ChartTooltip
            content={
              <ChartTooltipContent
                formatter={(value, name) => {
                  const label = name === "price" ? (isOption ? `${underlying} close` : underlying) : "Per-contract value";
                  return `${label}: ${fmtPrice(Number(value))}`;
                }}
              />
            }
          />
          {havePrice && (
            <Area xAxisId="x" yAxisId="price" type="monotone" dataKey="price" stroke={isOption ? "var(--muted-foreground)" : posColor} strokeWidth={isOption ? 1.5 : 2.5} fill="none" dot={sparse} connectNulls />
          )}
          {haveValue && <Area xAxisId="x" yAxisId="value" type="monotone" dataKey="value" stroke={posColor} strokeWidth={2.5} fill="none" dot={sparse} connectNulls />}
        </AreaChart>
      </ChartContainer>
      {isOption && (
        <p className="mt-2 text-[11px] text-muted-foreground">
          <span className="font-semibold" style={{ color: posColor }}>
            Per-contract value
          </span>{" "}
          (right) from the daily book snapshots ·{" "}
          <span className="font-semibold">{underlying} close</span> (left)
          {havePrice ? "" : " unavailable right now"}
          {strike != null && strike > 0 ? " · dashed line marks the strike" : ""}
        </p>
      )}
    </div>
  );
}
