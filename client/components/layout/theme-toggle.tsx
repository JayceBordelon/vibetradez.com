"use client";

import { Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { useEffect, useState } from "react";

import { cn } from "@/lib/utils";

/*
Light/dark toggle. next-themes resolves the active theme (system or
explicit); clicking flips to the opposite explicit theme. Mounted-guarded so
the icon doesn't flash the wrong glyph during hydration.
*/
export function ThemeToggle({ className }: { className?: string }) {
  const { resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  // Gate the resolved theme on `mounted` so the server render and the first
  // client render agree: pre-mount, isDark is always false (server can't know
  // the theme), so aria-label, onClick, and icon all match until the effect
  // runs. Without this, only the icon was guarded and aria-label mismatched.
  const isDark = mounted && resolvedTheme === "dark";
  return (
    <button
      type="button"
      aria-label={isDark ? "Switch to light mode" : "Switch to dark mode"}
      onClick={() => setTheme(isDark ? "light" : "dark")}
      className={cn(
        "inline-flex h-9 w-9 shrink-0 cursor-pointer items-center justify-center rounded-full border border-border text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
        className,
      )}
    >
      {mounted ? isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" /> : <Sun className="h-4 w-4 opacity-0" />}
    </button>
  );
}
