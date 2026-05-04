"use client";

import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { type ChartConfig, ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { formatMonthDay } from "@/lib/date-utils";
import { fmtMoneyInt } from "@/lib/format";

export function ExposureReturnsChart({ data }: { data: { date: string; invested: number; returned: number }[] }) {
  const chartConfig: ChartConfig = {
    invested: {
      label: "Invested",
      color: "var(--muted-foreground)",
    },
    returned: {
      label: "Returned",
      color: "var(--gpt)",
    },
  };

  return (
    <Card className="lg-card">
      <CardHeader>
        <CardTitle className="text-base">Exposure vs Returns</CardTitle>
        <CardDescription>Capital deployed compared to capital returned each day</CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="min-h-[260px] w-full">
          <BarChart data={data} accessibilityLayer>
            <CartesianGrid vertical={false} />
            <XAxis dataKey="date" tickLine={false} axisLine={false} tickMargin={8} tickFormatter={(v: string) => formatMonthDay(v)} />
            <YAxis tickLine={false} axisLine={false} tickMargin={8} tickFormatter={(v: number) => fmtMoneyInt(v)} />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  labelFormatter={(_, payload) => {
                    const item = payload?.[0]?.payload as { date: string } | undefined;
                    return item ? formatMonthDay(item.date) : "";
                  }}
                  formatter={(value) => fmtMoneyInt(value as number)}
                />
              }
            />
            <ChartLegend content={<ChartLegendContent />} />
            <Bar dataKey="invested" fill="var(--color-invested)" radius={[3, 3, 0, 0]} />
            <Bar dataKey="returned" fill="var(--color-returned)" radius={[3, 3, 0, 0]} />
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
