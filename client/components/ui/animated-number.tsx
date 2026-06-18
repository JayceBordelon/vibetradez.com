"use client";

import { useEffect, useRef, useState } from "react";

import { fmt, fmtMoney, fmtMoneyInt, fmtPnl, fmtPnlInt, fmtPrice } from "@/lib/format";

/*
AnimatedNumber renders a numeric readout that counts up or down to its new
value whenever a live update changes it, the same motion language as the
landing page's CountUp, applied to transitions instead of entrances.

Every change also signals its direction: the number phases green (up) or
red (down) for about a second while it counts. The transient breadcrumb
(a chip popping above the number with the signed difference, then fading)
is OPT-IN via the crumb prop: it lives on the headline stat strips only,
where a floating "+$155.47" reads as a heartbeat; sprayed across every
table cell it reads as confetti. The chip is absolutely positioned so it
never takes layout space in the number's container.

Behavior contract:
  - First render is INSTANT: a page load shows the real number, no 0-to-N
    theater outside the marketing page.
  - A value change tweens from the currently displayed number (so a
    retarget mid-flight never snaps back), easeOutCubic over ~700ms.
  - prefers-reduced-motion snaps immediately, with no flash or breadcrumb.
  - tabular-nums on the wrapper so digits don't jitter the layout.

The formatter is chosen by NAME, not passed as a function: server
components render these readouts too, and functions don't cross the RSC
boundary. Add new kinds to FORMATS as call sites need them.
*/

// signedPct formats a percent with a leading sign, but decides the sign from
// the value AFTER rounding to the displayed precision: a magnitude that rounds
// to zero renders a neutral "0.0%"/"0.00%" instead of "-0.0%" (a minus on a
// number that displays as zero) or "+0.0%" (a win claimed on a flat position).
function signedPct(n: number, d: number): string {
  const body = fmt(n, d); // toFixed already carries a "-" for negatives
  const rounded = Number(body); // "-0.00" -> -0, which === 0
  if (rounded === 0) return `${fmt(0, d)}%`;
  return `${rounded > 0 ? "+" : ""}${body}%`;
}

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
  usdSigned: (n: number) => {
    // Sign comes from the rounded magnitude, so a value that rounds to $0
    // reads as a neutral "$0", never a self-contradicting "-$0"/"+$0".
    const r = Math.round(n);
    if (r === 0) return "$0";
    return `${r < 0 ? "-" : "+"}$${Math.abs(r).toLocaleString("en-US")}`;
  },
  /** +4.3% (one decimal, signed) */
  pctSigned1: (n: number) => signedPct(n, 1),
  /** +3.77% (two decimals, signed) */
  pctSigned2: (n: number) => signedPct(n, 2),
  /** 68% (whole, unsigned) */
  pct0: (n: number) => `${fmt(n, 0)}%`,
  /** -19.5% (one decimal, sign only when negative) */
  pct1: (n: number) => `${fmt(n, 1)}%`,
} as const;

export type AnimatedNumberKind = keyof typeof FORMATS;

// The breadcrumb shows the signed step the value just took: dollar kinds
// get signed cents, percent kinds get signed percentage points.
function fmtDiff(kind: AnimatedNumberKind, diff: number): string {
  const sign = diff < 0 ? "-" : "+";
  if (kind.startsWith("pct")) {
    return `${sign}${Math.abs(diff).toFixed(2)}%`;
  }
  const abs = Math.abs(diff);
  const body = abs.toLocaleString("en-US", { minimumFractionDigits: abs < 1000 ? 2 : 0, maximumFractionDigits: abs < 1000 ? 2 : 0 });
  return `${sign}$${body}`;
}

const DURATION_MS = 700;
const FLASH_MS = 1000;
const CRUMB_MS = 1100;

export function AnimatedNumber({
  value,
  kind,
  className,
  crumb: crumbEnabled = false,
  neutral = false,
}: {
  value: number;
  kind: AnimatedNumberKind;
  className?: string;
  crumb?: boolean;
  /**
  neutral marks a number whose direction carries no good/bad meaning (an
  allocation percent, a count): no green/red flash, and its breadcrumb
  renders in muted gray instead of P&L colors.
  */
  neutral?: boolean;
}) {
  // displayed is what's on screen; it chases `value` through the tween.
  const [displayed, setDisplayed] = useState(value);
  const displayedRef = useRef(value);
  displayedRef.current = displayed;
  const [flash, setFlash] = useState<"up" | "down" | null>(null);
  const [crumb, setCrumb] = useState<{ id: number; text: string; up: boolean } | null>(null);
  const mounted = useRef(false);
  const prevTarget = useRef(value);
  const raf = useRef(0);
  const crumbSeq = useRef(0);
  const flashTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const crumbTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    if (!mounted.current) {
      // First paint already showed the real value via useState(value).
      mounted.current = true;
      prevTarget.current = value;
      return;
    }
    const from = prevTarget.current;
    prevTarget.current = value;
    if (value === from) return;
    if (typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      setDisplayed(value);
      return;
    }

    // Direction cues: phase the number's color and float the signed step.
    const up = value > from;
    if (!neutral) {
      setFlash(up ? "up" : "down");
      clearTimeout(flashTimer.current);
      flashTimer.current = setTimeout(() => setFlash(null), FLASH_MS);
    }
    if (crumbEnabled) {
      crumbSeq.current += 1;
      setCrumb({ id: crumbSeq.current, text: fmtDiff(kind, value - from), up });
      clearTimeout(crumbTimer.current);
      crumbTimer.current = setTimeout(() => setCrumb(null), CRUMB_MS);
    }

    const start = performance.now();
    const tweenFrom = displayedRef.current;
    const step = (now: number) => {
      const t = Math.min(1, (now - start) / DURATION_MS);
      const eased = 1 - (1 - t) ** 3; // easeOutCubic
      setDisplayed(t < 1 ? tweenFrom + (value - tweenFrom) * eased : value);
      if (t < 1) raf.current = requestAnimationFrame(step);
    };
    cancelAnimationFrame(raf.current);
    raf.current = requestAnimationFrame(step);
    return () => cancelAnimationFrame(raf.current);
  }, [value, kind, crumbEnabled, neutral]);

  // Timers die with the component so a detached crumb can't set state.
  useEffect(
    () => () => {
      clearTimeout(flashTimer.current);
      clearTimeout(crumbTimer.current);
      cancelAnimationFrame(raf.current);
    },
    [],
  );

  return (
    <span className={className ? `relative inline-block tabular-nums ${className}` : "relative inline-block tabular-nums"}>
      <span
        style={{
          color: flash === "up" ? "var(--green)" : flash === "down" ? "var(--red)" : undefined,
          transition: "color 250ms ease",
        }}
      >
        {FORMATS[kind](displayed)}
      </span>
      {crumb && (
        <span
          key={crumb.id}
          aria-hidden
          className="animate-vt-crumb pointer-events-none absolute -top-3 left-1/2 z-10 -translate-x-1/2 rounded-full px-1.5 font-mono text-[10px] font-bold whitespace-nowrap"
          style={
            neutral
              ? { color: "var(--muted-foreground)", background: "var(--muted)" }
              : { color: crumb.up ? "var(--green)" : "var(--red)", background: crumb.up ? "var(--green-bg)" : "var(--red-bg)" }
          }
        >
          {crumb.text}
        </span>
      )}
    </span>
  );
}
