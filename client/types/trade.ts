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
  profit_target: number;
  risk_level: "LOW" | "MEDIUM" | "HIGH";
  catalyst: string;
  mention_count: number;
  rank: number;
  gpt_score: number;
  gpt_rationale: string;
  claude_score: number;
  claude_rationale: string;
  combined_score: number;
  picked_by_openai: boolean;
  picked_by_claude: boolean;
}

export type ModelPicker = "all" | "openai" | "claude";

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

export interface ModelPickSummary {
  date: string;
  symbol: string;
  contract_type: string;
  pnl: number;
  pct_return: number;
  score: number;
}

export interface ModelDayPnl {
  date: string;
  pnl: number;
}

export interface ModelDayBreakdown {
  date: string;
  pnl: number;
  trades: number;
  winners: number;
  losers: number;
  picks: ModelPickSummary[];
}

export interface ModelStats {
  model: string;
  total_pnl: number;
  win_rate: number;
  avg_pct_return: number;
  trades_evaluated: number;
  avg_score: number;
  best_pick: ModelPickSummary | null;
  worst_pick: ModelPickSummary | null;
  cumulative_pnl: ModelDayPnl[];
  daily_breakdown: ModelDayBreakdown[];
}

export interface ModelComparisonResponse {
  range: string;
  start: string;
  end: string;
  top_n: number;
  openai: ModelStats;
  anthropic: ModelStats;
  combined: ModelStats;
  agreement_rate: number;
  total_dual_scored: number;
  total_days_covered: number;
}

export interface DashboardResponse {
  date: string;
  trades: DashboardTrade[];
}

export interface WeekDay {
  date: string;
  trades: DashboardTrade[];
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

export interface ChartCandle {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

export interface ChartResponse {
  symbol: string;
  candles: ChartCandle[];
}

export interface ChartParams {
  period: number;
  ptype: string;
  ftype: string;
  freq: number;
}
