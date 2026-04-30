package exec

import (
	"testing"

	"vibetradez.com/internal/trades"
)

func TestOCCSymbol_KnownFixtures(t *testing.T) {
	cases := []struct {
		name       string
		symbol     string
		expiration string
		kind       string
		strike     float64
		want       string
	}{
		{"AAPL 150 call", "AAPL", "2024-01-19", "CALL", 150.0, "AAPL  240119C00150000"},
		{"NVDA 875.50 put", "NVDA", "2026-04-17", "PUT", 875.50, "NVDA  260417P00875500"},
		{"SPY half-dollar strike", "SPY", "2025-12-19", "CALL", 600.5, "SPY   251219C00600500"},
		{"single-letter root", "F", "2025-06-20", "PUT", 12.0, "F     250620P00012000"},
		{"6-letter root", "GOOGL", "2025-03-21", "CALL", 200.25, "GOOGL 250321C00200250"},
		{"sub-dollar strike", "PLTR", "2025-02-21", "CALL", 0.5, "PLTR  250221C00000500"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := OCCSymbol(tc.symbol, tc.expiration, tc.kind, tc.strike)
			if err != nil {
				t.Fatalf("OCCSymbol: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
			if len(got) != 21 {
				t.Errorf("OCC symbol must be 21 chars, got %d (%q)", len(got), got)
			}
		})
	}
}

func TestOCCSymbol_RoundsCentsCorrectly(t *testing.T) {
	// 150.00 must NOT drift to 00149999 due to float arithmetic.
	got, err := OCCSymbol("AAPL", "2024-01-19", "CALL", 150.00000001)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(got, "00150000") {
		t.Errorf("expected strike 00150000, got %q", got)
	}
}

func TestOCCSymbol_Errors(t *testing.T) {
	bad := []struct {
		name       string
		symbol     string
		expiration string
		kind       string
		strike     float64
	}{
		{"empty symbol", "", "2024-01-19", "CALL", 150.0},
		{"too long root", "TOOLONG", "2024-01-19", "CALL", 150.0},
		{"invalid date", "AAPL", "01-19-2024", "CALL", 150.0},
		{"bad direction", "AAPL", "2024-01-19", "STRADDLE", 150.0},
		{"negative strike", "AAPL", "2024-01-19", "CALL", -1.0},
		{"zero strike", "AAPL", "2024-01-19", "CALL", 0},
		{"oversized strike", "AAPL", "2024-01-19", "CALL", 100000.0},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := OCCSymbol(tc.symbol, tc.expiration, tc.kind, tc.strike); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestBuildOpenOrderForTrade_HardcodedShape(t *testing.T) {
	tr := &trades.Trade{Symbol: "AAPL", ContractType: "CALL", StrikePrice: 150, Expiration: "2024-01-19"}
	o, err := BuildOpenOrderForTrade(tr, "AAPL  240119C00150000")
	if err != nil {
		t.Fatal(err)
	}
	if o.OrderType != "MARKET" || o.Session != "NORMAL" || o.Duration != "DAY" || o.OrderStrategyType != "SINGLE" {
		t.Errorf("envelope wrong: %+v", o)
	}
	if len(o.OrderLegCollection) != 1 {
		t.Fatalf("want 1 leg got %d", len(o.OrderLegCollection))
	}
	leg := o.OrderLegCollection[0]
	if leg.Instruction != "BUY_TO_OPEN" {
		t.Errorf("instruction must be BUY_TO_OPEN, got %q", leg.Instruction)
	}
	if leg.Quantity != 1 {
		t.Errorf("quantity must be 1, got %d", leg.Quantity)
	}
	if leg.Instrument.AssetType != "OPTION" {
		t.Errorf("asset type must be OPTION, got %q", leg.Instrument.AssetType)
	}
}

func TestBuildOpenOrderForTrade_RejectsNilTrade(t *testing.T) {
	if _, err := BuildOpenOrderForTrade(nil, "AAPL  240119C00150000"); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildOpenOrderForTrade_RejectsMissingOCC(t *testing.T) {
	if _, err := BuildOpenOrderForTrade(&trades.Trade{Symbol: "AAPL"}, ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildCloseOrderForPosition_SellToClose(t *testing.T) {
	p := &OpenPosition{Symbol: "AAPL", ContractType: "CALL", StrikePrice: 150, Expiration: "2024-01-19"}
	o, err := BuildCloseOrderForPosition(p)
	if err != nil {
		t.Fatal(err)
	}
	if o.OrderLegCollection[0].Instruction != "SELL_TO_CLOSE" {
		t.Errorf("expected SELL_TO_CLOSE, got %q", o.OrderLegCollection[0].Instruction)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
