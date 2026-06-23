"use client";

import { ChevronDown } from "lucide-react";
import { type ReactNode, useCallback, useLayoutEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";

/*
CollapsibleOutput clamps a tool body (or any long transcript block) to a fixed
height when it would otherwise run long, fading the cut edge and exposing a
"Show more" toggle. Short bodies render untouched and never grow a control.

The fade is a CSS alpha MASK, not a colored gradient overlay, so it works over
any background the wrapped body paints (the dark terminal pane, the green/red
order tickets, the plain page) without a color mismatch.

Measurement runs in useLayoutEffect (before paint) so a body that overflows is
clamped on first frame rather than flashing full height then snapping shut. A
ResizeObserver re-measures when the body reflows (font load, viewport change,
an inner table gaining rows).
*/

const COLLAPSED_MAX_PX = 320;
// A few px of slack so a body that's a hair over the cap isn't given a toggle
// that reveals almost nothing.
const OVERFLOW_SLACK_PX = 24;
const FADE_MASK = "linear-gradient(to bottom, #000 72%, transparent 100%)";

export function CollapsibleOutput({ children, className }: { children: ReactNode; className?: string }) {
  const innerRef = useRef<HTMLDivElement>(null);
  const [overflowing, setOverflowing] = useState(false);
  const [expanded, setExpanded] = useState(false);

  const measure = useCallback(() => {
    const el = innerRef.current;
    if (!el) return;
    setOverflowing(el.scrollHeight > COLLAPSED_MAX_PX + OVERFLOW_SLACK_PX);
  }, []);

  useLayoutEffect(() => {
    measure();
    const el = innerRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [measure]);

  const clamped = overflowing && !expanded;

  return (
    <div className={className}>
      <div
        ref={innerRef}
        className="overflow-hidden"
        style={
          clamped
            ? { maxHeight: COLLAPSED_MAX_PX, maskImage: FADE_MASK, WebkitMaskImage: FADE_MASK }
            : undefined
        }
      >
        {children}
      </div>
      {overflowing && (
        <button
          type="button"
          aria-expanded={expanded}
          onClick={() => setExpanded((o) => !o)}
          className="mt-1.5 inline-flex min-h-6 cursor-pointer items-center gap-1 rounded-md border border-border/60 px-2 py-0.5 font-mono text-[10px] font-semibold uppercase tracking-wider text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
        >
          <ChevronDown className={cn("h-3 w-3 transition-transform motion-reduce:transition-none", expanded && "rotate-180")} aria-hidden />
          {expanded ? "Show less" : "Show more"}
        </button>
      )}
    </div>
  );
}
