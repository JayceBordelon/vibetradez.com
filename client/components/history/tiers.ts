export const TIER_KEYS = ["top1", "top2", "top3"] as const;
export type TierKey = (typeof TIER_KEYS)[number];

export const TIER_LABELS: Record<TierKey, string> = {
  top1: "Top 1",
  top2: "Top 2",
  top3: "Top 3",
};

export const TIER_SUBLABELS: Record<TierKey, string> = {
  top1: "Highest-conviction pick",
  top2: "Top two by conviction",
  top3: "Full auto-execution basket",
};

/**
Tier color is decoupled from the green/red P&L palette so a single
chart can encode both tier (color) and sign (Y-axis side) without
collision. Top 1 is the hero series; Top 2 and Top 3 are the
nested universes around it.
*/
export const TIER_COLORS: Record<TierKey, string> = {
  top1: "var(--brand-green)",
  top2: "var(--chart-2)",
  top3: "var(--claude)",
};

/**
TIER_PILL_COLORS is a parallel palette for rank pills (white text on
a filled pill on the history rows). All three pass WCAG AA against
white text in the brand palette.
*/
export const TIER_PILL_COLORS: Record<TierKey, string> = {
  top1: "var(--brand-green)",
  top2: "var(--chart-2)",
  top3: "var(--claude)",
};

export function inTier(rank: number, tier: TierKey): boolean {
  if (tier === "top1") return rank === 1;
  if (tier === "top2") return rank <= 2;
  return rank <= 3;
}
