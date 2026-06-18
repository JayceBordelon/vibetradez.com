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

// A streamed mark older than this is treated as stale and ignored, so the
// overlay falls back to the freshly-polled server book instead of freezing
// the last-seen tick on screen after the stream goes quiet (market close, a
// trading halt, or a dropped SSE connection). Comfortably longer than the
// 60s book poll, so a quietly-ticking position during regular hours is never
// dropped — only marks that have genuinely stopped updating expire.
const QUOTE_TTL_MS = 120_000;

function freshQuote(q: LiveQuote | undefined, now: number): q is LiveQuote {
  return q !== undefined && q.mark > 0 && now - q.ts < QUOTE_TTL_MS;
}

// freshMark reads a single symbol's live mark, or null when there's no quote
// or the last one has gone stale. Used for standalone reads (e.g. the SPY
// mark that overlays the equity-curve's today point).
export function freshMark(quotes: Map<string, LiveQuote>, symbol: string): number | null {
  const q = quotes.get(symbol);
  return freshQuote(q, Date.now()) ? q.mark : null;
}

export function repriceHoldings(holdings: Holding[], quotes: Map<string, LiveQuote>): Holding[] {
  if (quotes.size === 0) return holdings;
  const now = Date.now();
  return holdings.map((h) => {
    const q = quotes.get(h.symbol);
    if (!freshQuote(q, now) || !(h.quantity > 0)) return h;
    const mult = h.kind === "option" ? 100 : 1;
    const market_value = h.quantity * q.mark * mult;
    return { ...h, market_value, unrealized_pnl: market_value - h.cost_basis };
  });
}

export function repricePositions(positions: PortfolioPosition[], quotes: Map<string, LiveQuote>): PortfolioPosition[] {
  if (quotes.size === 0) return positions;
  const now = Date.now();
  return positions.map((p) => {
    const q = quotes.get(p.symbol);
    if (!freshQuote(q, now) || !(p.quantity > 0)) return p;
    const mult = p.asset_type === "OPTION" ? 100 : 1;
    const market_value = p.quantity * q.mark * mult;
    return { ...p, market_value, unrealized_pnl: market_value - p.cost_basis };
  });
}
