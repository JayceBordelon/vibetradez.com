"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { ModelPickerFilter } from "@/components/layout/model-picker-filter";
import { cn } from "@/lib/utils";

const tabs = [
  { href: "/dashboard", label: "Live Dashboard", short: "Live" },
  { href: "/history", label: "Historical Analytics", short: "History" },
  { href: "/models", label: "Models", short: "Models" },
] as const;

export function NavBar() {
  const pathname = usePathname();

  return (
    <div className="flex flex-wrap items-stretch justify-center gap-3 border-b bg-card px-2 py-1.5 sm:justify-between sm:px-7">
      <div className="flex flex-wrap items-stretch justify-center">
        {tabs.map((tab) => {
          const isActive = pathname === tab.href;
          return (
            <Link
              key={tab.href}
              href={tab.href}
              className={cn(
                "flex items-center border-b-2 px-3 py-2 text-sm font-semibold tracking-wide transition-colors sm:px-5",
                isActive ? "border-transparent text-foreground" : "border-transparent text-muted-foreground hover:bg-muted hover:text-foreground",
                isActive && "nav-tab-active"
              )}
            >
              <span className="sm:hidden">{tab.short}</span>
              <span className="hidden sm:inline">{tab.label}</span>
            </Link>
          );
        })}
      </div>
      {pathname !== "/models" && <ModelPickerFilter />}
    </div>
  );
}
