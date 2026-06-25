import type * as React from "react";

import { cn } from "@/lib/utils";

interface StatStripProps {
  children: React.ReactNode;
  className?: string;
  /**
  Column count at the small breakpoint and up. Mobile is always 2-up so
  the value text stays readable. Defaults to 4.
  */
  cols?: 2 | 3 | 4 | 5 | 6;
}

const colsClass: Record<NonNullable<StatStripProps["cols"]>, string> = {
  2: "sm:grid-cols-2",
  3: "sm:grid-cols-3",
  4: "sm:grid-cols-2 lg:grid-cols-4",
  5: "sm:grid-cols-5",
  6: "sm:grid-cols-3 lg:grid-cols-6",
};

/*
StatStrip lays out a responsive grid of clean KPI tiles — the modern
product-dashboard pattern. Each Stat is a soft white card that leads with
one big tabular number over a quiet label, with an optional sub line. Calm
spacing, hairline borders, very soft shadows.
*/
export function StatStrip({ children, className, cols = 4 }: StatStripProps): React.JSX.Element {
  return <div className={cn("grid grid-cols-2 gap-3 sm:gap-4", colsClass[cols], className)}>{children}</div>;
}

interface StatProps {
  label: string;
  value: React.ReactNode;
  sub?: React.ReactNode;
  tone?: "positive" | "negative" | "neutral";
  /**
  Override the value color with a raw CSS color string. Takes precedence
  over `tone`. Useful for continuous-scale stats (e.g. win-rate red→green).
  */
  valueColor?: string;
  icon?: React.ComponentType<{ className?: string }>;
  className?: string;
}

export function Stat({ label, value, sub, tone = "neutral", valueColor, icon: Icon, className }: StatProps): React.JSX.Element {
  const valueToneClass = valueColor ? "" : tone === "positive" ? "text-green" : tone === "negative" ? "text-red" : "text-foreground";

  return (
    <div className={cn("flex min-w-0 flex-col rounded-xl border border-border bg-card px-4 py-4 shadow-sm sm:px-5 sm:py-[1.15rem]", className)}>
      <div className="flex items-center gap-1.5">
        {Icon && <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />}
        <span className="truncate text-[11px] font-medium uppercase tracking-[0.08em] text-muted-foreground">{label}</span>
      </div>
      <div
        className={cn("mt-2 truncate text-[24px] font-semibold leading-tight tracking-tight tabular-nums sm:text-[28px]", valueToneClass)}
        style={valueColor ? { color: valueColor } : undefined}
      >
        {value}
      </div>
      {sub && <div className="mt-1.5 truncate text-xs text-muted-foreground">{sub}</div>}
    </div>
  );
}
