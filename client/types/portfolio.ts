/*
Types for the v2 portfolio-manager dashboard. This surface describes a
SINGLE personal brokerage account (one book), not a multi-user product.
Mirrors the server wire shapes in internal/server/portfolio.go.
*/

export interface PortfolioPosition {
  symbol: string;
  underlying: string;
  asset_type: "EQUITY" | "OPTION";
  contract_type?: string;
  strike?: number;
  expiration?: string;
  dte?: number;
  quantity: number;
  market_value: number;
  cost_basis: number;
  unrealized_pnl: number;
  /** Position valued at today's opening print (absent: no quote). */
  open_value?: number;
  /** Position's mark at the last EOD snapshot before today; presence means
   *  it was held overnight (absent for positions opened today). */
  prev_close_value?: number;
}

export type PortfolioAction = "buy_equity" | "sell_equity" | "buy_option" | "sell_option" | "hold";

export interface PortfolioDecision {
  action: PortfolioAction;
  asset_type?: "EQUITY" | "OPTION";
  symbol?: string;
  underlying?: string;
  contract_type?: string;
  strike?: number;
  expiration?: string;
  quantity?: number;
  limit_price?: number;
  notional?: number;
  order_id?: string;
  status?: string;
  rationale: string;
  /** The trade page this move belongs to (executions tape navigation). */
  trade_id?: string;
  trade_kind?: "holding" | "closed";
}

/*
PortfolioResponse is GET /api/portfolio. `enabled` is false when the
broker isn't wired (trading disabled), so the dashboard renders an honest
"not running yet" state instead of erroring.
*/
export interface PortfolioResponse {
  enabled: boolean;
  mode: "live" | "";
  date: string;
  equity: number;
  settled_cash: number;
  unsettled_cash: number;
  high_water_mark: number;
  spy_close: number;
  positions: PortfolioPosition[];
  stance: string;
  summary?: string;
  action_items?: string;
  decisions: PortfolioDecision[];
}

export interface EquityCurvePoint {
  date: string;
  account_equity: number;
  settled_cash: number;
  unsettled_cash: number;
  high_water_mark: number;
  spy_close: number;
  /** Cumulative realized P&L of round trips closed inside the window, USD. */
  realized_cum: number;
  /** That day's EOD open-book unrealized P&L, USD (0 on flat days). */
  unrealized: number;
}

export interface EquityCurveResponse {
  points: EquityCurvePoint[];
}

/* Trade-detail chart series. */

export interface PositionValuePoint {
  date: string;
  market_value: number;
  quantity: number;
  cost_basis: number;
}

export interface PositionHistoryResponse {
  symbol: string;
  points: PositionValuePoint[];
}

export interface PricePoint {
  date: string;
  close: number;
}

export interface PriceHistoryResponse {
  symbol: string;
  points: PricePoint[];
}

/*
Per-position surfaces behind /holdings and /closed. `kind` routes to the
right detail renderer; `id` is the symbol with spaces stripped (so an OCC
option symbol is URL-safe), and a closed trade's id appends its close date
to stay unique across re-trades.
*/
export type TradeKind = "stock" | "option";

export interface Holding {
  id: string;
  kind: TradeKind;
  symbol: string;
  underlying: string;
  label: string;
  contract_type?: string;
  strike?: number;
  expiration?: string;
  dte?: number;
  quantity: number;
  market_value: number;
  cost_basis: number;
  unrealized_pnl: number;
  opened_date?: string;
  open_rationale?: string;
  /** Day-split anchors; see PortfolioPosition. */
  open_value?: number;
  prev_close_value?: number;
}

export interface HoldingsResponse {
  holdings: Holding[];
  positions_source?: "live" | "snapshot" | "";
  positions_as_of?: string;
}

export interface ClosedTrade {
  id: string;
  kind: TradeKind;
  symbol: string;
  underlying: string;
  label: string;
  contract_type?: string;
  strike?: number;
  expiration?: string;
  quantity: number;
  opened_date: string;
  closed_date: string;
  entry_price: number;
  exit_price: number;
  cost_basis: number;
  proceeds: number;
  realized_pnl: number;
  realized_pct: number;
  hold_days: number;
  open_rationale?: string;
  close_rationale?: string;
}

export interface ClosedTradesResponse {
  trades: ClosedTrade[];
}
