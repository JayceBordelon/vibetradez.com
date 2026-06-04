package exec

import (
	"context"
	"fmt"
	"sync"
)

/*
ServiceConfig configures the broker service. SchwabAccountHash resolves
the Schwab-issued account hash that every Trader API call needs; it's
injected so the service doesn't reach back into the schwab client.
*/
type ServiceConfig struct {
	SchwabAccountHash func(ctx context.Context) (string, error)
}

/*
Service is the broker surface the v2 portfolio manager trades through. It
is a thin wrapper over a TraderClient (the live Schwab trader): the real
risk caps are enforced upstream in internal/portfolio (the tool layer), and
the order-placement entry points (PlaceEquityOrderAgent,
PlaceOptionOrderAgent, GetPositionsAgent) live in portfolio_service.go. The
mutex serializes the broker calls, which only run from the once-a-day
session and the intraday risk sweep.
*/
type Service struct {
	trader TraderClient
	cfg    ServiceConfig
	mu     sync.Mutex
}

func NewService(trader TraderClient, cfg ServiceConfig) *Service {
	return &Service{trader: trader, cfg: cfg}
}

/*
Mode reports the trading mode for the /health endpoint and the dashboard.
This account always trades live, so it is a constant.
*/
func (s *Service) Mode() string { return "live" }

/*
AvailableFundsAgent returns the cash available for a new position, under
the service mutex so it can't race an in-flight order on the same process.
Exposed to the portfolio agent as part of the snapshot (settled cash).
*/
func (s *Service) AvailableFundsAgent(ctx context.Context) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		return 0, fmt.Errorf("account hash: %w", err)
	}
	return s.trader.AvailableFunds(ctx, hash)
}
