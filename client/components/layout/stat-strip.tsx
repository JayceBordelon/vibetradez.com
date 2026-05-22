import type * as React from "react";

import { cn } from "@/lib/utils";

interface StatStripProps {
  children: React.ReactNode;
  className?: string;
  /**
  Column count at the small breakpoint and up. Mobile is always
  2-up so the value text doesn't shrink below readable. Defaults
  to 4, which fits the standard P&L / Win-rate / ROC / Profit-factor
  set without crowding.
  */
  cols?: 2 | 3 | 4 | 5 | 6;
}

const colsClass: Record<NonNullable<StatStripProps["cols"]>, string> = {
  2: "sm:grid-cols-2",
  3: "sm:grid-cols-3",
  4: "sm:grid-cols-4",
  5: "sm:grid-cols-5",
  6: "sm:grid-cols-6",
};

/**
StatStrip lays a horizontal row of large readouts with thin hairline
dividers between them, the FT/Bloomberg pattern. It replaces the
4-card grid pattern (`StatCard` × 4 inside a Card wrapper) so a
single section reads as one continuous strip of numbers instead of
a row of bordered tiles.
*/
export function StatStrip({ children, className, cols = 4 }: StatStripProps): React.JSX.Element {
  return (
    <div className={cn("grid grid-cols-2 divide-x divide-y divide-border/50 border-y border-border/50 sm:divide-y-0 sm:border-y-0 sm:border-t sm:border-b", colsClass[cols], className)}>
      {children}
    </div>
  );
}

interface StatProps {
  label: string;
  value: React.ReactNode;
  sub?: React.ReactNode;
  tone?: "positive" | "negative" | "neutral";
  /**
  Override the value color with a raw CSS color string. Takes
  precedence over `tone`. Useful for continuous-scale stats (e.g.
  win-rate red→green) that shouldn't snap to one of three tones.
  */
  valueColor?: string;
  icon?: React.ComponentType<{ className?: string }>;
  className?: string;
}

export function Stat({ label, value, sub, tone = "neutral", valueColor, icon: Icon, className }: StatProps): React.JSX.Element {
  const valueToneClass = valueColor ? "" : tone === "positive" ? "text-green" : tone === "negative" ? "text-red" : "text-foreground";

  return (
    <div className={cn("min-w-0 px-4 py-4 sm:px-5", className)}>
      <div className="flex items-center gap-1.5">
        {Icon && <Icon className="h-3 w-3 shrink-0 text-muted-foreground" aria-hidden />}
        <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</span>
      </div>
      <div className={cn("mt-1.5 truncate text-[22px] font-semibold tabular-nums leading-tight sm:text-[26px]", valueToneClass)} style={valueColor ? { color: valueColor } : undefined}>
        {value}
      </div>
      {sub && <div className="mt-0.5 truncate text-[11px] text-muted-foreground">{sub}</div>}
    </div>
  );
}
