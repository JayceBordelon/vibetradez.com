import type * as React from "react";

import { cn } from "@/lib/utils";

interface StatStripProps {
  children: React.ReactNode;
  className?: string;
  /**
  Column count once the row is wide enough to sit on one line. Mobile stays
  2-up (or 3-up for the dense counts) so the value text stays readable.
  Defaults to 4.
  */
  cols?: 2 | 3 | 4 | 5 | 6;
}

/*
Each layout is an OPEN metrics row, not a grid of boxes: big tabular numbers
over quiet labels, separated by generous whitespace and — once they sit on a
single line — a single thin vertical hairline (`divide-x`) between items. No
per-stat borders, fills, or shadows. The dividers only switch on at the
breakpoint where the strip is one row, so a 2×2 mobile wrap never shows a
stray rule; there, whitespace alone does the separating. The matching child
padding gives each number room to breathe off its hairline.
*/
const colsClass: Record<NonNullable<StatStripProps["cols"]>, string> = {
  2: "grid-cols-2 gap-x-6 sm:gap-x-0 sm:divide-x sm:divide-border sm:[&>*]:px-6 sm:[&>*:first-child]:pl-0 sm:[&>*:last-child]:pr-0",
  3: "grid-cols-2 gap-x-6 gap-y-7 sm:grid-cols-3 sm:gap-x-0 sm:gap-y-0 sm:divide-x sm:divide-border sm:[&>*]:px-5 sm:[&>*:first-child]:pl-0 sm:[&>*:last-child]:pr-0",
  4: "grid-cols-2 gap-x-6 gap-y-8 lg:grid-cols-4 lg:gap-x-0 lg:gap-y-0 lg:divide-x lg:divide-border lg:[&>*]:px-6 lg:[&>*:first-child]:pl-0 lg:[&>*:last-child]:pr-0",
  5: "grid-cols-2 gap-x-6 gap-y-8 sm:grid-cols-5 sm:gap-x-0 sm:gap-y-0 sm:divide-x sm:divide-border sm:[&>*]:px-5 sm:[&>*:first-child]:pl-0 sm:[&>*:last-child]:pr-0",
  6: "grid-cols-3 gap-x-6 gap-y-8 sm:grid-cols-6 sm:gap-x-0 sm:gap-y-0 sm:divide-x sm:divide-border sm:[&>*]:px-5 sm:[&>*:first-child]:pl-0 sm:[&>*:last-child]:pr-0",
};

export function StatStrip({ children, className, cols = 4 }: StatStripProps): React.JSX.Element {
  return <div className={cn("grid", colsClass[cols], className)}>{children}</div>;
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

/*
One open metric: a quiet uppercase label over a big tabular number, with an
optional sub line. No box — it leans entirely on type and the strip's
hairline/whitespace rhythm for separation.
*/
export function Stat({ label, value, sub, tone = "neutral", valueColor, icon: Icon, className }: StatProps): React.JSX.Element {
  const valueToneClass = valueColor ? "" : tone === "positive" ? "text-green" : tone === "negative" ? "text-red" : "text-foreground";

  return (
    <div className={cn("flex min-w-0 flex-col", className)}>
      <div className="flex items-center gap-1.5">
        {Icon && <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />}
        <span className="truncate text-[11px] font-medium uppercase tracking-[0.08em] text-muted-foreground">{label}</span>
      </div>
      <div
        className={cn("mt-2 truncate text-[26px] font-semibold leading-tight tracking-tight tabular-nums sm:text-[30px]", valueToneClass)}
        style={valueColor ? { color: valueColor } : undefined}
      >
        {value}
      </div>
      {sub && <div className="mt-1.5 truncate text-xs text-muted-foreground">{sub}</div>}
    </div>
  );
}
