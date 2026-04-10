"use client";

import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

const OPTIONS = ["1", "3", "5", "10"] as const;

interface TopNFilterProps {
	value: number;
	onChange: (n: number) => void;
}

export function TopNFilter({ value, onChange }: TopNFilterProps) {
	return (
		<ToggleGroup
			type="single"
			value={String(value)}
			onValueChange={(v) => v && onChange(Number(v))}
			variant="outline"
			size="sm"
		>
			{OPTIONS.map((n) => (
				<ToggleGroupItem
					key={n}
					value={n}
					className="h-9 px-3 text-sm font-semibold"
				>
					Top {n}
				</ToggleGroupItem>
			))}
		</ToggleGroup>
	);
}
