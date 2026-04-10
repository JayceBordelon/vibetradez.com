"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { cn } from "@/lib/utils";

const tabs = [
	{ href: "/", label: "Live Dashboard" },
	{ href: "/history", label: "Historical Analytics" },
	{ href: "/models", label: "Models" },
] as const;

export function NavBar() {
	const pathname = usePathname();

	return (
		<div className="flex items-stretch border-b bg-card px-4 sm:px-7">
			{tabs.map((tab) => {
				const isActive = pathname === tab.href;
				return (
					<Link
						key={tab.href}
						href={tab.href}
						className={cn(
							"flex items-center border-b-2 px-5 py-3 text-sm font-semibold tracking-wide transition-colors",
							isActive
								? "border-primary text-primary"
								: "border-transparent text-muted-foreground hover:bg-muted hover:text-foreground",
						)}
					>
						{tab.label}
					</Link>
				);
			})}
		</div>
	);
}
