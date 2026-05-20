import type { ApiResponse, DashboardResponse, MarketStatus, WeekResponse } from "@/types/trade";

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
"Unexpected token < in JSON" SyntaxError — callers had no way to
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
  getTrades: (date?: string) => clientFetch<DashboardResponse>(date ? `/api/trades/today?date=${date}` : "/api/trades/today"),

  getWeekTrades: (start: string, end: string) => clientFetch<WeekResponse>(`/api/trades/week?start=${start}&end=${end}`),

  /*
  Live quotes are now an SSE stream at /api/quotes/stream — see
  hooks/use-quote-stream.ts. /api/market/status is the cheap polled
  endpoint that tells the dashboard whether to open the stream or
  render the market-closed page.
  */
  getMarketStatus: () => clientFetch<MarketStatus>("/api/market/status"),

  me: () => clientFetch<MeResponse>("/api/me"),

  // Note: /auth/logout lives under /auth/*, not /api/*, so no X-VT-Source header.
  logout: () => authFetch<ApiResponse>("/auth/logout", { method: "POST" }),

  /*
  Programmatic unsubscribe path — requires the HMAC token minted server-side
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
