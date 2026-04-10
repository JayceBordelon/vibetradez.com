"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { cn } from "@/lib/utils";

const tabs = [
	{ href: "/", label: "Live Dashboard", short: "Live" },
	{ href: "/history", label: "Historical Analytics", short: "History" },
	{ href: "/models", label: "Models", short: "Models" },
] as const;

export function NavBar() {
	const pathname = usePathname();

	return (
		<div className="flex flex-wrap items-stretch justify-center border-b bg-card px-2 sm:px-7">
			{tabs.map((tab) => {
				const isActive = pathname === tab.href;
				return (
					<Link
						key={tab.href}
						href={tab.href}
						className={cn(
							"flex items-center border-b-2 px-3 py-3 text-sm font-semibold tracking-wide transition-colors sm:px-5",
							isActive
								? "border-primary text-primary"
								: "border-transparent text-muted-foreground hover:bg-muted hover:text-foreground",
						)}
					>
						<span className="sm:hidden">{tab.short}</span>
						<span className="hidden sm:inline">{tab.label}</span>
					</Link>
				);
			})}
		</div>
	);
}
