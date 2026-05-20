export interface Trade {
  symbol: string;
  contract_type: "CALL" | "PUT";
  strike_price: number;
  expiration: string;
  dte: number;
  estimated_price: number;
  thesis: string;
  sentiment_score: number;
  current_price: number;
  target_price: number;
  stop_loss: number;
  risk_level: "LOW" | "MEDIUM" | "HIGH";
  catalyst: string;
  mention_count: number;
  rank: number;
  score: number;
  rationale: string;
  model: string;
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
  state: "submitted" | "holding" | "closed" | "close_failed" | "failed";
  symbol: string;
  contract_type: string;
  strike_price: number;
  open_price: number;
  close_price: number;
  realized_pnl: number;
  /**
   * Number of contracts this position covers. The two-phase selector
   * can fire a quantity > 1 order when the greedy fill duplicates a
   * rank — UI multiplies live-holding P&L and exposure by this. Legacy
   * single-contract executions report 1.
   */
  quantity: number;
  executed_at?: string | null;
  closed_at?: string | null;
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

export interface LiveQuotesResponse {
  connected: boolean;
  market_open: boolean;
  as_of: string;
  quotes: Record<string, LiveQuoteEntry>;
  options: Record<string, LiveOptionEntry>;
}

export interface ApiResponse {
  ok: boolean;
  message: string;
}
