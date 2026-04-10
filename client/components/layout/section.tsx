import type * as React from "react";

import { cn } from "@/lib/utils";

interface SectionProps {
	title?: string;
	subtitle?: string;
	actions?: React.ReactNode;
	children: React.ReactNode;
	className?: string;
	contentClassName?: string;
}

export function Section({
	title,
	subtitle,
	actions,
	children,
	className,
	contentClassName,
}: SectionProps): React.JSX.Element {
	const hasHeader = Boolean(title || subtitle || actions);

	return (
		<section className={cn("py-8", className)}>
			{hasHeader && (
				<div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
					<div className="min-w-0">
						{title && (
							<h2 className="text-lg font-semibold tracking-tight">
								{title}
							</h2>
						)}
						{subtitle && (
							<p className="mt-0.5 text-sm text-muted-foreground">
								{subtitle}
							</p>
						)}
					</div>
					{actions && (
						<div className="flex shrink-0 items-center gap-2 sm:ml-4">
							{actions}
						</div>
					)}
				</div>
			)}
			<div className={cn("mt-4", contentClassName)}>{children}</div>
		</section>
	);
}
