"use client";

import { useMemo } from "react";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";

import { type ChartConfig, ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import type { EquityCurvePoint } from "@/types/portfolio";

interface EquityCurveChartProps {
  points: EquityCurvePoint[];
  /**
  Open-book unrealized return percent (live, not yet booked into the EOD
  equity curve). Rendered as a third headline stat beside Account and SPY;
  omitted when null (no open positions).
  */
  unrealizedPct?: number | null;
}

const chartConfig: ChartConfig = {
  account: { label: "Account", color: "var(--green)" },
  spy: { label: "SPY", color: "var(--muted-foreground)" },
};

/*
Both series are indexed to 100 at the first point so the account's return is
comparable to buy-and-hold SPY on a single axis. Days with a missing SPY
close (0) are skipped so a gap doesn't crater the line.
*/
function buildSeries(points: EquityCurvePoint[]) {
  const firstEquity = points.find((p) => p.account_equity > 0)?.account_equity ?? 0;
  const firstSpy = points.find((p) => p.spy_close > 0)?.spy_close ?? 0;
  return points.map((p) => ({
    date: p.date,
    account: firstEquity > 0 && p.account_equity > 0 ? (p.account_equity / firstEquity) * 100 : null,
    spy: firstSpy > 0 && p.spy_close > 0 ? (p.spy_close / firstSpy) * 100 : null,
  }));
}

function lastReturn(series: Array<number | null>): number {
  for (let i = series.length - 1; i >= 0; i--) {
    const v = series[i];
    if (v != null) return v - 100;
  }
  return 0;
}

function fmtPctSigned(n: number): string {
  return `${n >= 0 ? "+" : ""}${n.toFixed(1)}%`;
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

export function EquityCurveChart({ points, unrealizedPct = null }: EquityCurveChartProps) {
  const data = useMemo(() => buildSeries(points), [points]);

  if (data.length < 2) {
    return <div className="flex h-56 items-center justify-center text-sm text-muted-foreground">Not enough history yet to chart the curve. It fills in once the daily snapshot has run for a few sessions.</div>;
  }

  const acctRet = lastReturn(data.map((d) => d.account));
  const spyRet = lastReturn(data.map((d) => d.spy));
  const edge = acctRet - spyRet;
  const acctColor = acctRet >= 0 ? "var(--green)" : "var(--red)";
  const lastYear = data[data.length - 1].date.slice(0, 4);

  return (
    <div>
      {/* Returns header: account vs SPY, colored */}
      <div className="mb-4 flex flex-wrap items-end gap-x-6 gap-y-2">
        <div>
          <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Account</div>
          <div className="text-2xl font-bold tabular-nums" style={{ color: acctColor }}>
            {fmtPctSigned(acctRet)}
          </div>
        </div>
        <div>
          <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">SPY</div>
          <div className="text-2xl font-bold tabular-nums text-muted-foreground">{fmtPctSigned(spyRet)}</div>
        </div>
        {unrealizedPct !== null && (
          <div>
            <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Unrealized</div>
            <div className="text-2xl font-bold tabular-nums" style={{ color: unrealizedPct >= 0 ? "var(--green)" : "var(--red)" }}>
              {fmtPctSigned(unrealizedPct)}
            </div>
          </div>
        )}
        <div
          className={`mb-1 rounded-full border px-2.5 py-1 text-xs font-semibold ${edge >= 0 ? "border-green-border bg-green-bg text-green" : "border-red-border bg-red-bg text-red"}`}
        >
          {edge >= 0 ? "Beating SPY by " : "Trailing SPY by "}
          {Math.abs(edge).toFixed(1)} pts
        </div>
      </div>

      <ChartContainer config={chartConfig} className="h-64 w-full">
        <AreaChart data={data} margin={{ top: 8, right: 12, left: 4, bottom: 4 }}>
          <defs>
            <linearGradient id="acctFill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={acctColor} stopOpacity={0.28} />
              <stop offset="92%" stopColor={acctColor} stopOpacity={0} />
            </linearGradient>
          </defs>
          {/* --chart-grid is the token tuned per theme for in-plot reference
              lines; var(--border) at half opacity disappeared in light mode. */}
          <CartesianGrid vertical={false} stroke="var(--chart-grid)" />
          <XAxis dataKey="date" tickLine={false} axisLine={false} tickMargin={8} fontSize={11} minTickGap={36} tickFormatter={(v: string) => fmtTick(v, lastYear)} />
          <YAxis tickLine={false} axisLine={false} tickMargin={8} fontSize={11} width={40} domain={["auto", "auto"]} tickFormatter={(v: number) => v.toFixed(0)} />
          <ChartTooltip content={<ChartTooltipContent formatter={(value, name) => `${name === "account" ? "Account" : "SPY"}: ${Number(value).toFixed(1)}`} />} />
          {/* SPY: dashed reference line, no fill */}
          <Area type="monotone" dataKey="spy" stroke="var(--muted-foreground)" strokeWidth={1.5} strokeDasharray="4 4" fill="none" dot={false} connectNulls />
          {/* Account: gradient-filled, on top */}
          <Area type="monotone" dataKey="account" stroke={acctColor} strokeWidth={2.5} fill="url(#acctFill)" dot={false} connectNulls />
        </AreaChart>
      </ChartContainer>
    </div>
  );
}
