package schwab

import (
	"encoding/json"
	"testing"
)

func TestFormatOCCSymbol(t *testing.T) {
	cases := []struct {
		name         string
		sym          string
		contractType string
		exp          string
		strike       float64
		want         string
		wantErr      bool
	}{
		{
			name:         "AAPL 150 CALL 2024-01-19",
			sym:          "AAPL",
			contractType: "CALL",
			exp:          "2024-01-19",
			strike:       150.00,
			want:         "AAPL  240119C00150000",
		},
		{
			name:         "lowercased symbol uppercased",
			sym:          "aapl",
			contractType: "call",
			exp:          "2024-01-19",
			strike:       150.00,
			want:         "AAPL  240119C00150000",
		},
		{
			name:         "PUT with fractional strike",
			sym:          "TSLA",
			contractType: "PUT",
			exp:          "2026-05-23",
			strike:       212.50,
			want:         "TSLA  260523P00212500",
		},
		{
			name:         "6-char symbol no padding",
			sym:          "BRKB",
			contractType: "CALL",
			exp:          "2026-12-18",
			strike:       400.00,
			want:         "BRKB  261218C00400000",
		},
		{
			name:         "low strike",
			sym:          "F",
			contractType: "CALL",
			exp:          "2026-05-30",
			strike:       0.50,
			want:         "F     260530C00000500",
		},
		{name: "empty symbol", sym: "", contractType: "CALL", exp: "2024-01-19", strike: 150, wantErr: true},
		{name: "symbol too long", sym: "ABCDEFG", contractType: "CALL", exp: "2024-01-19", strike: 150, wantErr: true},
		{name: "bad contract type", sym: "AAPL", contractType: "FOO", exp: "2024-01-19", strike: 150, wantErr: true},
		{name: "bad date", sym: "AAPL", contractType: "CALL", exp: "not-a-date", strike: 150, wantErr: true},
		{name: "zero strike", sym: "AAPL", contractType: "CALL", exp: "2024-01-19", strike: 0, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FormatOCCSymbol(tc.sym, tc.contractType, tc.exp, tc.strike)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if len(got) != 21 {
				t.Errorf("OCC symbol must be exactly 21 chars, got %d (%q)", len(got), got)
			}
		})
	}
}

func TestParseEquityTick(t *testing.T) {
	raw := `{"key": "AAPL", "1": 220.45, "2": 220.50, "3": 220.48, "8": 12345678, "15": 219.00, "18": 1.23, "42": 0.56}`
	var c map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ev := parseEquityTick(c)
	if ev == nil {
		t.Fatal("expected non-nil tick")
	}
	if ev.Symbol != "AAPL" {
		t.Errorf("Symbol: got %q, want AAPL", ev.Symbol)
	}
	if ev.BidPrice != 220.45 || ev.AskPrice != 220.50 || ev.LastPrice != 220.48 {
		t.Errorf("bid/ask/last mismatch: %+v", ev)
	}
	if ev.OpenPrice != 219.00 {
		t.Errorf("OpenPrice: got %v, want 219.00", ev.OpenPrice)
	}
	if ev.NetChange != 1.23 || ev.NetChangePct != 0.56 {
		t.Errorf("net change mismatch: %+v", ev)
	}
	if ev.Volume != 12345678 {
		t.Errorf("Volume: got %v, want 12345678", ev.Volume)
	}
}

func TestParseEquityTick_MissingKey(t *testing.T) {
	c := map[string]interface{}{"1": 220.0}
	if ev := parseEquityTick(c); ev != nil {
		t.Errorf("expected nil for missing key, got %+v", ev)
	}
}

func TestParseOptionTick(t *testing.T) {
	raw := `{"key": "AAPL  240119C00150000", "2": 1.20, "3": 1.25, "4": 1.22, "8": 5000, "9": 12345, "10": 0.32, "28": 0.55, "30": -0.04, "37": 1.225}`
	var c map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ev := parseOptionTick(c)
	if ev == nil {
		t.Fatal("expected non-nil tick")
	}
	if ev.OCCSymbol != "AAPL  240119C00150000" {
		t.Errorf("OCCSymbol: got %q", ev.OCCSymbol)
	}
	if ev.Bid != 1.20 || ev.Ask != 1.25 || ev.Last != 1.22 || ev.Mark != 1.225 {
		t.Errorf("bid/ask/last/mark mismatch: %+v", ev)
	}
	if ev.Delta != 0.55 || ev.Theta != -0.04 || ev.ImpliedVol != 0.32 {
		t.Errorf("greeks mismatch: %+v", ev)
	}
	if ev.Volume != 5000 || ev.OpenInterest != 12345 {
		t.Errorf("volume/OI mismatch: %+v", ev)
	}
}

func TestHandleFrame_LoginAck(t *testing.T) {
	s := NewStreamClient(nil)
	raw := []byte(`{"response":[{"service":"ADMIN","command":"LOGIN","content":{"code":0,"msg":"ok"}}]}`)
	if err := s.handleFrame(raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Without a conn, onLoginSuccess is a no-op but loggedIn should still flip.
	s.mu.Lock()
	logged := s.loggedIn
	s.mu.Unlock()
	if !logged {
		t.Error("expected loggedIn=true after LOGIN ack")
	}
}

func TestHandleFrame_LoginReject(t *testing.T) {
	s := NewStreamClient(nil)
	raw := []byte(`{"response":[{"service":"ADMIN","command":"LOGIN","content":{"code":3,"msg":"bad token"}}]}`)
	if err := s.handleFrame(raw); err == nil {
		t.Error("expected error on LOGIN reject (code != 0)")
	}
}

func TestHandleFrame_DataFanout(t *testing.T) {
	s := NewStreamClient(nil)
	raw := []byte(`{"data":[
		{"service":"LEVELONE_EQUITIES","command":"SUBS","content":[{"key":"AAPL","3":220.48}]},
		{"service":"LEVELONE_OPTIONS","command":"SUBS","content":[{"key":"AAPL  240119C00150000","37":1.225}]}
	]}`)
	if err := s.handleFrame(raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Drain channel; expect two events.
	var got []StreamEvent
	for i := 0; i < 2; i++ {
		select {
		case ev := <-s.eventsCh:
			got = append(got, ev)
		default:
			t.Fatalf("expected 2 events, got %d", i)
		}
	}
	hasEquity, hasOption := false, false
	for _, ev := range got {
		if ev.Equity != nil && ev.Equity.Symbol == "AAPL" {
			hasEquity = true
		}
		if ev.Option != nil && ev.Option.Mark == 1.225 {
			hasOption = true
		}
	}
	if !hasEquity || !hasOption {
		t.Errorf("expected both equity + option events, got %+v", got)
	}
}
