"use client";

import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

const OPTIONS = ["1", "3", "5", "10"] as const;

interface TopNFilterProps {
  value: number;
  onChange: (n: number) => void;
}

export function TopNFilter({ value, onChange }: TopNFilterProps) {
  return (
    <div className="flex shrink-0 items-center gap-2">
      <span className="hidden text-[11px] font-semibold uppercase tracking-wider text-muted-foreground sm:inline">Top</span>
      <ToggleGroup type="single" value={String(value)} onValueChange={(v) => v && onChange(Number(v))} variant="outline" size="sm">
        {OPTIONS.map((n) => (
          <ToggleGroupItem key={n} value={n} className="h-8 px-2 text-xs font-semibold sm:px-3 sm:text-sm">
            {n}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  );
}
