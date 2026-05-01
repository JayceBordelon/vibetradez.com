"use client";

import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";

const OPTIONS = [
  { value: "week", label: "Week" },
  { value: "month", label: "Month" },
  { value: "year", label: "Year" },
  { value: "all", label: "All" },
] as const;

export function ModeToggle({ mode, onChange }: { mode: string; onChange: (mode: string) => void }) {
  return (
    <Tabs value={mode} onValueChange={onChange}>
      <TabsList className="lg-control h-11 gap-1 bg-transparent p-1 sm:h-9">
        {OPTIONS.map((o) => (
          <TabsTrigger key={o.value} value={o.value} className="h-9 px-3 text-xs font-semibold sm:h-7 sm:px-4 sm:text-sm">
            {o.label}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  );
}
