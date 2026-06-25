"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { AccountMenu } from "@/components/layout/account-menu";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { Wordmark } from "@/components/layout/wordmark";
import { Button } from "@/components/ui/button";
import { useSession } from "@/lib/session";
import { cn } from "@/lib/utils";

const tabs = [
  { href: "/dashboard", label: "Dashboard", short: "Dashboard" },
  { href: "/holdings", label: "Holdings", short: "Holdings" },
  { href: "/closed", label: "Closed", short: "Closed" },
  { href: "/transcripts", label: "Sessions", short: "Sessions" },
] as const;

interface NavBarProps {
  onSubscribe?: () => void;
}

export function NavBar({ onSubscribe }: NavBarProps) {
  const pathname = usePathname();
  const { user, loading } = useSession();

  return (
    <header className="sticky top-0 z-30 border-b border-border/70 bg-background/80 backdrop-blur-xl">
      <div className="mx-auto flex max-w-[1200px] flex-wrap items-center gap-x-5 gap-y-1 px-4 py-2.5 sm:px-7 sm:py-3">
        <Link href="/" className="inline-flex min-h-11 shrink-0 items-center py-1 sm:min-h-9 sm:py-0" aria-label="VibeTradez home">
          <Wordmark />
        </Link>

        <nav className="order-3 flex w-full items-center justify-center gap-1 sm:order-none sm:ml-2 sm:w-auto sm:justify-start">
          {tabs.map((tab) => {
            const isActive = pathname === tab.href || pathname.startsWith(`${tab.href}/`);
            return (
              <Link
                key={tab.href}
                href={tab.href}
                aria-current={isActive ? "page" : undefined}
                className={cn(
                  "relative inline-flex min-h-9 items-center rounded-full px-3 py-1.5 text-[13px] font-medium transition-colors sm:min-h-0",
                  isActive ? "bg-secondary text-secondary-foreground" : "text-muted-foreground hover:bg-muted hover:text-foreground",
                )}
              >
                <span className="sm:hidden">{tab.short}</span>
                <span className="hidden sm:inline">{tab.label}</span>
              </Link>
            );
          })}
        </nav>

        <div className="ml-auto flex items-center gap-2">
          <ThemeToggle />
          {loading ? (
            <div className="h-9 w-20 rounded-full bg-muted/60" aria-hidden="true" />
          ) : user ? (
            <AccountMenu user={user} />
          ) : (
            onSubscribe && (
              <Button variant="outline" size="sm" onClick={onSubscribe} className="h-9 rounded-full px-4 text-[13px] font-medium" aria-label="Sign in or sign up">
                Sign in
              </Button>
            )
          )}
        </div>
      </div>
    </header>
  );
}
