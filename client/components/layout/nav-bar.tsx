"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { AccountMenu } from "@/components/layout/account-menu";
import { Button } from "@/components/ui/button";
import { useSession } from "@/lib/session";
import { cn } from "@/lib/utils";

const tabs = [
  { href: "/dashboard", label: "Live Dashboard", short: "Live" },
  { href: "/history", label: "Historical Analytics", short: "History" },
] as const;

interface NavBarProps {
  onSubscribe?: () => void;
}

export function NavBar({ onSubscribe }: NavBarProps) {
  const pathname = usePathname();
  const { user, loading } = useSession();

  return (
    <header className="sticky top-0 z-30 border-b border-foreground/5 bg-background/55 backdrop-blur-2xl backdrop-saturate-150 dark:border-white/5">
      {/* Specular top edge — same idea as .lg-edge-shine but inline so we
          don't paint a pseudo-element across the full-width header. */}
      <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-b from-foreground/15 to-transparent dark:from-white/15" aria-hidden />
      <div className="mx-auto flex max-w-[1200px] flex-wrap items-center gap-x-4 gap-y-2 px-4 py-2.5 sm:px-7 sm:py-3">
        <Link href="/" className="inline-flex min-h-11 shrink-0 items-center py-2 text-[19px] font-extrabold tracking-tight sm:min-h-9 sm:py-0 sm:text-[21px]">
          <span className="text-foreground">Vibe</span>
          <span className="text-gradient-brand">Tradez</span>
        </Link>

        <nav className="order-3 flex w-full items-center justify-center gap-1 sm:order-none sm:ml-3 sm:w-auto sm:justify-start">
          {tabs.map((tab) => {
            const isActive = pathname === tab.href;
            return (
              <Link
                key={tab.href}
                href={tab.href}
                aria-current={isActive ? "page" : undefined}
                className={cn(
                  "relative inline-flex min-h-9 items-center px-3 py-1.5 text-sm font-semibold tracking-tight transition-colors sm:min-h-0 sm:px-4",
                  isActive ? "text-foreground" : "text-muted-foreground hover:text-foreground"
                )}
              >
                <span className="sm:hidden">{tab.short}</span>
                <span className="hidden sm:inline">{tab.label}</span>
                {isActive && <span aria-hidden className="absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-gradient-brand sm:inset-x-3" />}
              </Link>
            );
          })}
        </nav>

        <div className="ml-auto flex items-center gap-2">
          {loading ? (
            <div className="h-8 w-20 rounded-full bg-foreground/5 dark:bg-white/5" aria-hidden="true" />
          ) : user ? (
            <AccountMenu user={user} />
          ) : (
            onSubscribe && (
              <Button
                variant="outline"
                size="sm"
                onClick={onSubscribe}
                className="h-9 gap-1.5 rounded-full border-foreground/10 bg-foreground/5 px-3 text-xs font-semibold hover:bg-foreground/10 dark:border-white/10 dark:bg-white/5 dark:hover:bg-white/10 sm:px-3.5 sm:text-sm"
                aria-label="Sign in or sign up"
              >
                Sign in
              </Button>
            )
          )}
        </div>
      </div>
    </header>
  );
}
