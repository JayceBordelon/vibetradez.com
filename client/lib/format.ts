export function fmt(n: number, d = 2): string {
  return Number(n).toFixed(d);
}

export function fmtPct(n: number): string {
  return `${n > 0 ? "+" : ""}${fmt(n, 0)}%`;
}

export function fmtPctDec(n: number): string {
  return `${n > 0 ? "+" : ""}${fmt(n, 1)}%`;
}

export function fmtMoney(n: number): string {
  return `$${fmt(n)}`;
}

export function fmtMoneyInt(n: number): string {
  return `$${fmt(Math.abs(n), 0)}`;
}

export function fmtPnlInt(n: number): string {
  if (n > 0) return `+$${fmt(n, 0)}`;
  if (n < 0) return `-$${fmt(Math.abs(n), 0)}`;
  return "$0";
}

export function pnlColor(v: number): string {
  if (v > 0) return "text-green";
  if (v < 0) return "text-red";
  return "text-muted-foreground";
}

/**
 * Maps a percentage (0-100) to a CSS color along the red → green hue
 * range, so a stat like "win rate" or "agreement rate" can be shaded
 * continuously by quality instead of snapping to one of three discrete
 * tone buckets. Pass the result to a StatCard's `valueColor` prop.
 */
export function percentHueColor(pct: number): string {
  const clamped = Math.max(0, Math.min(100, pct));
  /**
  Hue 0 = red, 145 ≈ project's green hue. Linear in between.
  Lightness comes from --percent-hue-l so light/dark themes can each
  pick a value that meets WCAG contrast against their background.
  */
  const hue = (clamped / 100) * 145;
  return `hsl(${hue.toFixed(0)} 70% var(--percent-hue-l))`;
}
