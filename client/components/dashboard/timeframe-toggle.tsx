"use client";

import { cn } from "@/lib/utils";
import type { ChartParams } from "@/types/trade";

/*
Schwab pricehistory accepts (periodType × period × frequencyType ×
frequency). The presets below cover the common trader windows:
intraday minute candles for 1D / 5D, daily candles for the rest.
*/
export interface TimeframePreset {
  id: string;
  label: string;
  params: ChartParams;
  /** True when candles span multiple days — let the chart layer
   *  swap its X-axis tick formatter to dates instead of times. */
  multiDay: boolean;
}

export const TIMEFRAME_PRESETS: TimeframePreset[] = [
  { id: "1D", label: "1D", params: { period: 1, ptype: "day", ftype: "minute", freq: 5 }, multiDay: false },
  { id: "5D", label: "5D", params: { period: 5, ptype: "day", ftype: "minute", freq: 5 }, multiDay: true },
  { id: "1M", label: "1M", params: { period: 1, ptype: "month", ftype: "daily", freq: 1 }, multiDay: true },
  { id: "3M", label: "3M", params: { period: 3, ptype: "month", ftype: "daily", freq: 1 }, multiDay: true },
  { id: "6M", label: "6M", params: { period: 6, ptype: "month", ftype: "daily", freq: 1 }, multiDay: true },
  { id: "1Y", label: "1Y", params: { period: 1, ptype: "year", ftype: "daily", freq: 1 }, multiDay: true },
];

interface TimeframeToggleProps {
  value: string;
  onChange: (preset: TimeframePreset) => void;
}

export function TimeframeToggle({ value, onChange }: TimeframeToggleProps) {
  return (
    <div className="lg-control inline-flex shrink-0 items-center gap-0.5 p-0.5" role="tablist" aria-label="Chart timeframe">
      {TIMEFRAME_PRESETS.map((p) => {
        const isActive = p.id === value;
        return (
          <button
            key={p.id}
            type="button"
            role="tab"
            aria-selected={isActive}
            onClick={() => onChange(p)}
            className={cn(
              "inline-flex h-9 min-w-10 items-center justify-center rounded-md px-2 text-xs font-semibold tabular-nums transition-colors sm:h-7 sm:text-[11px]",
              isActive ? "bg-foreground text-background shadow-sm" : "text-muted-foreground hover:text-foreground"
            )}
          >
            {p.label}
          </button>
        );
      })}
    </div>
  );
}
