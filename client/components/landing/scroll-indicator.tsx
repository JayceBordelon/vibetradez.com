"use client";

import { ChevronDown } from "lucide-react";
import { useEffect, useState } from "react";

import { cn } from "@/lib/utils";

/*
A "scroll down" cue pinned to the bottom of the full-height hero. It fades out
the moment the visitor actually scrolls (and comes back if they return to the
very top), so it nudges without lingering. Decorative, hidden from a11y tree.
*/
export function ScrollIndicator() {
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8);
    onScroll(); // sync initial state (e.g. on reload mid-page)
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <div
      aria-hidden
      className={cn(
        "pointer-events-none absolute inset-x-0 bottom-6 z-10 flex flex-col items-center gap-1.5 text-muted-foreground transition-opacity duration-500 sm:bottom-8",
        scrolled ? "opacity-0" : "opacity-100",
      )}
    >
      <span className="text-[10px] font-semibold uppercase tracking-[0.2em]">Scroll</span>
      <ChevronDown className="h-5 w-5 animate-bounce" />
    </div>
  );
}
