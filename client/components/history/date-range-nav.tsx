"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";

import { Button } from "@/components/ui/button";

export function DateRangeNav({
	label,
	canPrev,
	canNext,
	onPrev,
	onNext,
}: {
	label: string;
	canPrev: boolean;
	canNext: boolean;
	onPrev: () => void;
	onNext: () => void;
}) {
	return (
		<div className="flex shrink-0 items-center overflow-hidden rounded-md border bg-card">
			<Button
				variant="ghost"
				size="icon"
				className="h-9 w-9 rounded-none"
				disabled={!canPrev}
				onClick={onPrev}
				aria-label="Previous period"
			>
				<ChevronLeft className="h-4 w-4" />
			</Button>
			<span className="min-w-[110px] border-x px-2 py-1.5 text-center text-xs font-medium tabular-nums sm:min-w-[140px] sm:px-3 sm:text-sm">
				{label}
			</span>
			<Button
				variant="ghost"
				size="icon"
				className="h-9 w-9 rounded-none"
				disabled={!canNext}
				onClick={onNext}
				aria-label="Next period"
			>
				<ChevronRight className="h-4 w-4" />
			</Button>
		</div>
	);
}
