"use client";

import { useEffect, useState } from "react";
import { Area, Bar, CartesianGrid, ComposedChart, Line, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { api } from "@/lib/api";
import type { ChartParams } from "@/types/trade";

interface TradeInfo {
  symbol: string;
  contract_type: string;
  strike_price: number;
  expiration: string;
  current_price: number;
  estimated_price: number;
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
  optionMark: number | null;
}

// estimateDelta picks a sensible Black-Scholes-ish delta for the contract
// based on its moneyness so we can model how the premium tracks the
// underlying without needing a real Greeks feed. Sign convention: positive
// delta on calls, negative on puts.
function estimateDelta(contractType: string, strike: number, underlying: number): number {
  if (underlying <= 0 || strike <= 0) return 0.5;
  const moneyness = (underlying - strike) / strike; // positive = ITM for calls
  if (contractType === "PUT") {
    // -0.7 deep ITM put (underlying << strike), -0.3 deep OTM put
    if (moneyness <= -0.05) return -0.7;
    if (moneyness >= 0.05) return -0.3;
    return -0.5;
  }
  if (moneyness >= 0.05) return 0.7;
  if (moneyness <= -0.05) return 0.3;
  return 0.5;
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

        // Approximate the option premium series from the underlying using
        // a sticky delta — Schwab doesn't expose intraday option history,
        // so this is the cleanest way to plot a "contract price over time"
        // line alongside the stock candles. When a summary exists we
        // anchor entry/exit to the recorded prices and interpolate in
        // between with the same model.
        const trd = trade;
        const entryUnderlying = summary?.stock_open ?? trd?.current_price ?? 0;
        const entryPremium = summary?.entry_price ?? trd?.estimated_price ?? 0;
        const exitUnderlying = summary?.stock_close;
        const exitPremium = summary?.closing_price;
        const delta = trd ? estimateDelta(trd.contract_type, trd.strike_price, entryUnderlying) : 0.5;

        // When we have both endpoints, blend the linear-delta estimate with
        // the recorded entry/exit so the line lands exactly on those marks
        // and tracks the underlying in between.
        let blendNum = 0;
        let blendDen = 0;
        if (entryUnderlying > 0 && entryPremium > 0) {
          blendNum += entryPremium - entryPremium; // anchor at 0 by definition
          blendDen += 1;
        }
        if (exitUnderlying !== undefined && exitPremium !== undefined && entryUnderlying > 0) {
          const modeled = entryPremium + delta * (exitUnderlying - entryUnderlying);
          if (modeled !== 0) {
            blendNum += exitPremium / modeled - 1;
            blendDen += 1;
          }
        }
        const correction = blendDen > 0 ? 1 + blendNum / blendDen : 1;

        const points: DataPoint[] = res.candles.map((c) => {
          let optionMark: number | null = null;
          if (trd && entryPremium > 0 && entryUnderlying > 0) {
            const raw = entryPremium + delta * (c.close - entryUnderlying);
            optionMark = Math.max(0.01, raw * correction);
          }
          return {
            time: c.time,
            label: formatTime(c.time),
            close: c.close,
            open: c.open,
            high: c.high,
            low: c.low,
            volume: c.volume,
            optionMark,
          };
        });

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
  }, [symbol, timeframe, trade, summary]);

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

  // First/last candle of the most recent trading day. BUY anchors to the
  // entry candle whenever a trade exists (live or settled); SELL only
  // shows once the trade has an EOD summary.
  let buyTime: number | undefined;
  let sellTime: number | undefined;
  if (trade) {
    const recentDate = formatDate(data[data.length - 1].time);
    const dayCandles = data.filter((d) => formatDate(d.time) === recentDate);
    if (dayCandles.length > 0) {
      buyTime = dayCandles[0].time;
      if (summary) sellTime = dayCandles[dayCandles.length - 1].time;
    }
  }

  // Premium-axis bounds — option marks are an order of magnitude smaller
  // than the underlying, so the line gets its own right-side scale.
  const optionMarks = data.map((d) => d.optionMark).filter((v): v is number => v !== null);
  const hasOptionLine = optionMarks.length > 0;
  let oMin = 0;
  let oMax = 1;
  if (hasOptionLine) {
    oMin = Math.min(...optionMarks);
    oMax = Math.max(...optionMarks);
    const oPad = Math.max(0.05, (oMax - oMin) * 0.15);
    oMin = Math.max(0, oMin - oPad);
    oMax = oMax + oPad;
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
      {hasOptionLine && (
        <div className="flex flex-wrap items-center gap-x-3 px-3 pb-1 text-[10px] text-muted-foreground sm:px-4">
          <span className="inline-flex items-center gap-1.5">
            <span className="inline-block h-0.5 w-3 rounded" style={{ background: strokeColor }} /> Stock
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="inline-block h-0.5 w-3 rounded" style={{ background: "var(--amber)", borderTop: "1px dashed var(--amber)" }} /> Contract{" "}
            <span className="italic opacity-70">(modeled from delta &middot; anchored to entry / exit)</span>
          </span>
        </div>
      )}
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

            {hasOptionLine && (
              <YAxis
                yAxisId="option"
                orientation="right"
                stroke="var(--amber)"
                tick={{ fontSize: 10, fill: "var(--amber)" }}
                tickLine={false}
                axisLine={false}
                domain={[oMin, oMax]}
                tickFormatter={(v) => `$${v.toFixed(2)}`}
                width={55}
              />
            )}

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
                      {d.optionMark !== null && (
                        <>
                          <span style={{ color: "var(--amber)" }}>Contract</span>
                          <span className="tabular-nums font-medium" style={{ color: "var(--amber)" }}>
                            ${d.optionMark.toFixed(2)}
                          </span>
                        </>
                      )}
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
              name="Stock"
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

            {/* Option premium overlay (estimated from underlying movement
                via a sticky delta — Schwab doesn't expose intraday option
                history, so this is the closest approximation we can plot). */}
            {hasOptionLine && (
              <Line
                yAxisId="option"
                type="monotone"
                dataKey="optionMark"
                name="Contract"
                stroke="var(--amber)"
                strokeWidth={1.5}
                strokeDasharray="4 2"
                dot={false}
                isAnimationActive={false}
                connectNulls
              />
            )}

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
            {buyTime !== undefined && (
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

            {sellTime !== undefined && (
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
