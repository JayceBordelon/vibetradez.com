package schwab

import (
	"crypto/rand"
	"testing"
)

func newRandomKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return k
}

func TestEncryptDecryptWithKey_RoundTrip(t *testing.T) {
	key := newRandomKey(t)
	plain := "fake-refresh-token-with-some-length"

	enc, err := EncryptWithKey(plain, key)
	if err != nil {
		t.Fatalf("EncryptWithKey: %v", err)
	}
	got, err := DecryptWithKey(enc, key)
	if err != nil {
		t.Fatalf("DecryptWithKey: %v", err)
	}
	if got != plain {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, plain)
	}
}

func TestDecryptWithKey_RejectsWrongKey(t *testing.T) {
	k1 := newRandomKey(t)
	k2 := newRandomKey(t)
	enc, err := EncryptWithKey("secret", k1)
	if err != nil {
		t.Fatalf("EncryptWithKey: %v", err)
	}
	if _, err := DecryptWithKey(enc, k2); err == nil {
		t.Fatal("DecryptWithKey with wrong key should fail")
	}
}

func TestEncryptWithKey_RejectsShortKey(t *testing.T) {
	if _, err := EncryptWithKey("x", []byte("too-short")); err == nil {
		t.Fatal("EncryptWithKey with <32-byte key should fail")
	}
	if _, err := DecryptWithKey("AAAA", []byte("too-short")); err == nil {
		t.Fatal("DecryptWithKey with <32-byte key should fail")
	}
}

/*
TestTryDecrypt_PrefersNewKeyFallsBackToLegacy guards the audit fix:
tokens persisted before SCHWAB_TOKEN_ENCRYPTION_KEY was introduced
must still decrypt (so the operator doesn't have to re-auth on
deploy), and the caller learns whether the legacy path was used so
it can re-encrypt under the new key.
*/
func TestTryDecrypt_PrefersNewKeyFallsBackToLegacy(t *testing.T) {
	legacySecret := "schwab-secret-from-2025"
	newKey := newRandomKey(t)

	// Legacy-encrypted ciphertext (what's already in the DB before deploy).
	legacyEnc, err := Encrypt("refresh-token", legacySecret)
	if err != nil {
		t.Fatalf("legacy Encrypt: %v", err)
	}
	// New-encrypted ciphertext (what we persist after deploy).
	newEnc, err := EncryptWithKey("refresh-token", newKey)
	if err != nil {
		t.Fatalf("EncryptWithKey: %v", err)
	}

	// New row decrypts under the new key, doesn't touch legacy.
	pt, usedLegacy, err := tryDecrypt(newEnc, newKey, legacySecret)
	if err != nil || pt != "refresh-token" || usedLegacy {
		t.Fatalf("new ciphertext: pt=%q usedLegacy=%v err=%v", pt, usedLegacy, err)
	}

	// Legacy row falls back transparently and the flag is set so the
	// caller knows to re-persist under the new key.
	pt, usedLegacy, err = tryDecrypt(legacyEnc, newKey, legacySecret)
	if err != nil || pt != "refresh-token" || !usedLegacy {
		t.Fatalf("legacy ciphertext: pt=%q usedLegacy=%v err=%v (want pt=refresh-token, usedLegacy=true)", pt, usedLegacy, err)
	}

	// Garbage ciphertext that doesn't decrypt under either key errors.
	if _, _, err := tryDecrypt("not-base64-or-anything", newKey, legacySecret); err == nil {
		t.Fatal("garbage ciphertext should error")
	}
}

func TestTryDecrypt_NoLegacySecretAvailable(t *testing.T) {
	newKey := newRandomKey(t)
	enc, _ := EncryptWithKey("x", newKey)

	// New-key path still works without a legacy secret.
	if pt, _, err := tryDecrypt(enc, newKey, ""); err != nil || pt != "x" {
		t.Fatalf("new path with empty legacy secret: pt=%q err=%v", pt, err)
	}

	// A row encrypted with the legacy key + no legacy secret = fail.
	legacyEnc, _ := Encrypt("y", "some-secret")
	if _, _, err := tryDecrypt(legacyEnc, newKey, ""); err == nil {
		t.Fatal("legacy ciphertext without legacy secret should fail")
	}
}
