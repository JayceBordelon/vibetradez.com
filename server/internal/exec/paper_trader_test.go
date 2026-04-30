package exec

import (
	"context"
	"errors"
	"testing"

	"vibetradez.com/internal/trades"
)

type fakeMarks struct {
	mark float64
	err  error
}

func (f *fakeMarks) OptionMark(_ context.Context, _, _, _ string, _ float64) (float64, error) {
	return f.mark, f.err
}

func TestPaperTrader_PlaceFillsAtMark(t *testing.T) {
	pt := NewPaperTrader(&fakeMarks{mark: 3.50})
	tr := &trades.Trade{Symbol: "AAPL", ContractType: "CALL", StrikePrice: 150, Expiration: "2024-01-19"}
	order, err := BuildOpenOrderForTrade(tr, "AAPL  240119C00150000")
	if err != nil {
		t.Fatal(err)
	}
	id, err := pt.PlaceOrder(context.Background(), "PAPER-ACCOUNT", order)
	if err != nil {
		t.Fatal(err)
	}
	st, err := pt.GetOrder(context.Background(), "PAPER-ACCOUNT", id)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Filled {
		t.Errorf("expected filled, got %+v", st)
	}
	if st.FillPrice != 3.50 {
		t.Errorf("expected fill price 3.50, got %f", st.FillPrice)
	}
	if st.FilledQuantity != 1 {
		t.Errorf("expected qty 1, got %d", st.FilledQuantity)
	}
}

func TestPaperTrader_RejectsWhenMarkLookupFails(t *testing.T) {
	pt := NewPaperTrader(&fakeMarks{err: errors.New("schwab down")})
	tr := &trades.Trade{Symbol: "AAPL", ContractType: "CALL", StrikePrice: 150, Expiration: "2024-01-19"}
	order, _ := BuildOpenOrderForTrade(tr, "AAPL  240119C00150000")
	id, err := pt.PlaceOrder(context.Background(), "PAPER-ACCOUNT", order)
	if err != nil {
		t.Fatalf("PlaceOrder should succeed even on mark failure (status=REJECTED): %v", err)
	}
	st, err := pt.GetOrder(context.Background(), "PAPER-ACCOUNT", id)
	if err != nil {
		t.Fatal(err)
	}
	if st.Filled {
		t.Error("expected NOT filled when mark lookup failed")
	}
	if st.RawStatus != "REJECTED" {
		t.Errorf("expected REJECTED, got %q", st.RawStatus)
	}
	if st.ErrorMessage == "" {
		t.Error("expected error message to be populated")
	}
}

func TestPaperTrader_AccountHashConstant(t *testing.T) {
	pt := NewPaperTrader(&fakeMarks{mark: 1.0})
	h, err := pt.AccountHash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h != "PAPER-ACCOUNT" {
		t.Errorf("got %q want PAPER-ACCOUNT", h)
	}
}

func TestPaperTrader_GetUnknownOrder(t *testing.T) {
	pt := NewPaperTrader(&fakeMarks{mark: 1.0})
	if _, err := pt.GetOrder(context.Background(), "PAPER-ACCOUNT", "paper-bogus"); err == nil {
		t.Fatal("expected error for unknown order id")
	}
}

func TestDecodeOCCSymbolRoundTrip(t *testing.T) {
	cases := []struct {
		symbol     string
		expiration string
		kind       string
		strike     float64
	}{
		{"AAPL", "2024-01-19", "CALL", 150.0},
		{"NVDA", "2026-04-17", "PUT", 875.50},
		{"F", "2025-06-20", "PUT", 12.0},
		{"GOOGL", "2025-03-21", "CALL", 200.25},
	}
	for _, tc := range cases {
		t.Run(tc.symbol, func(t *testing.T) {
			occ, err := OCCSymbol(tc.symbol, tc.expiration, tc.kind, tc.strike)
			if err != nil {
				t.Fatal(err)
			}
			gotSym, gotExp, gotKind, gotStrike, err := decodeOCCSymbol(occ)
			if err != nil {
				t.Fatal(err)
			}
			if gotSym != tc.symbol || gotExp != tc.expiration || gotKind != tc.kind || gotStrike != tc.strike {
				t.Errorf("round-trip mismatch: got (%s, %s, %s, %f), want (%s, %s, %s, %f)",
					gotSym, gotExp, gotKind, gotStrike, tc.symbol, tc.expiration, tc.kind, tc.strike)
			}
		})
	}
}
