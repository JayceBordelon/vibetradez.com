"use client";

import { LogIn, LogOut } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import type { SessionUser } from "@/lib/api";
import { useSession } from "@/lib/session";
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
  const { user, loading } = useSession();

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
          {loading ? (
            <div className="h-8 w-20 rounded-md bg-muted/60" aria-hidden="true" />
          ) : user ? (
            <AccountMenu user={user} />
          ) : (
            onSubscribe && (
              <Button variant="outline" size="sm" onClick={onSubscribe} className="h-8 gap-1.5 px-2 text-xs sm:px-3 sm:text-sm" aria-label="Sign in or sign up">
                <LogIn className="h-3.5 w-3.5 sm:hidden" />
                <span className="hidden sm:inline">Sign in</span>
              </Button>
            )
          )}
        </div>
      </div>
    </header>
  );
}

function AccountMenu({ user }: { user: SessionUser }) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const { signOut } = useSession();

  useEffect(() => {
    if (!open) return;
    function onDocClick(e: MouseEvent) {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onDocClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDocClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const initials = (user.name || user.email).trim().charAt(0).toUpperCase() || "?";
  const [pictureFailed, setPictureFailed] = useState(false);
  // Google returns a =s96-c suffix; we display at 24-32px so a smaller
  // variant is plenty and avoids the bigger images' stricter rate limits.
  const avatarSrc = user.picture_url ? user.picture_url.replace(/=s\d+(-c)?$/, "=s64-c") : "";
  const showImage = Boolean(avatarSrc) && !pictureFailed;

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex h-8 w-8 shrink-0 cursor-pointer items-center justify-center overflow-hidden rounded-full border bg-background text-xs font-semibold transition-colors hover:bg-muted sm:w-auto sm:justify-start sm:gap-2 sm:overflow-visible sm:pl-1 sm:pr-3"
        aria-haspopup="menu"
        aria-expanded={open}
      >
        <span className="flex size-full shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted sm:size-6">
          {showImage ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={avatarSrc} alt="" referrerPolicy="no-referrer" onError={() => setPictureFailed(true)} className="h-full w-full object-cover" />
          ) : (
            <span className="text-[10px] font-extrabold">{initials}</span>
          )}
        </span>
        <span className="hidden max-w-[140px] truncate sm:inline">{user.email}</span>
      </button>
      {open && (
        <div role="menu" className="absolute right-0 top-full z-30 mt-2 w-56 overflow-hidden rounded-md border bg-popover shadow-lg">
          <div className="border-b px-3 py-2">
            <div className="truncate text-xs font-semibold">{user.name || "Signed in"}</div>
            <div className="truncate text-[11px] text-muted-foreground">{user.email}</div>
          </div>
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setOpen(false);
              void signOut();
            }}
            className="flex w-full cursor-pointer items-center gap-2 px-3 py-2 text-left text-xs font-semibold hover:bg-muted"
          >
            <LogOut className="h-3.5 w-3.5" />
            Sign out
          </button>
        </div>
      )}
    </div>
  );
}
