package portfoliowire

import (
	"context"
	"testing"

	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/portfolio"
	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/store"
)

type fakeMD struct {
	quotes map[string]schwab.StockQuote
	chain  *schwab.OptionChain
}

func (f *fakeMD) GetQuotes(symbols []string) (map[string]schwab.StockQuote, error) {
	return f.quotes, nil
}
func (f *fakeMD) GetOptionChain(string, string, string, string, float64) (*schwab.OptionChain, error) {
	return f.chain, nil
}
func (f *fakeMD) MarketCap(string) (float64, error) { return 50_000_000_000, nil }
func (f *fakeMD) GetDailyPriceHistory(string) (*schwab.PriceHistory, error) {
	return &schwab.PriceHistory{Candles: []schwab.Candle{{Close: 100, High: 100, Low: 100}}}, nil
}
func (f *fakeMD) GetFundamentals(string) (*schwab.Fundamentals, error) {
	return &schwab.Fundamentals{}, nil
}

type fakeBroker struct {
	positions []exec.BrokerPosition
	funds     float64
}

func (f *fakeBroker) GetPositionsAgent(context.Context) ([]exec.BrokerPosition, error) {
	return f.positions, nil
}
func (f *fakeBroker) AvailableFundsAgent(context.Context) (float64, error) { return f.funds, nil }
func (f *fakeBroker) PlaceEquityOrderAgent(context.Context, string, string, int, float64) (string, error) {
	return "eq-1", nil
}
func (f *fakeBroker) PlaceOptionOrderAgent(context.Context, string, string, int, float64) (string, error) {
	return "op-1", nil
}
func (f *fakeBroker) OrderStatusAgent(context.Context, string) (exec.OrderStatus, error) {
	return exec.OrderStatus{}, nil
}
func (f *fakeBroker) CancelOrderAgent(context.Context, string) error { return nil }

type fakeHW struct {
	mark float64
	ok   bool
}

func (f *fakeHW) GetHighWaterMark() (float64, bool, error) { return f.mark, f.ok, nil }
func (f *fakeHW) RecentPortfolioDecisions(int) ([]store.PortfolioDecisionRow, error) {
	return nil, nil
}
func (f *fakeHW) RecentPortfolioStances(int) ([]store.PortfolioStanceRow, error) { return nil, nil }
func (f *fakeHW) LatestPortfolioSession() (string, string, bool, error)          { return "", "", false, nil }

func TestSnapshot_AssemblesEquityAndBudget(t *testing.T) {
	bk := &fakeBroker{
		funds: 2000,
		positions: []exec.BrokerPosition{
			{Symbol: "MDT", AssetType: "EQUITY", Underlying: "MDT", Quantity: 20, MarketValue: 1600, AverageCost: 78},
		},
	}
	r := NewReader(&fakeMD{}, bk, &fakeHW{mark: 9000, ok: true})

	snap, err := r.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Equity = positions ($1,600) + settled cash ($2,000) = $3,600.
	if snap.Equity != 3600 {
		t.Fatalf("expected equity 3600, got %f", snap.Equity)
	}
	if snap.SettledCash != 2000 {
		t.Fatalf("expected settled cash 2000, got %f", snap.SettledCash)
	}
	// Deployment budget = settled cash * 50%.
	if snap.DeploymentBudget != 2000*portfolio.DefaultDailyDeploymentPct {
		t.Fatalf("unexpected deployment budget %f", snap.DeploymentBudget)
	}
	// High-water from the store ($9,000) beats current equity.
	if snap.HighWaterMark != 9000 {
		t.Fatalf("expected high-water 9000, got %f", snap.HighWaterMark)
	}
	if len(snap.Positions) != 1 || snap.Positions[0].AssetType != portfolio.AssetEquity {
		t.Fatalf("unexpected positions: %+v", snap.Positions)
	}
}

func TestSnapshot_FreshAccountUsesEquityAsHighWater(t *testing.T) {
	bk := &fakeBroker{funds: 6000}
	r := NewReader(&fakeMD{}, bk, &fakeHW{ok: false}) // empty curve
	snap, err := r.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.HighWaterMark != snap.Equity {
		t.Fatalf("fresh account high-water should equal equity, got hwm=%f equity=%f", snap.HighWaterMark, snap.Equity)
	}
}

func TestToPortfolioPosition_DecodesOption(t *testing.T) {
	// 2 MDT calls held; broker omits strike/exp so they're decoded from OCC.
	bp := exec.BrokerPosition{
		Symbol:      "MDT   260417C00079000",
		AssetType:   "OPTION",
		Underlying:  "MDT",
		Quantity:    2,
		MarketValue: 240,
		AverageCost: 1.10,
	}
	p := toPortfolioPosition(bp)
	if p.AssetType != portfolio.AssetOption {
		t.Fatalf("expected option asset type, got %s", p.AssetType)
	}
	if p.Strike != 79 || p.ContractType != "CALL" || p.Expiration != "2026-04-17" {
		t.Fatalf("OCC not decoded: strike=%f type=%s exp=%s", p.Strike, p.ContractType, p.Expiration)
	}
	// Cost basis = avg cost * qty * 100 (option multiplier).
	if diff := p.CostBasis - 220; diff < -1e-6 || diff > 1e-6 {
		t.Fatalf("expected cost basis ~220, got %f", p.CostBasis)
	}
}
