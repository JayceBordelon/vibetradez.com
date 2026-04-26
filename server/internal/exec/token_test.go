package exec

import (
	"strings"
	"testing"
	"time"
)

func testSecret() []byte {
	s := make([]byte, 32)
	for i := range s {
		s[i] = byte(i + 1)
	}
	return s
}

func TestMintVerifyRoundTrip(t *testing.T) {
	secret := testSecret()
	exp := time.Now().Add(5 * time.Minute)
	tok, err := Mint(42, ActionExecute, exp, secret)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	id, act, err := Verify(tok, secret)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if id != 42 {
		t.Errorf("decision id: got %d want 42", id)
	}
	if act != ActionExecute {
		t.Errorf("action: got %q want execute", act)
	}
}

func TestMintRejectsBadAction(t *testing.T) {
	if _, err := Mint(1, "yolo", time.Now().Add(time.Minute), testSecret()); err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestMintRejectsShortSecret(t *testing.T) {
	if _, err := Mint(1, ActionExecute, time.Now().Add(time.Minute), []byte("too short")); err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	secret := testSecret()
	tok, _ := Mint(7, ActionDecline, time.Now().Add(time.Minute), secret)
	parts := strings.SplitN(tok, ".", 2)
	if len(parts) != 2 {
		t.Fatalf("malformed mint output")
	}
	/*
		Flip a character in the payload — keeping the same length so base64
		still decodes — and verify the signature mismatch fires.
	*/
	tampered := flipFirstChar(parts[0]) + "." + parts[1]
	if _, _, err := Verify(tampered, secret); err == nil {
		t.Fatal("expected signature mismatch on tampered payload")
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	secret := testSecret()
	tok, _ := Mint(7, ActionDecline, time.Now().Add(time.Minute), secret)
	parts := strings.SplitN(tok, ".", 2)
	tampered := parts[0] + "." + flipFirstChar(parts[1])
	if _, _, err := Verify(tampered, secret); err == nil {
		t.Fatal("expected signature mismatch on tampered tag")
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	secret := testSecret()
	tok, _ := Mint(1, ActionExecute, time.Now().Add(-1*time.Second), secret)
	if _, _, err := Verify(tok, secret); err == nil {
		t.Fatal("expected expired error")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	tok, _ := Mint(1, ActionExecute, time.Now().Add(time.Minute), testSecret())
	other := make([]byte, 32)
	for i := range other {
		other[i] = byte(99 - i)
	}
	if _, _, err := Verify(tok, other); err == nil {
		t.Fatal("expected verification failure with wrong secret")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "no-dot", "one.two.three", "...", "@@@.@@@"} {
		if _, _, err := Verify(bad, testSecret()); err == nil {
			t.Errorf("expected error for malformed input %q", bad)
		}
	}
}

func TestTokenHashIsDeterministicAndDifferentPerToken(t *testing.T) {
	secret := testSecret()
	a, _ := Mint(1, ActionExecute, time.Now().Add(time.Minute), secret)
	b, _ := Mint(1, ActionDecline, time.Now().Add(time.Minute), secret)
	if TokenHash(a) == TokenHash(b) {
		t.Fatal("execute and decline tokens for same decision must hash differently")
	}
	// Re-hash the same token via two separate calls (assigned to distinct
	// vars so staticcheck doesn't flag the comparison as trivial).
	first := TokenHash(a)
	second := TokenHash(a)
	if first != second {
		t.Fatal("token hash not deterministic")
	}
}

/*
flipFirstChar swaps the first base64url character so the slice still
decodes but produces different bytes — useful for tamper tests.
*/
func flipFirstChar(s string) string {
	if len(s) == 0 {
		return s
	}
	c := s[0]
	switch {
	case c == 'A':
		c = 'B'
	case c >= 'a' && c <= 'z':
		c = 'A'
	default:
		c = 'a'
	}
	return string(c) + s[1:]
}
