import type { ApiResponse, ChartParams, ChartResponse, DashboardResponse, LiveQuotesResponse, ModelComparisonResponse, WeekResponse } from "@/types/trade";

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
    ...options,
    headers: { ...HEADERS, ...options?.headers },
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

  subscribe: (email: string, name: string) =>
    clientFetch<ApiResponse>("/api/subscribe", {
      method: "POST",
      headers: { "Content-Type": "application/json", ...HEADERS },
      body: JSON.stringify({ email, name }),
    }),

  unsubscribe: (email: string) =>
    clientFetch<ApiResponse>("/api/unsubscribe", {
      method: "POST",
      headers: { "Content-Type": "application/json", ...HEADERS },
      body: JSON.stringify({ email }),
    }),
};
