export interface Trade {
  symbol: string;
  contract_type: "CALL" | "PUT";
  // Resolved at open (null pre-resolution). The pre-bell picker stores
  // the symbol + contract intent only; the 9:30:00 ET at-open agent
  // picks the actual strike + expiration + limit price from the live
  // chain and stamps them onto the trade row when it fires the order.
  // Cards render a "Finding contracts..." placeholder while these are
  // null. Remain null forever for candidates the agent chose to skip.
  strike_price: number | null;
  expiration: string | null;
  dte: number | null;
  estimated_price: number | null;
  stop_loss: number | null;
  thesis: string;
  sentiment_score: number;
  current_price: number;
  target_price: number;
  risk_level: "LOW" | "MEDIUM" | "HIGH";
  catalyst: string;
  mention_count: number;
  rank: number;
  score: number;
  rationale: string;
  model: string;
  // Contract intent set by the picker. target_otm_pct is signed-OTM in
  // percent (sign is implicit from contract_type); min_dte is the
  // minimum acceptable DTE the executor uses to walk the chain.
  target_otm_pct: number;
  min_dte: number;
}

export interface TradeSummary {
  symbol: string;
  contract_type: string;
  strike_price: number;
  expiration: string;
  entry_price: number;
  closing_price: number;
  stock_open: number;
  stock_close: number;
  notes: string;
}

export interface DashboardTrade {
  trade: Trade;
  summary: TradeSummary | null;
}

/**
Execution surfaces a position taken on a trade via the auto-execution
pipeline (paper or live). Mode is always rendered in the badge so paper
is never mistaken for a real position. Server omits this field entirely
when no qualifying pick converted to an execution that day.
*/
export interface Execution {
  mode: "paper" | "live";
  state: "submitted" | "holding" | "closed" | "close_failed" | "failed" | "canceled" | "skipped";
  symbol: string;
  contract_type: string;
  strike_price: number;
  open_price: number;
  close_price: number;
  realized_pnl: number;
  /**
   * Number of contracts this position covers. Always 1 under the
   * top-3-only selector; legacy rows from the prior greedy-fill
   * regime may report > 1. UI multiplies live-holding P&L and
   * exposure by this value.
   */
  quantity: number;
  executed_at?: string | null;
  closed_at?: string | null;
  /**
   * For state='skipped' this is the at-open agent's reason for
   * declining the trade. For state='failed' or 'canceled' it carries
   * the broker error / cancellation cause. Empty / undefined for
   * healthy states.
   */
  note?: string | null;
}

export interface DashboardResponse {
  date: string;
  trades: DashboardTrade[];
  /**
  Plural per day so the basket auto-executor can fire up to N
  contracts and each trade card can find its matching execution.
  Empty / undefined when no auto-execution fired that day.
  */
  executions?: Execution[] | null;
}

export interface WeekDay {
  date: string;
  trades: DashboardTrade[];
  executions?: Execution[] | null;
}

export interface WeekResponse {
  start: string;
  end: string;
  days: WeekDay[];
}

export interface LiveQuoteEntry {
  last_price: number;
  open_price: number;
  net_change: number;
  net_change_pct: number;
  bid_price: number;
  ask_price: number;
  volume: number;
}

export interface LiveOptionEntry {
  bid: number;
  ask: number;
  last: number;
  mark: number;
  volume: number;
  open_interest: number;
  delta: number;
  theta: number;
  implied_vol: number;
}

/*
LiveQuotesResponse is the SSE `snapshot` event payload and the
in-memory shape the dashboard consumes. Per-tick `quote` events update
single map entries in place. Kept named "LiveQuotesResponse" rather
than "LiveQuotesSnapshot" so the existing consumer call sites
(MorningLayout, TradeTable, execution-badge) don't need wholesale
renames.
*/
export interface LiveQuotesResponse {
  connected: boolean;
  market_open: boolean;
  as_of: string;
  quotes: Record<string, LiveQuoteEntry>;
  options: Record<string, LiveOptionEntry>;
}

/*
QuoteUpdate is the SSE `quote` event payload, a single contract or
ticker's latest values. Exactly one of `quote` / `option` will be set;
the matching key is in `symbol` / `option_key`.
*/
export interface QuoteUpdate {
  symbol?: string;
  option_key?: string;
  quote?: LiveQuoteEntry;
  option?: LiveOptionEntry;
  as_of: string;
}

/*
MarketStatus is GET /api/market/status. The dashboard polls this on
mount + on visibility return to decide between rendering the live
streaming UI (open) or the closed-page (otherwise). NextOpen /
NextClose are populated regardless of state so we can render an
honest countdown.

session is a coarse label matching the server's calendar.Status:
  - "premarket", trading day, before 9:30 ET
  - "open"     , trading day, 9:30 ET to close (16:00 or 13:00 ET)
  - "afterhours", trading day, after close
  - "closed"   , weekend or holiday
*/
export interface MarketStatus {
  open: boolean;
  session: "premarket" | "open" | "afterhours" | "closed";
  next_open?: string;
  next_close?: string;
}

export interface ApiResponse {
  ok: boolean;
  message: string;
}
