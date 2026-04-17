import type { ApiResponse, ChartParams, ChartResponse, DashboardResponse, LiveQuotesResponse, ModelComparisonResponse, WeekResponse } from "@/types/trade";

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

export async function serverFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${SERVER_API_BASE}${path}`, {
    ...options,
    headers: { ...HEADERS, ...options?.headers },
  });
  return res.json();
}

export async function clientFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    ...options,
    headers: { ...HEADERS, ...options?.headers },
  });
  return res.json();
}

async function authFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    ...options,
  });
  return res.json();
}

export const api = {
  getTradeDates: (limit = 30) => clientFetch<{ dates: string[] }>(`/api/trades/dates?limit=${limit}`),

  getTrades: (date?: string) => clientFetch<DashboardResponse>(date ? `/api/trades/today?date=${date}` : "/api/trades/today"),

  getWeekTrades: (start: string, end: string) => clientFetch<WeekResponse>(`/api/trades/week?start=${start}&end=${end}`),

  getLiveQuotes: () => clientFetch<LiveQuotesResponse>("/api/quotes/live"),

  getModelComparison: (range: "week" | "month" | "year" | "all" = "all") => clientFetch<ModelComparisonResponse>(`/api/model-comparison?range=${range}`),

  getChartData: (symbol: string, params: ChartParams) =>
    clientFetch<ChartResponse>(`/api/chart/${symbol}?periodType=${params.ptype}&period=${params.period}&frequencyType=${params.ftype}&frequency=${params.freq}`),

  me: () => clientFetch<MeResponse>("/api/me"),

  // Note: /auth/logout lives under /auth/*, not /api/*, so no X-VT-Source header.
  logout: () => authFetch<ApiResponse>("/auth/logout", { method: "POST" }),

  unsubscribe: (email: string) =>
    clientFetch<ApiResponse>("/api/unsubscribe", {
      method: "POST",
      headers: { "Content-Type": "application/json", ...HEADERS },
      body: JSON.stringify({ email }),
    }),
};
