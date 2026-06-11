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

  const isDark = resolvedTheme === "dark";
  return (
    <button
      type="button"
      aria-label={isDark ? "Switch to light mode" : "Switch to dark mode"}
      onClick={() => setTheme(isDark ? "light" : "dark")}
      className={cn(
        "inline-flex h-9 w-9 shrink-0 cursor-pointer items-center justify-center border border-border text-muted-foreground transition-colors hover:border-green hover:text-green",
        className,
      )}
    >
      {mounted ? isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" /> : <Sun className="h-4 w-4 opacity-0" />}
    </button>
  );
}
