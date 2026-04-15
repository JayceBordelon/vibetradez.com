"use client";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

const OPTIONS = [1, 3, 5, 10] as const;

interface TopNFilterProps {
  value: number;
  onChange: (n: number) => void;
}

export function TopNFilter({ value, onChange }: TopNFilterProps) {
  return (
    <Select value={String(value)} onValueChange={(v) => onChange(Number(v))}>
      <SelectTrigger size="sm" className="h-8 w-[104px] text-xs font-semibold" aria-label="Show top N picks">
        <SelectValue />
      </SelectTrigger>
      <SelectContent align="end">
        {OPTIONS.map((n) => (
          <SelectItem key={n} value={String(n)} className="text-xs">
            Top {n}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
