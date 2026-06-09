import type { ClosedTradesResponse, EquityCurveResponse, HoldingsResponse, PortfolioResponse, PositionHistoryResponse, PriceHistoryResponse } from "@/types/portfolio";
import type { ApiResponse, MarketStatus, TranscriptResponse } from "@/types/trade";

export interface SessionUser {
  id: number;
  email: string;
  name: string;
  picture_url: string;
}

export interface MeResponse {
  user: SessionUser | null;
}

const HEADERS: Record<string, string> = {
  "X-VT-Source": "dashboard",
};

const SERVER_API_BASE = process.env.API_URL || "http://trading-server:8080";

/*
ApiError carries enough structure for callers to render distinct UI
states for transport failures vs server errors vs malformed bodies.
Status 0 means we never reached the server (network blip, parse
error). The original Response is exposed for callers who want
headers, redirected URLs, etc.
*/
export class ApiError extends Error {
  status: number;
  statusText: string;
  body: string;
  url: string;
  constructor(message: string, init: { status: number; statusText: string; body: string; url: string }) {
    super(message);
    this.name = "ApiError";
    this.status = init.status;
    this.statusText = init.statusText;
    this.body = init.body;
    this.url = init.url;
  }
}

/*
parseResponse centralizes the "did we actually get JSON back from a
2xx" check. Without this, a 502 from Traefik or a 401 redirect to an
HTML login page would throw inside res.json() with a useless
"Unexpected token < in JSON" SyntaxError, callers had no way to
distinguish that from a real malformed body. Now every non-2xx
throws ApiError, and a body that fails to parse as JSON throws
ApiError too (rather than the obscure SyntaxError).
*/
async function parseResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let body = "";
    try {
      body = await res.text();
    } catch {
      // ignore read failures; body stays empty
    }
    throw new ApiError(`HTTP ${res.status} ${res.statusText} for ${res.url}`, {
      status: res.status,
      statusText: res.statusText,
      body,
      url: res.url,
    });
  }
  try {
    return (await res.json()) as T;
  } catch (err) {
    throw new ApiError(`malformed JSON body from ${res.url}: ${(err as Error).message}`, {
      status: res.status,
      statusText: res.statusText,
      body: "",
      url: res.url,
    });
  }
}

export async function serverFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${SERVER_API_BASE}${path}`, {
    ...options,
    headers: { ...HEADERS, ...options?.headers },
  });
  return parseResponse<T>(res);
}

export async function clientFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    ...options,
    headers: { ...HEADERS, ...options?.headers },
  });
  return parseResponse<T>(res);
}

async function authFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    ...options,
  });
  return parseResponse<T>(res);
}

export const api = {
  /*
  v2 portfolio manager. getPortfolio returns the live book (positions,
  cash, equity) plus today's agent stance + moves for the single
  brokerage account. getEquityCurve returns the daily equity-vs-SPY
  series for the chart.
  */
  getPortfolio: (date?: string) => clientFetch<PortfolioResponse>(date ? `/api/portfolio?date=${date}` : "/api/portfolio"),

  getEquityCurve: (start?: string, end?: string) => {
    const qs = new URLSearchParams();
    if (start) qs.set("start", start);
    if (end) qs.set("end", end);
    const suffix = qs.toString() ? `?${qs.toString()}` : "";
    return clientFetch<EquityCurveResponse>(`/api/portfolio/equity-curve${suffix}`);
  },

  // /holdings: every open position. /closed: every completed round trip.
  getHoldings: () => clientFetch<HoldingsResponse>("/api/portfolio/holdings"),
  getClosedTrades: () => clientFetch<ClosedTradesResponse>("/api/portfolio/closed"),

  /*
  Trade-detail chart series. getPositionHistory returns a position's
  per-day snapshot values (the held period exactly); getPriceHistory
  returns daily closes for an equity symbol, empty when the market-data
  source is unreachable, so callers must degrade gracefully.
  */
  getPositionHistory: (symbol: string) => clientFetch<PositionHistoryResponse>(`/api/portfolio/position-history?symbol=${encodeURIComponent(symbol)}`),
  getPriceHistory: (symbol: string, start?: string, end?: string) => {
    const qs = new URLSearchParams({ symbol });
    if (start) qs.set("start", start);
    if (end) qs.set("end", end);
    return clientFetch<PriceHistoryResponse>(`/api/price-history?${qs.toString()}`);
  },

  /*
  Daily model-reasoning transcript. kind is "selection" (the 9:25
  picker conversation) or "execution" (the 9:30 at-open agent). Returns
  available:false with empty events when nothing was captured for the
  date.
  */
  getTranscript: (date: string, kind: "selection" | "execution" | "portfolio") =>
    clientFetch<TranscriptResponse>(`/api/transcript?date=${encodeURIComponent(date)}&kind=${kind}`),

  // /api/market/status: the cheap polled endpoint that tells the dashboard
  // whether the market is open (drives the auto-refresh cadence).
  getMarketStatus: () => clientFetch<MarketStatus>("/api/market/status"),

  me: () => clientFetch<MeResponse>("/api/me"),

  // Note: /auth/logout lives under /auth/*, not /api/*, so no X-VT-Source header.
  logout: () => authFetch<ApiResponse>("/auth/logout", { method: "POST" }),

  /*
  Programmatic unsubscribe path, requires the HMAC token minted server-side
  and emailed to the subscriber. No call site uses it from the website
  today (UnsubscribeForm now points users to the link in their email),
  but kept as a thin wrapper so any future tool that already holds a
  valid token can call it.
  */
  unsubscribe: (email: string, token: string) =>
    clientFetch<ApiResponse>("/api/unsubscribe", {
      method: "POST",
      headers: { "Content-Type": "application/json", ...HEADERS },
      body: JSON.stringify({ email, token }),
    }),
};
