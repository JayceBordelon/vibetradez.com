"use client";

import { Check, Loader2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { cn } from "@/lib/utils";

/*
The app-wide loading placeholder. A calm card that names what it's fetching
and checks each step off as the data lands, rather than a skeleton blob.
All motion runs client-side after mount, so SSR/first paint is a clean
static frame that hydrates without mismatch. Under prefers-reduced-motion
the cascade jumps to its finished state and the spinner holds still.

The prop names (command, lines) are kept from the previous loader so every
call site works unchanged; `command` now reads as a plain title.
*/

export function TerminalLoader({
  command,
  lines,
  className,
  minHeightClass = "min-h-[55vh]",
  compact = false,
}: {
  command: string;
  lines: string[];
  className?: string;
  minHeightClass?: string;
  compact?: boolean;
}) {
  const total = lines.length;
  const [step, setStep] = useState(0);
  const [reduced, setReduced] = useState(false);
  const stepRef = useRef(0);
  stepRef.current = step;

  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      setReduced(true);
      setStep(total);
      return;
    }
    const id = setInterval(() => {
      setStep((s) => (s >= total ? 0 : s + 1));
    }, 720);
    return () => clearInterval(id);
  }, [total]);

  const ratio = total === 0 ? 1 : Math.min(1, step / total);

  return (
    <div className={cn("flex w-full items-center justify-center", minHeightClass, className)} role="status" aria-busy="true">
      <div className={cn("vt-shimmer w-full rounded-2xl border border-border bg-card shadow-sm", compact ? "max-w-sm p-5" : "max-w-md p-6")}>
        <div className="flex items-center gap-2.5">
          <Loader2 className={cn("h-4 w-4 shrink-0 text-primary", !reduced && "animate-spin")} aria-hidden />
          <span className={cn("font-medium text-foreground", compact ? "text-sm" : "text-[15px]")}>{command}…</span>
        </div>

        <ul className={cn("space-y-2", compact ? "mt-4" : "mt-5")}>
          {lines.map((line, i) => {
            const done = reduced || i < step;
            const active = !reduced && i === step;
            return (
              <li
                key={i}
                className={cn("flex items-center gap-2.5 text-sm transition-colors duration-300", done ? "text-muted-foreground" : active ? "text-foreground" : "text-muted-foreground/45")}
              >
                <span aria-hidden className="inline-flex h-4 w-4 shrink-0 items-center justify-center">
                  {done ? (
                    <span className="flex h-4 w-4 items-center justify-center rounded-full bg-green-bg">
                      <Check className="h-2.5 w-2.5 text-green" strokeWidth={3} />
                    </span>
                  ) : active ? (
                    <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-primary" />
                  ) : (
                    <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/30" />
                  )}
                </span>
                <span className="min-w-0 truncate">{line}</span>
              </li>
            );
          })}
        </ul>

        <div className="mt-5 h-1 w-full overflow-hidden rounded-full bg-foreground/[0.06]" aria-hidden>
          <div className="h-full rounded-full bg-primary/70 transition-[width] duration-500 ease-out" style={{ width: `${Math.round(ratio * 100)}%` }} />
        </div>
        <span className="sr-only">Loading</span>
      </div>
    </div>
  );
}
