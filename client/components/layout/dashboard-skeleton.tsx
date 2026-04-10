import type * as React from "react";

import { Skeleton } from "@/components/ui/skeleton";

export function DashboardSkeleton(): React.JSX.Element {
	return (
		<div className="space-y-6 py-4">
			<div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
				{Array.from({ length: 4 }).map((_, i) => (
					<Skeleton key={i} className="h-[120px] rounded-lg" />
				))}
			</div>
			<Skeleton className="h-[360px] w-full rounded-lg" />
			<div className="space-y-2">
				{Array.from({ length: 6 }).map((_, i) => (
					<Skeleton key={i} className="h-12 w-full rounded-md" />
				))}
			</div>
		</div>
	);
}

export function HistorySkeleton(): React.JSX.Element {
	return (
		<div className="space-y-6 py-4">
			<div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
				{Array.from({ length: 4 }).map((_, i) => (
					<Skeleton key={i} className="h-[120px] rounded-lg" />
				))}
			</div>
			<div className="grid grid-cols-2 gap-3 sm:grid-cols-4 xl:grid-cols-8">
				{Array.from({ length: 8 }).map((_, i) => (
					<Skeleton key={i} className="h-[88px] rounded-lg" />
				))}
			</div>
			<div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
				{Array.from({ length: 3 }).map((_, i) => (
					<Skeleton key={i} className="h-[240px] rounded-lg" />
				))}
			</div>
		</div>
	);
}
