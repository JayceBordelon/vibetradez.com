"use client";

import { useMemo } from "react";
import { Bar, BarChart, XAxis, YAxis } from "recharts";
import { type ChartConfig, ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { computeTradePnl } from "@/lib/calculations";
import { fmtPnlInt } from "@/lib/format";
import type { DashboardTrade, Execution } from "@/types/trade";

interface PnlChartProps {
  trades: DashboardTrade[];
  executions?: Execution[] | null;
}

const chartConfig: ChartConfig = {
  pnl: {
    label: "P&L",
  },
};

export function PnlChart({ trades, executions }: PnlChartProps) {
  const data = useMemo(() => {
    return trades
      .map((dt) => {
        const result = computeTradePnl(dt, executions);
        if (!result.hasData) return null;
        return {
          name: `$${dt.trade.symbol} ${dt.trade.contract_type}`,
          pnl: result.pnl,
          fill: result.pnl >= 0 ? "var(--green)" : "var(--red)",
        };
      })
      .filter((d): d is { name: string; pnl: number; fill: string } => d !== null)
      .sort((a, b) => b.pnl - a.pnl);
  }, [trades, executions]);

  if (data.length === 0) {
    return <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">No closed trades to chart</div>;
  }

  const chartHeight = Math.max(200, data.length * 40);

  return (
    <div>
      <ChartContainer config={chartConfig} className="w-full" style={{ height: chartHeight }}>
        <BarChart data={data} layout="vertical" margin={{ top: 5, right: 30, left: 10, bottom: 5 }}>
          {/*
            Force the number axis to always include 0. Bars draw from a
            baseline of 0, but Recharts' default auto-domain is
            [dataMin, dataMax], which drops 0 when every bar shares a
            sign (e.g. an all-losses day). With 0 off-axis the
            least-extreme bar collapses to zero width and appears to
            vanish. Anchoring the domain to 0 on the data side keeps
            every bar measured from the same baseline.
          */}
          <XAxis
            type="number"
            domain={[(dataMin: number) => Math.min(0, dataMin), (dataMax: number) => Math.max(0, dataMax)]}
            tickFormatter={(v: number) => fmtPnlInt(v)}
            fontSize={11}
          />
          <YAxis type="category" dataKey="name" width={120} fontSize={11} tickLine={false} />
          <ChartTooltip content={<ChartTooltipContent formatter={(value) => fmtPnlInt(Number(value))} hideIndicator />} />
          {/*
            Per-bar color comes from each datum's `fill` field
            Recharts v3 reads it directly on the Bar, no need for
            <Cell> children (deprecated as of recharts 3.7 per
            CLAUDE.md, removed in v4).
          */}
          <Bar dataKey="pnl" radius={[0, 4, 4, 0]} barSize={24} />
        </BarChart>
      </ChartContainer>
    </div>
  );
}
