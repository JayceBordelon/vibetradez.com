import type { LiveQuote } from "@/hooks/use-live-quotes";
import type { Holding, PortfolioPosition } from "@/types/portfolio";

/*
Live re-pricing overlays: take the last server-reported book and replace
each position's mark with the latest streamed quote, recomputing market
value and unrealized P&L. Symbols line up by construction: equity
positions carry the ticker the stream publishes, options carry the OCC
symbol the stream publishes. Positions without a tick yet keep their
server values, so the overlay degrades to exactly the polled book.
*/

export function repriceHoldings(holdings: Holding[], quotes: Map<string, LiveQuote>): Holding[] {
  if (quotes.size === 0) return holdings;
  return holdings.map((h) => {
    const q = quotes.get(h.symbol);
    if (!q || !(q.mark > 0) || !(h.quantity > 0)) return h;
    const mult = h.kind === "option" ? 100 : 1;
    const market_value = h.quantity * q.mark * mult;
    return { ...h, market_value, unrealized_pnl: market_value - h.cost_basis };
  });
}

export function repricePositions(positions: PortfolioPosition[], quotes: Map<string, LiveQuote>): PortfolioPosition[] {
  if (quotes.size === 0) return positions;
  return positions.map((p) => {
    const q = quotes.get(p.symbol);
    if (!q || !(q.mark > 0) || !(p.quantity > 0)) return p;
    const mult = p.asset_type === "OPTION" ? 100 : 1;
    const market_value = p.quantity * q.mark * mult;
    return { ...p, market_value, unrealized_pnl: market_value - p.cost_basis };
  });
}
