"use client";

import type * as React from "react";

import { cn } from "@/lib/utils";

interface PageToolbarProps {
	title: string;
	subtitle?: string;
	primaryControls?: React.ReactNode;
	secondaryControls?: React.ReactNode;
	rightSlot?: React.ReactNode;
}

export function PageToolbar({
	title,
	subtitle,
	primaryControls,
	secondaryControls,
	rightSlot,
}: PageToolbarProps): React.JSX.Element {
	const hasControls = Boolean(primaryControls || secondaryControls || rightSlot);

	return (
		<div
			className={cn(
				"sticky top-0 z-10 border-b bg-background/80 backdrop-blur-md",
			)}
		>
			<div className="mx-auto flex max-w-[1200px] flex-col gap-3 px-4 py-4 sm:px-7 lg:flex-row lg:items-center lg:justify-between lg:gap-6">
				<div className="min-w-0 lg:flex-1">
					<h1 className="truncate text-xl font-semibold tracking-tight sm:text-2xl">
						{title}
					</h1>
					{subtitle && (
						<p className="mt-0.5 truncate text-sm text-muted-foreground">
							{subtitle}
						</p>
					)}
				</div>
				{hasControls && (
					<div className="min-w-0 lg:flex-shrink-0">
						<div className="flex flex-wrap items-center gap-2 sm:gap-3">
							{primaryControls}
							{secondaryControls}
							{rightSlot}
						</div>
					</div>
				)}
			</div>
		</div>
	);
}
