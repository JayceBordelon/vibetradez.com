import Link from "next/link";

import { Button } from "@/components/ui/button";

interface TopBarProps {
	onSubscribe?: () => void;
}

export function TopBar({ onSubscribe }: TopBarProps) {
	return (
		<div className="flex items-center justify-between border-b bg-card px-4 sm:px-7 py-3">
			<Link
				href="/"
				className="text-[22px] font-extrabold tracking-tight"
			>
				<span className="text-foreground">Vibe</span>
				<span className="text-gradient-brand">Tradez</span>
			</Link>
			{onSubscribe && (
				<Button variant="outline" size="sm" onClick={onSubscribe}>
					Subscribe
				</Button>
			)}
		</div>
	);
}
