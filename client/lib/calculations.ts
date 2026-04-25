import type { Trade } from "@/types/trade";

export function calcMoneyness(trade: Trade) {
  const s = trade.current_price;
  const k = trade.strike_price;
  if (s === 0 || k === 0) return { pct: 0, label: "ATM", variant: "outline" as const };

  const pct = trade.contract_type === "CALL" ? ((s - k) / k) * 100 : ((k - s) / k) * 100;

  if (Math.abs(pct) < 1) return { pct, label: "ATM", variant: "outline" as const };
  if (pct > 0)
    return {
      pct,
      label: `${Math.abs(pct).toFixed(1)}% ITM`,
      variant: "default" as const,
    };
  return {
    pct,
    label: `${Math.abs(pct).toFixed(1)}% OTM`,
    variant: "destructive" as const,
  };
}

export function calcBreakeven(trade: Trade): number {
  return trade.contract_type === "CALL" ? trade.strike_price + trade.estimated_price : trade.strike_price - trade.estimated_price;
}

export function calcMaxLoss(trade: Trade): number {
  return trade.estimated_price * 100;
}

export function sentimentLabel(score: number): string {
  if (score > 0.3) return "Bullish";
  if (score < -0.3) return "Bearish";
  return "Neutral";
}

export function sentimentColor(score: number): string {
  if (score > 0.3) return "text-green";
  if (score < -0.3) return "text-red";
  return "text-muted-foreground";
}
