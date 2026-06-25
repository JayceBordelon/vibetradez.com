import { Info } from "lucide-react";
import type * as React from "react";

import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

interface StatCardProps {
  label: string;
  value: string;
  sub?: string;
  /**
   * Icon component (lucide icons or custom SVG components both work).
   * The component is rendered as `<Icon className="..." />`, so any
   * component that accepts a className prop satisfies the type.
   */
  icon?: React.ComponentType<{ className?: string }>;
  tone?: "neutral" | "positive" | "negative";
  /**
   * Override the value text color with a raw CSS color string. Takes
   * precedence over `tone` for the value display. Useful for stats that
   * interpolate along a continuous scale (e.g. win rate from red to
   * green) rather than snapping to one of three semantic tones.
   */
  valueColor?: string;
  delta?: { value: string; positive: boolean };
  tooltip?: string;
  className?: string;
}

export function StatCard({ label, value, sub, icon: Icon, tone = "neutral", valueColor, delta, tooltip, className }: StatCardProps): React.JSX.Element {
  const valueToneClass = valueColor ? "" : tone === "positive" ? "text-green" : tone === "negative" ? "text-red" : "text-foreground";

  const dotToneClass = tone === "positive" ? "bg-green" : tone === "negative" ? "bg-red" : "bg-primary";

  const eyebrow = (
    <div className="flex items-center gap-2">
      {Icon ? (
        <Icon className="h-3.5 w-3.5 text-muted-foreground" />
      ) : (
        <span className={cn("h-[2px] w-[2px] rounded-full", dotToneClass)} aria-hidden>
          <span className={cn("block h-1.5 w-1.5 rounded-full", dotToneClass)} />
        </span>
      )}
      <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</span>
      {tooltip && <Info className="ml-auto h-3.5 w-3.5 text-muted-foreground/60 opacity-0 transition-opacity group-hover:opacity-100" aria-hidden />}
    </div>
  );

  // Open metric, no box: the label/number/sub sit directly on the page,
  // separated by spacing rather than card chrome.
  const card = (
    <div className={cn("group flex min-w-0 flex-col", className)}>
      {eyebrow}
      <div className={cn("mt-2 text-[28px] font-semibold tabular-nums leading-tight", valueToneClass)} style={valueColor ? { color: valueColor } : undefined}>
        {value}
      </div>
      {delta && (
        <div className={cn("mt-1 inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs font-medium", delta.positive ? "bg-green-bg text-green" : "bg-red-bg text-red")}>{delta.value}</div>
      )}
      {sub && <div className="mt-1 text-xs text-muted-foreground">{sub}</div>}
    </div>
  );

  if (!tooltip) return card;

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>{card}</TooltipTrigger>
        <TooltipContent side="top">{tooltip}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
