package server

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

/*
Data-contract tests. The Go API response structs are the source of truth for
the JSON the Next.js client consumes (the TS interfaces in
client/types/portfolio.ts mirror these key sets). A field rename, removal, or
retag here silently breaks the client, which reads the missing key as
undefined. These tests pin the exact JSON key set of every /api/portfolio*
response shape, so any drift fails CI and forces a matching TS update.

When you intentionally change a wire shape: update the expected keys here AND
the matching interface in client/types/portfolio.ts in the same change.
*/

// jsonKeys marshals v and returns its top-level object keys, sorted. Fields
// tagged omitempty only appear when non-zero, so callers populate every field
// to assert the FULL contract surface.
func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func assertContract(t *testing.T, name string, v any, want []string) {
	t.Helper()
	got := jsonKeys(t, v)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("%s JSON contract drifted:\n got:  %v\n want: %v\nUpdate client/types/portfolio.ts to match, then update this test.", name, got, want)
	}
}

func f64(v float64) *float64 { return &v }

func TestContract_PortfolioResponse(t *testing.T) {
	resp := portfolioResponse{
		Enabled: true, Mode: "live", Date: "2026-06-04", Equity: 1, SettledCash: 1, UnsettledCash: 1,
		HighWaterMark: 1, SPYClose: 1,
		Positions:       []portfolioPositionView{},
		PositionsSource: "live", PositionsAsOf: "2026-06-04", Stance: "x", Summary: "y",
		Decisions: []portfolioDecisionView{},
	}
	assertContract(t, "portfolioResponse", resp, []string{
		"enabled", "mode", "date", "equity", "settled_cash", "unsettled_cash", "high_water_mark",
		"spy_close", "positions", "positions_source", "positions_as_of",
		"stance", "summary", "decisions",
	})
}

func TestContract_PositionView(t *testing.T) {
	v := portfolioPositionView{
		Symbol: "MDT", Underlying: "MDT", AssetType: "EQUITY", ContractType: "CALL", Strike: 1,
		Expiration: "2026-04-17", DTE: 1, Quantity: 1, MarketValue: 1, CostBasis: 1, UnrealizedPnl: 1,
	}
	assertContract(t, "portfolioPositionView", v, []string{
		"symbol", "underlying", "asset_type", "contract_type", "strike", "expiration", "dte",
		"quantity", "market_value", "cost_basis", "unrealized_pnl",
	})
}

func TestContract_DecisionView(t *testing.T) {
	v := portfolioDecisionView{
		Action: "buy_equity", AssetType: "EQUITY", Symbol: "MDT", Underlying: "MDT", ContractType: "CALL",
		Strike: f64(1), Expiration: "2026-04-17", Quantity: 1, LimitPrice: 1, Notional: 1, OrderID: "o",
		Status: "submitted", Rationale: "r", TradeID: "uuid-1", TradeKind: "holding",
	}
	assertContract(t, "portfolioDecisionView", v, []string{
		"action", "asset_type", "symbol", "underlying", "contract_type", "strike", "expiration",
		"quantity", "limit_price", "notional", "order_id", "status", "rationale", "trade_id", "trade_kind",
	})
}

func TestContract_EquityCurve(t *testing.T) {
	assertContract(t, "equityCurvePointView", equityCurvePointView{
		Date: "2026-06-04", AccountEquity: 1, SettledCash: 1, UnsettledCash: 1, HighWaterMark: 1, SPYClose: 1, RealizedCum: 1, Unrealized: 1,
	}, []string{"date", "account_equity", "settled_cash", "unsettled_cash", "high_water_mark", "spy_close", "realized_cum", "unrealized"})
	assertContract(t, "equityCurveResponse", equityCurveResponse{Points: []equityCurvePointView{}}, []string{"points"})
}

func TestContract_Holdings(t *testing.T) {
	h := holdingView{
		ID: "MDT", Kind: "stock", Symbol: "MDT", Underlying: "MDT", Label: "MDT", ContractType: "CALL",
		Strike: 1, Expiration: "2026-04-17", DTE: 1, Quantity: 1, MarketValue: 1, CostBasis: 1,
		UnrealizedPnl: 1, OpenedDate: "2026-06-01", OpenRationale: "r",
	}
	assertContract(t, "holdingView", h, []string{
		"id", "kind", "symbol", "underlying", "label", "contract_type", "strike", "expiration", "dte",
		"quantity", "market_value", "cost_basis", "unrealized_pnl", "opened_date", "open_rationale",
	})
	assertContract(t, "holdingsResponse", holdingsResponse{
		Holdings: []holdingView{}, PositionsSource: "live", PositionsAsOf: "2026-06-04",
	}, []string{"holdings", "positions_source", "positions_as_of"})
}

func TestContract_Closed(t *testing.T) {
	c := closedTradeView{
		ID: "MDT-2026-06-02", Kind: "stock", Symbol: "MDT", Underlying: "MDT", Label: "MDT", ContractType: "CALL",
		Strike: 1, Expiration: "2026-04-17", Quantity: 1, OpenedDate: "2026-05-20", ClosedDate: "2026-06-02",
		EntryPrice: 1, ExitPrice: 1, CostBasis: 1, Proceeds: 1, RealizedPnl: 1, RealizedPct: 1, HoldDays: 1,
		OpenRationale: "o", CloseRationale: "c",
	}
	assertContract(t, "closedTradeView", c, []string{
		"id", "kind", "symbol", "underlying", "label", "contract_type", "strike", "expiration", "quantity",
		"opened_date", "closed_date", "entry_price", "exit_price", "cost_basis", "proceeds", "realized_pnl",
		"realized_pct", "hold_days", "open_rationale", "close_rationale",
	})
	assertContract(t, "closedTradesResponse", closedTradesResponse{Trades: []closedTradeView{}}, []string{"trades"})
}

func TestContract_PositionHistory(t *testing.T) {
	assertContract(t, "positionValuePointView", positionValuePointView{
		Date: "2026-06-04", MarketValue: 1, Quantity: 1, CostBasis: 1,
	}, []string{"date", "market_value", "quantity", "cost_basis"})
	assertContract(t, "positionHistoryResponse", positionHistoryResponse{Symbol: "NVDA", Points: []positionValuePointView{}}, []string{"symbol", "points"})
}

func TestContract_PriceHistory(t *testing.T) {
	assertContract(t, "pricePointView", pricePointView{Date: "2026-06-04", Close: 1}, []string{"date", "close"})
	assertContract(t, "priceHistoryResponse", priceHistoryResponse{Symbol: "NVDA", Points: []pricePointView{}}, []string{"symbol", "points"})
}
