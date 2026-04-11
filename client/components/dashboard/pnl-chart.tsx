"use client";

import { useMemo } from "react";
import { Bar, BarChart, Cell, XAxis, YAxis } from "recharts";
import { type ChartConfig, ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { fmtPnlInt } from "@/lib/format";
import type { DashboardTrade } from "@/types/trade";

interface PnlChartProps {
  trades: DashboardTrade[];
}

const chartConfig: ChartConfig = {
  pnl: {
    label: "P&L",
  },
};

export function PnlChart({ trades }: PnlChartProps) {
  const data = useMemo(() => {
    return trades
      .filter((dt) => dt.summary)
      .map((dt) => {
        const entry = dt.summary!.entry_price;
        const close = dt.summary!.closing_price;
        const pnl = (close - entry) * 100;
        return {
          name: `$${dt.trade.symbol} ${dt.trade.contract_type}`,
          pnl,
          fill: pnl >= 0 ? "#10b981" : "#ef4444",
        };
      })
      .sort((a, b) => b.pnl - a.pnl);
  }, [trades]);

  if (data.length === 0) {
    return <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">No closed trades to chart</div>;
  }

  const chartHeight = Math.max(200, data.length * 40);

  return (
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
  );
}
