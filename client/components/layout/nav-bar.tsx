"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { AccountMenu } from "@/components/layout/account-menu";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { Button } from "@/components/ui/button";
import { useSession } from "@/lib/session";
import { cn } from "@/lib/utils";

const tabs = [
  { href: "/dashboard", label: "Live Dashboard", short: "Live" },
  { href: "/holdings", label: "Holdings", short: "Holdings" },
  { href: "/closed", label: "Closed", short: "Closed" },
] as const;

interface NavBarProps {
  onSubscribe?: () => void;
}

export function NavBar({ onSubscribe }: NavBarProps) {
  const pathname = usePathname();
  const { user, loading } = useSession();

  return (
    <header className="sticky top-0 z-30 border-b border-dashed border-border bg-background/90 backdrop-blur-sm">
      <div className="mx-auto flex max-w-[1200px] flex-wrap items-center gap-x-4 gap-y-0 px-4 py-2 sm:px-7 sm:py-2.5">
        <Link href="/" className="inline-flex min-h-11 shrink-0 items-center py-2 font-mono text-[15px] font-extrabold tracking-tight sm:min-h-9 sm:py-0">
          <span className="text-foreground">VIBE</span>
          <span className="phosphor text-green">TRADEZ</span>
        </Link>

        <nav className="order-3 flex w-full items-center justify-center gap-1 sm:order-none sm:ml-4 sm:w-auto sm:justify-start">
          {tabs.map((tab) => {
            const isActive = pathname === tab.href || pathname.startsWith(`${tab.href}/`);
            return (
              <Link
                key={tab.href}
                href={tab.href}
                aria-current={isActive ? "page" : undefined}
                className={cn(
                  "relative inline-flex min-h-9 items-center px-3 py-1.5 font-mono text-[12px] font-bold uppercase tracking-[0.08em] transition-colors sm:min-h-0",
                  isActive ? "phosphor text-green" : "text-muted-foreground hover:text-foreground"
                )}
              >
                <span aria-hidden className={cn("mr-1.5", isActive ? "text-green" : "text-muted-foreground/50")}>
                  {isActive ? ">" : "·"}
                </span>
                <span className="sm:hidden">{tab.short}</span>
                <span className="hidden sm:inline">{tab.label}</span>
              </Link>
            );
          })}
        </nav>

        <div className="ml-auto flex items-center gap-2">
          <ThemeToggle />
          {loading ? (
            <div className="h-8 w-20 border border-dashed border-border bg-muted/40" aria-hidden="true" />
          ) : user ? (
            <AccountMenu user={user} />
          ) : (
            onSubscribe && (
              <Button
                variant="outline"
                size="sm"
                onClick={onSubscribe}
                className="h-9 gap-1.5 rounded-none border-border bg-transparent px-3 font-mono text-xs font-bold uppercase tracking-[0.08em] hover:border-green hover:bg-transparent hover:text-green sm:px-3.5"
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
