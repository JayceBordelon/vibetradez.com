import { Info, type LucideIcon } from "lucide-react";
import type * as React from "react";

import { Card, CardContent } from "@/components/ui/card";
import {
	Tooltip,
	TooltipContent,
	TooltipProvider,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

interface StatCardProps {
	label: string;
	value: string;
	sub?: string;
	icon?: LucideIcon;
	tone?: "neutral" | "positive" | "negative";
	delta?: { value: string; positive: boolean };
	tooltip?: string;
	className?: string;
	index?: number;
}

export function StatCard({
	label,
	value,
	sub,
	icon: Icon,
	tone = "neutral",
	delta,
	tooltip,
	className,
	index,
}: StatCardProps): React.JSX.Element {
	const valueToneClass =
		tone === "positive"
			? "text-green"
			: tone === "negative"
				? "text-red"
				: "text-foreground";

	const dotToneClass =
		tone === "positive"
			? "bg-green"
			: tone === "negative"
				? "bg-red"
				: "bg-primary";

	const animationStyle =
		typeof index === "number"
			? { animationDelay: `${index * 40}ms` }
			: undefined;

	const eyebrow = (
		<div className="flex items-center gap-2">
			{Icon ? (
				<Icon className="h-3.5 w-3.5 text-muted-foreground" />
			) : (
				<span
					className={cn("h-[2px] w-[2px] rounded-full", dotToneClass)}
					aria-hidden
				>
					<span
						className={cn("block h-1.5 w-1.5 rounded-full", dotToneClass)}
					/>
				</span>
			)}
			<span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
				{label}
			</span>
			{tooltip && (
				<Info
					className="ml-auto h-3.5 w-3.5 text-muted-foreground/60 opacity-0 transition-opacity group-hover:opacity-100"
					aria-hidden
				/>
			)}
		</div>
	);

	const body = (
		<CardContent className="p-5">
			{eyebrow}
			<div
				className={cn(
					"mt-2 text-[28px] font-semibold tabular-nums leading-tight",
					valueToneClass,
				)}
			>
				{value}
			</div>
			{delta && (
				<div
					className={cn(
						"mt-1 inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs font-medium",
						delta.positive
							? "bg-green-bg text-green"
							: "bg-red-bg text-red",
					)}
				>
					{delta.value}
				</div>
			)}
			{sub && <div className="mt-1 text-xs text-muted-foreground">{sub}</div>}
		</CardContent>
	);

	const card = (
		<Card
			className={cn(
				"group gap-0 py-0 transition-all duration-150 hover:-translate-y-0.5 hover:shadow-md",
				typeof index === "number" &&
					"animate-in fade-in slide-in-from-bottom-1 duration-300",
				className,
			)}
			style={animationStyle}
		>
			{body}
		</Card>
	);

	if (!tooltip) return card;

	return (
		<TooltipProvider>
			<Tooltip>
				<TooltipTrigger asChild>{card}</TooltipTrigger>
				<TooltipContent side="top">{tooltip}</TooltipContent>
			</Tooltip>
		</TooltipProvider>
	);
}
