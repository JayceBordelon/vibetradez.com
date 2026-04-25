"use client";

import { useEffect, useState } from "react";
import { Area, Bar, CartesianGrid, ComposedChart, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { api } from "@/lib/api";
import type { ChartParams } from "@/types/trade";

interface TradeInfo {
  symbol: string;
  contract_type: string;
  strike_price: number;
  expiration: string;
  current_price: number;
}

interface SummaryInfo {
  entry_price: number;
  closing_price: number;
  stock_open: number;
  stock_close: number;
}

interface StockChartProps {
  symbol: string;
  timeframe: ChartParams;
  strikePrice?: number;
  trade?: TradeInfo;
  summary?: SummaryInfo;
}

interface DataPoint {
  time: number;
  label: string;
  close: number;
  open: number;
  high: number;
  low: number;
  volume: number;
}

function formatTime(epoch: number): string {
  const d = new Date(epoch * 1000);
  const h = d.getHours();
  const m = d.getMinutes();
  const ampm = h >= 12 ? "PM" : "AM";
  return `${h % 12 || 12}:${String(m).padStart(2, "0")} ${ampm}`;
}

function formatDate(epoch: number): string {
  const d = new Date(epoch * 1000);
  return `${d.getMonth() + 1}/${d.getDate()}`;
}

export function StockChart({ symbol, timeframe, strikePrice, trade, summary }: StockChartProps) {
  const [data, setData] = useState<DataPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    api
      .getChartData(symbol, timeframe)
      .then((res) => {
        if (cancelled) return;
        if (!res?.candles?.length) {
          setError("No chart data available");
          setLoading(false);
          return;
        }

        const points: DataPoint[] = res.candles.map((c) => ({
          time: c.time,
          label: formatTime(c.time),
          close: c.close,
          open: c.open,
          high: c.high,
          low: c.low,
          volume: c.volume,
        }));

        setData(points);
        setLoading(false);
      })
      .catch(() => {
        if (!cancelled) {
          setError("Chart unavailable");
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [symbol, timeframe]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <span className="text-sm text-muted-foreground">Loading chart...</span>
      </div>
    );
  }

  if (error || !data.length) {
    return (
      <div className="flex h-full items-center justify-center">
        <span className="text-sm text-muted-foreground">{error || "No data"}</span>
      </div>
    );
  }

  const first = data[0].close;
  const last = data[data.length - 1].close;
  const isUp = last >= first;
  const strokeColor = isUp ? "var(--gpt)" : "var(--red)";
  const fillId = `area-fill-${symbol}`;

  // Compute Y-axis domain with padding
  let min = Number.POSITIVE_INFINITY;
  let max = Number.NEGATIVE_INFINITY;
  for (const p of data) {
    if (p.low < min) min = p.low;
    if (p.high > max) max = p.high;
  }
  if (strikePrice !== undefined) {
    if (strikePrice < min) min = strikePrice;
    if (strikePrice > max) max = strikePrice;
  }
  const pad = (max - min) * 0.08;
  const yMin = Math.floor((min - pad) * 100) / 100;
  const yMax = Math.ceil((max + pad) * 100) / 100;

  // Find max volume for scaling
  const maxVol = Math.max(...data.map((d) => d.volume));

  // Show date labels only when data spans multiple days
  const firstDate = formatDate(data[0].time);
  const lastDate = formatDate(data[data.length - 1].time);
  const multiDay = firstDate !== lastDate;

  // Find the first and last candle of the most recent trading day for
  // buy/sell markers. The "buy" is at open, "sell" is near close.
  let buyTime: number | undefined;
  let sellTime: number | undefined;
  if (summary) {
    const lastDate = formatDate(data[data.length - 1].time);
    const dayCandles = data.filter((d) => formatDate(d.time) === lastDate);
    if (dayCandles.length > 0) {
      buyTime = dayCandles[0].time;
      sellTime = dayCandles[dayCandles.length - 1].time;
    }
  }

  // Show only ~5 tick labels so they never overlap, even on small screens.
  const tickInterval = Math.max(1, Math.floor(data.length / 5));

  // Format change from first to last candle
  const change = last - first;
  const changePct = first > 0 ? (change / first) * 100 : 0;
  const changeSign = change >= 0 ? "+" : "";

  return (
    <div className="flex h-full w-full flex-col">
      {/* Chart header */}
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5 px-3 pt-2.5 pb-1 sm:px-4">
        <span className="font-mono text-sm font-bold text-foreground sm:text-base">${symbol}</span>
        {trade && (
          <span className="text-xs text-muted-foreground">
            {trade.contract_type} ${trade.strike_price} · Exp {trade.expiration}
          </span>
        )}
        <span className="ml-auto font-mono text-sm font-semibold tabular-nums" style={{ color: isUp ? "var(--gpt)" : "var(--red)" }}>
          ${last.toFixed(2)}{" "}
          <span className="text-xs">
            {changeSign}
            {change.toFixed(2)} ({changeSign}
            {changePct.toFixed(2)}%)
          </span>
        </span>
      </div>
      <div className="min-h-0 flex-1">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
            <defs>
              <linearGradient id={fillId} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={strokeColor} stopOpacity={0.15} />
                <stop offset="100%" stopColor={strokeColor} stopOpacity={0.01} />
              </linearGradient>
            </defs>

            <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" vertical={false} />

            <XAxis
              dataKey="time"
              stroke="var(--chart-text)"
              tick={{ fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              interval={tickInterval}
              tickFormatter={(t) => {
                if (multiDay) return formatDate(t);
                return formatTime(t);
              }}
            />

            <YAxis yAxisId="price" stroke="var(--chart-text)" tick={{ fontSize: 10 }} tickLine={false} axisLine={false} domain={[yMin, yMax]} tickFormatter={(v) => `$${v.toFixed(2)}`} width={65} />

            <YAxis yAxisId="volume" orientation="right" hide domain={[0, maxVol * 5]} />

            <Tooltip
              content={({ active, payload }) => {
                if (!active || !payload?.length) return null;
                const d = payload[0].payload as DataPoint;
                return (
                  <div className="rounded-lg border bg-card px-3 py-2 text-xs shadow-md">
                    <div className="mb-1 font-medium text-muted-foreground">{multiDay ? `${formatDate(d.time)} ${formatTime(d.time)}` : formatTime(d.time)}</div>
                    <div className="grid grid-cols-2 gap-x-4 gap-y-0.5">
                      <span className="text-muted-foreground">O</span>
                      <span className="tabular-nums font-medium text-foreground">${d.open.toFixed(2)}</span>
                      <span className="text-muted-foreground">H</span>
                      <span className="tabular-nums font-medium text-foreground">${d.high.toFixed(2)}</span>
                      <span className="text-muted-foreground">L</span>
                      <span className="tabular-nums font-medium text-foreground">${d.low.toFixed(2)}</span>
                      <span className="text-muted-foreground">C</span>
                      <span className="tabular-nums font-medium text-foreground">${d.close.toFixed(2)}</span>
                      <span className="text-muted-foreground">Vol</span>
                      <span className="tabular-nums font-medium text-foreground">
                        {d.volume >= 1_000_000 ? `${(d.volume / 1_000_000).toFixed(1)}M` : d.volume >= 1_000 ? `${(d.volume / 1_000).toFixed(0)}K` : d.volume.toLocaleString()}
                      </span>
                    </div>
                  </div>
                );
              }}
            />

            {/* Volume bars */}
            <Bar yAxisId="volume" dataKey="volume" fill="var(--muted-foreground)" fillOpacity={0.08} isAnimationActive={false} />

            {/* Price area */}
            <Area
              yAxisId="price"
              type="monotone"
              dataKey="close"
              stroke={strokeColor}
              strokeWidth={1.5}
              fill={`url(#${fillId})`}
              isAnimationActive={false}
              dot={false}
              activeDot={{
                r: 3,
                stroke: strokeColor,
                strokeWidth: 2,
                fill: "var(--card)",
              }}
            />

            {/* Strike price reference line */}
            {strikePrice !== undefined && (
              <ReferenceLine
                yAxisId="price"
                y={strikePrice}
                stroke="var(--claude)"
                strokeDasharray="6 3"
                strokeWidth={1}
                label={{
                  value: `Strike $${strikePrice}`,
                  position: "insideTopRight",
                  fill: "var(--claude)",
                  fontSize: 10,
                  fontWeight: 600,
                  offset: 4,
                }}
              />
            )}

            {/* Buy/sell drawn as full-height vertical reference lines so the
                entry/exit moments are unambiguous on the chart, instead of
                tiny dots that floated above the price line. Strokes use the
                model brand colors and the labels stay anchored at the top. */}
            {summary && buyTime !== undefined && (
              <ReferenceLine
                yAxisId="price"
                x={buyTime}
                stroke="var(--gpt)"
                strokeWidth={2}
                strokeDasharray="3 3"
                label={{
                  value: "BUY",
                  position: "insideTopLeft",
                  fill: "var(--gpt)",
                  fontSize: 9,
                  fontWeight: 700,
                  offset: 6,
                }}
              />
            )}

            {summary && sellTime !== undefined && (
              <ReferenceLine
                yAxisId="price"
                x={sellTime}
                stroke="var(--red)"
                strokeWidth={2}
                strokeDasharray="3 3"
                label={{
                  value: "SELL",
                  position: "insideTopRight",
                  fill: "var(--red)",
                  fontSize: 9,
                  fontWeight: 700,
                  offset: 6,
                }}
              />
            )}
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
