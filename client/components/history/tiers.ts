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

export function inTier(rank: number, tier: TierKey): boolean {
  if (tier === "top1") return rank === 1;
  if (tier === "top3") return rank <= 3;
  return true;
}
