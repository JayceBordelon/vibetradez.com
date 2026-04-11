import type * as React from "react";

import { cn } from "@/lib/utils";

interface MetricProps {
  label: string;
  value: string | React.ReactNode;
  className?: string;
  align?: "left" | "right";
}

export function Metric({ label, value, className, align = "left" }: MetricProps): React.JSX.Element {
  return (
    <div className={cn("flex min-w-0 items-baseline justify-between gap-2", className)}>
      <span className="shrink-0 text-xs text-muted-foreground">{label}</span>
      {typeof value === "string" ? (
        <span className={cn("min-w-0 truncate text-right text-sm tabular-nums", align === "right" ? "font-semibold" : "font-medium")}>{value}</span>
      ) : (
        <span className="min-w-0 truncate text-right">{value}</span>
      )}
    </div>
  );
}
