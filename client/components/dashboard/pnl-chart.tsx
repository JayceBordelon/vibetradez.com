"use client";

import { useMemo } from "react";
import { Bar, BarChart, Cell, XAxis, YAxis } from "recharts";
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
          fill: result.pnl >= 0 ? "#10b981" : "#ef4444",
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
          <XAxis type="number" tickFormatter={(v: number) => fmtPnlInt(v)} fontSize={11} />
          <YAxis type="category" dataKey="name" width={120} fontSize={11} tickLine={false} />
          <ChartTooltip content={<ChartTooltipContent formatter={(value) => fmtPnlInt(Number(value))} hideIndicator />} />
          <Bar dataKey="pnl" radius={[0, 4, 4, 0]} barSize={24}>
            {data.map((entry, index) => (
              <Cell key={index} fill={entry.fill} />
            ))}
          </Bar>
        </BarChart>
      </ChartContainer>
    </div>
  );
}
