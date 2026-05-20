package unsub

import (
	"crypto/rand"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return k
}

func TestSignVerify_RoundTrip(t *testing.T) {
	key := testKey(t)
	tok := Sign(key, "alice@example.com")
	if tok == "" {
		t.Fatal("Sign produced empty token")
	}
	if !Verify(key, "alice@example.com", tok) {
		t.Fatal("Verify rejected freshly-minted token")
	}
}

func TestVerify_WrongEmail(t *testing.T) {
	key := testKey(t)
	tok := Sign(key, "alice@example.com")
	if Verify(key, "bob@example.com", tok) {
		t.Fatal("token for alice should not validate for bob")
	}
}

func TestVerify_WrongKey(t *testing.T) {
	k1 := testKey(t)
	k2 := testKey(t)
	tok := Sign(k1, "alice@example.com")
	if Verify(k2, "alice@example.com", tok) {
		t.Fatal("token signed with k1 should not validate with k2")
	}
}

func TestVerify_EmptyInputs(t *testing.T) {
	key := testKey(t)
	tok := Sign(key, "alice@example.com")
	cases := []struct{ key, email, tok string }{
		{"", "alice@example.com", tok},
		{string(key), "", tok},
		{string(key), "alice@example.com", ""},
	}
	for _, tc := range cases {
		if Verify([]byte(tc.key), tc.email, tc.tok) {
			t.Errorf("Verify should reject any-empty: %+v", tc)
		}
	}
}

func TestVerify_NormalizesEmail(t *testing.T) {
	key := testKey(t)
	// Token minted for canonical form.
	tok := Sign(key, "alice@example.com")
	// Validates against whitespace + mixed case (DB rows commonly carry these).
	cases := []string{
		"  alice@example.com  ",
		"ALICE@EXAMPLE.COM",
		"Alice@Example.Com",
	}
	for _, candidate := range cases {
		if !Verify(key, candidate, tok) {
			t.Errorf("Verify should normalize %q", candidate)
		}
	}
}

/*
TestSign_Deterministic guards the property the email-link UX depends
on: the same email + key must produce the same token every time, so
users can re-use a link from any past email.
*/
func TestSign_Deterministic(t *testing.T) {
	key := testKey(t)
	a := Sign(key, "alice@example.com")
	b := Sign(key, "alice@example.com")
	if a != b {
		t.Fatalf("Sign should be deterministic, got %q vs %q", a, b)
	}
}
