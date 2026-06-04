package exec

import (
	"context"
	"fmt"
	"log"
)

/*
MaxPortfolioOrderCostCeiling is a gross, absolute defense-in-depth ceiling
on the total cost of any single order the v2 portfolio manager submits. It
is NOT the risk policy — the real per-order cap is a percentage of live
equity enforced in internal/portfolio (caps.go, CheckBuy). This is only a
catastrophe backstop on the way out to the broker: it catches a fat-finger
or a buggy caller that tries to push an absurd order (e.g. 10,000 shares),
regardless of what the tool layer believed. Set deliberately far above any
sane order for the target account size so it never blocks legitimate
sizing; if the account ever grows past it, raise it consciously.
*/
const MaxPortfolioOrderCostCeiling = 25000.00

/*
PlaceEquityOrderAgent submits a single equity LIMIT order (side "BUY" or
"SELL") for the v2 portfolio manager and returns the broker order id. It is
intentionally thin: the portfolio tool layer has already enforced the real
risk caps, so this method only builds the order, re-checks the gross
catastrophe ceiling, and submits. It does NOT write the v1 executions/trades
tables — portfolio moves are persisted to portfolio_decisions by the caller
after the agent run. Held under the service mutex so it can't race the v1
reconcile/close crons on the shared trader.
*/
func (s *Service) PlaceEquityOrderAgent(ctx context.Context, symbol, side string, quantity int, limitPrice float64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cost := limitPrice * float64(quantity); cost > MaxPortfolioOrderCostCeiling+1e-6 {
		return "", fmt.Errorf("equity order cost $%.2f (%d × $%.2f) exceeds the absolute ceiling $%.2f", cost, quantity, limitPrice, MaxPortfolioOrderCostCeiling)
	}
	order, err := BuildEquityOrder(symbol, side, quantity, limitPrice)
	if err != nil {
		return "", fmt.Errorf("build equity order: %w", err)
	}
	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		return "", fmt.Errorf("account hash: %w", err)
	}
	orderID, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		return "", fmt.Errorf("place equity order: %w", err)
	}
	log.Printf("portfolio: equity %s submitted (%s %d @ $%.2f, order=%s, mode=%s)", side, symbol, quantity, limitPrice, orderID, s.Mode())
	return orderID, nil
}

/*
PlaceOptionOrderAgent submits a single option LIMIT order against a
pre-built OCC symbol (instruction "BUY_TO_OPEN" or "SELL_TO_CLOSE") for the
v2 portfolio manager and returns the broker order id. Same thin contract as
PlaceEquityOrderAgent: real caps are upstream, this re-checks only the gross
ceiling (limit × 100 × contracts) and submits.
*/
func (s *Service) PlaceOptionOrderAgent(ctx context.Context, occSymbol, instruction string, quantity int, limitPrice float64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cost := limitPrice * 100 * float64(quantity); cost > MaxPortfolioOrderCostCeiling+1e-6 {
		return "", fmt.Errorf("option order cost $%.2f (%d × $%.2f × 100) exceeds the absolute ceiling $%.2f", cost, quantity, limitPrice, MaxPortfolioOrderCostCeiling)
	}
	order, err := BuildOptionLimitOrder(occSymbol, instruction, quantity, limitPrice)
	if err != nil {
		return "", fmt.Errorf("build option order: %w", err)
	}
	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		return "", fmt.Errorf("account hash: %w", err)
	}
	orderID, err := s.trader.PlaceOrder(ctx, hash, order)
	if err != nil {
		return "", fmt.Errorf("place option order: %w", err)
	}
	log.Printf("portfolio: option %s submitted (%s %d @ $%.2f, order=%s, mode=%s)", instruction, occSymbol, quantity, limitPrice, orderID, s.Mode())
	return orderID, nil
}

/*
GetPositionsAgent returns the account's open positions for the v2 portfolio
manager's session snapshot. Held under the service mutex (consistent with
AvailableFundsAgent) so it sees a coherent broker view.
*/
func (s *Service) GetPositionsAgent(ctx context.Context) ([]BrokerPosition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("account hash: %w", err)
	}
	return s.trader.GetPositions(ctx, hash)
}

/*
OrderStatusAgent returns the current broker state of a previously placed
order (working / filled / canceled, fill price, etc.) for the portfolio
agent's get_order_status tool. Held under the service mutex so it can't race
an in-flight place/cancel on the same process.
*/
func (s *Service) OrderStatusAgent(ctx context.Context, orderID string) (OrderStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		return OrderStatus{}, fmt.Errorf("account hash: %w", err)
	}
	return s.trader.GetOrder(ctx, hash, orderID)
}

/*
CancelOrderAgent requests cancellation of a resting order for the portfolio
agent's cancel_order tool. Best-effort at the broker (a filled order can't
be canceled). Held under the service mutex.
*/
func (s *Service) CancelOrderAgent(ctx context.Context, orderID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		return fmt.Errorf("account hash: %w", err)
	}
	return s.trader.CancelOrder(ctx, hash, orderID)
}
