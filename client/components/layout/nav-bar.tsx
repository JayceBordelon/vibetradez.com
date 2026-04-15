"use client";

import { Mail } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { ModelPickerFilter } from "@/components/layout/model-picker-filter";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const tabs = [
  { href: "/dashboard", label: "Live Dashboard", short: "Live" },
  { href: "/history", label: "Historical Analytics", short: "History" },
  { href: "/models", label: "Models", short: "Models" },
] as const;

interface NavBarProps {
  onSubscribe?: () => void;
}

export function NavBar({ onSubscribe }: NavBarProps) {
  const pathname = usePathname();

  return (
    <header className="sticky top-0 z-20 border-b bg-background/85 backdrop-blur-md">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 px-3 py-2 sm:px-7 sm:py-2.5">
        <Link href="/" className="shrink-0 text-[19px] font-extrabold tracking-tight sm:text-[22px]">
          <span className="text-foreground">Vibe</span>
          <span className="text-gradient-brand">Tradez</span>
        </Link>

        <nav className="order-3 flex w-full items-stretch justify-center sm:order-none sm:ml-2 sm:w-auto sm:justify-start">
          {tabs.map((tab) => {
            const isActive = pathname === tab.href;
            return (
              <Link
                key={tab.href}
                href={tab.href}
                className={cn(
                  "flex items-center border-b-2 px-3 py-1.5 text-sm font-semibold tracking-wide transition-colors sm:px-4",
                  isActive ? "border-transparent text-foreground" : "border-transparent text-muted-foreground hover:bg-muted hover:text-foreground",
                  isActive && "nav-tab-active"
                )}
              >
                <span className="sm:hidden">{tab.short}</span>
                <span className="hidden sm:inline">{tab.label}</span>
              </Link>
            );
          })}
        </nav>

        <div className="ml-auto flex items-center gap-2">
          {pathname !== "/models" && <ModelPickerFilter />}
          {onSubscribe && (
            <Button variant="outline" size="sm" onClick={onSubscribe} className="h-8 gap-1.5 px-2 text-xs sm:px-3 sm:text-sm" aria-label="Subscribe">
              <Mail className="h-3.5 w-3.5 sm:hidden" />
              <span className="hidden sm:inline">Subscribe</span>
            </Button>
          )}
        </div>
      </div>
    </header>
  );
}
