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
TestVerifyWithFallback_AcceptsPreviousKey covers UNSUBSCRIBE_HMAC_KEY
rotation: an email link signed under the operator's PREVIOUS key
must still validate after they swap in a new primary key, as long
as the previous key is still listed in UNSUBSCRIBE_HMAC_KEY_PREVIOUS.
Without this, every outstanding subscriber email's unsub link
breaks the moment the operator rotates the key.
*/
func TestVerifyWithFallback_AcceptsPreviousKey(t *testing.T) {
	oldKey := testKey(t)
	newKey := testKey(t)
	tok := Sign(oldKey, "alice@example.com")

	// Plain Verify against the NEW key fails (different secret).
	if Verify(newKey, "alice@example.com", tok) {
		t.Fatal("Verify with new key alone should reject token signed by old key")
	}

	// VerifyWithFallback accepts when the previous key is in the fallback list.
	if !VerifyWithFallback(newKey, [][]byte{oldKey}, "alice@example.com", tok) {
		t.Fatal("VerifyWithFallback should accept token signed by previous key")
	}
}

func TestVerifyWithFallback_PrefersPrimary(t *testing.T) {
	k1 := testKey(t)
	k2 := testKey(t)
	tok := Sign(k1, "alice@example.com")
	// Even with k2 in fallback, the primary k1 must work first.
	if !VerifyWithFallback(k1, [][]byte{k2}, "alice@example.com", tok) {
		t.Fatal("VerifyWithFallback should accept primary signature")
	}
}

func TestVerifyWithFallback_RejectsUnknownKey(t *testing.T) {
	good := testKey(t)
	other := testKey(t)
	tok := Sign(other, "alice@example.com")
	if VerifyWithFallback(good, [][]byte{testKey(t), testKey(t)}, "alice@example.com", tok) {
		t.Fatal("VerifyWithFallback should reject token not signed by primary nor any previous key")
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
