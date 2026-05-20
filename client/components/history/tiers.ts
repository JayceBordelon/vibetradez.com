export const TIER_KEYS = ["top1", "top3", "top10"] as const;
export type TierKey = (typeof TIER_KEYS)[number];

export const TIER_LABELS: Record<TierKey, string> = {
  top1: "Top 1",
  top3: "Top 3",
  top10: "Top 10",
};

export const TIER_SUBLABELS: Record<TierKey, string> = {
  top1: "Highest-conviction pick",
  top3: "Auto-execution basket",
  top10: "All ranked picks",
};

/**
Tier color is decoupled from the green/red P&L palette so a single
chart can encode both tier (color) and sign (Y-axis side) without
collision. Top 1 takes the saturated teal accent because it's the
hero series; Top 3 takes the warm orange accent; Top 10 sits as the
neutral zinc backdrop.
*/
export const TIER_COLORS: Record<TierKey, string> = {
  top1: "var(--gpt)",
  top3: "var(--claude)",
  top10: "var(--chart-3)",
};

/**
TIER_PILL_COLORS is a parallel palette for rank pills (white text on a
filled pill on the history rows). The chart palette above uses light
zinc for Top 10 so the bar fills read at low saturation — but white
text on that light zinc was failing WCAG AA contrast (~2.4:1). The
pill uses a dedicated darker token (--tier-pill-top10) so the chart
look doesn't change. Top 1 and Top 3 keep their brand colors which
already pass.
*/
export const TIER_PILL_COLORS: Record<TierKey, string> = {
  top1: "var(--gpt)",
  top3: "var(--claude)",
  top10: "var(--tier-pill-top10)",
};

export function inTier(rank: number, tier: TierKey): boolean {
  if (tier === "top1") return rank === 1;
  if (tier === "top3") return rank <= 3;
  return true;
}
