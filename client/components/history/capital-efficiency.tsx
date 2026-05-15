import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { fmtMoneyInt, fmtPctDec, fmtPnlInt } from "@/lib/format";

import type { TierAggregate } from "./history-shell";
import { TIER_COLORS, TIER_KEYS, TIER_LABELS, TIER_SUBLABELS, type TierKey } from "./tiers";

interface CapitalEfficiencyProps {
  tiers: Record<TierKey, TierAggregate>;
}

function signTone(v: number): "positive" | "negative" | "neutral" {
  if (v > 0) return "positive";
  if (v < 0) return "negative";
  return "neutral";
}

function TierBlock({ tier, agg }: { tier: TierKey; agg: TierAggregate }) {
  const accent = TIER_COLORS[tier];
  return (
    <div className="min-w-0">
      <div className="mb-2 flex items-center gap-2">
        <span className="h-2 w-2 rounded-full" style={{ backgroundColor: accent }} aria-hidden />
        <span className="text-sm font-semibold">{TIER_LABELS[tier]}</span>
        <span className="text-[11px] text-muted-foreground">{TIER_SUBLABELS[tier]}</span>
      </div>
      <StatStrip cols={4}>
        <Stat label="Deployed" value={fmtMoneyInt(agg.totalInvested)} />
        <Stat label="Returned" value={fmtMoneyInt(agg.totalReturn)} />
        <Stat label="Net P&L" value={fmtPnlInt(agg.totalPnl)} tone={signTone(agg.totalPnl)} />
        <Stat label="ROC" value={fmtPctDec(agg.roc)} tone={signTone(agg.roc)} />
      </StatStrip>
    </div>
  );
}

export function CapitalEfficiency({ tiers }: CapitalEfficiencyProps) {
  return (
    <div className="space-y-6">
      {TIER_KEYS.map((tier) => (
        <TierBlock key={tier} tier={tier} agg={tiers[tier]} />
      ))}
    </div>
  );
}
