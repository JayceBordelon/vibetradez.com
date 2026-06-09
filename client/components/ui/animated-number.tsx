"use client";

import { useEffect, useRef, useState } from "react";

import { fmt, fmtMoney, fmtMoneyInt, fmtPnl, fmtPnlInt, fmtPrice } from "@/lib/format";

/*
AnimatedNumber renders a numeric readout that counts up or down to its new
value whenever a live update changes it, the same motion language as the
landing page's CountUp, applied to transitions instead of entrances.

Behavior contract:
  - First render is INSTANT: a page load shows the real number, no 0-to-N
    theater outside the marketing page.
  - A value change tweens from the currently displayed number (so a
    retarget mid-flight never snaps back), easeOutCubic over ~700ms:
    fast enough to read as a tick, slow enough to register direction.
  - prefers-reduced-motion snaps immediately.
  - tabular-nums on the wrapper so digits don't jitter the layout.

The formatter is chosen by NAME, not passed as a function: server
components render these readouts too, and functions don't cross the RSC
boundary. Add new kinds to FORMATS as call sites need them.
*/

const FORMATS = {
  /** $1,480.42 */
  money: fmtMoney,
  /** $1,480 (unsigned, whole dollars) */
  moneyInt: fmtMoneyInt,
  /** +$206.45 / -$99.00, cents */
  pnl: fmtPnl,
  /** +$206 / -$99, whole dollars */
  pnlInt: fmtPnlInt,
  /** $79.50 / $1,234: cents only when present */
  price: fmtPrice,
  /** +$1,204 / -$86: signed compact dollars (chart headline style) */
  usdSigned: (n: number) => `${n < 0 ? "-" : "+"}$${Math.abs(n).toLocaleString("en-US", { maximumFractionDigits: 0 })}`,
  /** +4.3% (one decimal, signed) */
  pctSigned1: (n: number) => `${n >= 0 ? "+" : ""}${fmt(n, 1)}%`,
  /** +3.77% (two decimals, signed) */
  pctSigned2: (n: number) => `${n >= 0 ? "+" : ""}${fmt(n, 2)}%`,
  /** 68% (whole, unsigned) */
  pct0: (n: number) => `${fmt(n, 0)}%`,
  /** -19.5% (one decimal, sign only when negative) */
  pct1: (n: number) => `${fmt(n, 1)}%`,
} as const;

export type AnimatedNumberKind = keyof typeof FORMATS;

const DURATION_MS = 700;

export function AnimatedNumber({ value, kind, className }: { value: number; kind: AnimatedNumberKind; className?: string }) {
  // displayed is what's on screen; it chases `value` through the tween.
  const [displayed, setDisplayed] = useState(value);
  const displayedRef = useRef(value);
  displayedRef.current = displayed;
  const mounted = useRef(false);
  const raf = useRef(0);

  useEffect(() => {
    if (!mounted.current) {
      // First paint already showed the real value via useState(value).
      mounted.current = true;
      return;
    }
    if (value === displayedRef.current) return;
    if (typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      setDisplayed(value);
      return;
    }
    const from = displayedRef.current;
    const start = performance.now();
    const step = (now: number) => {
      const t = Math.min(1, (now - start) / DURATION_MS);
      const eased = 1 - (1 - t) ** 3; // easeOutCubic
      setDisplayed(t < 1 ? from + (value - from) * eased : value);
      if (t < 1) raf.current = requestAnimationFrame(step);
    };
    cancelAnimationFrame(raf.current);
    raf.current = requestAnimationFrame(step);
    return () => cancelAnimationFrame(raf.current);
  }, [value]);

  return <span className={className ? `tabular-nums ${className}` : "tabular-nums"}>{FORMATS[kind](displayed)}</span>;
}
