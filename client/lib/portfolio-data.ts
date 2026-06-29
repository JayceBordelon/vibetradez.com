import { serverFetch } from "@/lib/api";
import type { ClosedTrade, ClosedTradesResponse, EquityCurvePoint, EquityCurveResponse, Holding, HoldingsResponse, PortfolioResponse } from "@/types/portfolio";

/*
Server-side data access for the /holdings and /closed pages. These run in
RSC / generateMetadata (no browser), so they hit the trading server
directly via serverFetch and degrade to empty on any failure rather than
throwing a 500 page.
*/

export async function fetchHoldings(): Promise<Holding[]> {
  try {
    const data = await serverFetch<HoldingsResponse>("/api/portfolio/holdings");
    return data.holdings ?? [];
  } catch {
    return [];
  }
}

export async function fetchClosedTrades(): Promise<ClosedTrade[]> {
  try {
    const data = await serverFetch<ClosedTradesResponse>("/api/portfolio/closed");
    return data.trades ?? [];
  } catch {
    return [];
  }
}

// Live account equity for the landing hero. Returns null when the manager
// is disabled or the fetch fails, so callers can fall back to static copy.
export async function fetchAccountEquity(): Promise<number | null> {
  try {
    const data = await serverFetch<PortfolioResponse>("/api/portfolio");
    return data.enabled && data.equity > 0 ? data.equity : null;
  } catch {
    return null;
  }
}

// Daily account-equity series for the landing hero sparkline. Returns [] when
// the manager is disabled or the fetch fails (including at build time), so the
// hero degrades to a decorative curve instead of erroring.
export async function fetchEquityCurve(): Promise<EquityCurvePoint[]> {
  try {
    const data = await serverFetch<EquityCurveResponse>("/api/portfolio/equity-curve");
    return data.points ?? [];
  } catch {
    return [];
  }
}

// Detail lookups are by id alone: a holding's id (compacted symbol) and a
// closed trade's id (compacted symbol + close date) are each unique across
// stocks and options, so the kind never needs to be in the URL.
export async function findHolding(id: string): Promise<Holding | null> {
  const holdings = await fetchHoldings();
  return holdings.find((h) => h.id === id) ?? null;
}

export async function findClosedTrade(id: string): Promise<ClosedTrade | null> {
  const trades = await fetchClosedTrades();
  return trades.find((t) => t.id === id) ?? null;
}
