import type * as React from "react";

import { Skeleton } from "@/components/ui/skeleton";

// Mirrors the dashboard's real anatomy (stat strip, the P&L chart, the
// executions tape) so the page doesn't reflow when data lands.
export function DashboardSkeleton(): React.JSX.Element {
  return (
    <div className="space-y-6 py-4">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-[120px] rounded-2xl" />
        ))}
      </div>
      <Skeleton className="h-[360px] w-full rounded-2xl" />
      <div className="space-y-2">
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full rounded-xl" />
        ))}
      </div>
    </div>
  );
}
