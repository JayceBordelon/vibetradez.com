"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";

import { Button } from "@/components/ui/button";
import { formatDateShort } from "@/lib/date-utils";

interface DateNavigatorProps {
  dates: string[];
  index: number;
  onChange: (i: number) => void;
}

export function DateNavigator({ dates, index, onChange }: DateNavigatorProps) {
  const label = dates.length > 0 && index < dates.length ? formatDateShort(dates[index]) : "No data";

  return (
    <div className="flex items-center overflow-hidden rounded-md border bg-card">
      <Button variant="ghost" size="icon" className="h-9 w-9 rounded-none" disabled={index >= dates.length - 1} onClick={() => onChange(index + 1)} aria-label="Previous day">
        <ChevronLeft className="h-4 w-4" />
      </Button>
      <span className="min-w-[140px] border-x px-3 py-1.5 text-center text-sm font-medium">{label}</span>
      <Button variant="ghost" size="icon" className="h-9 w-9 rounded-none" disabled={index <= 0} onClick={() => onChange(index - 1)} aria-label="Next day">
        <ChevronRight className="h-4 w-4" />
      </Button>
    </div>
  );
}
