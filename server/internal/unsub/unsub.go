/*
Package unsub mints and verifies HMAC-signed unsubscribe tokens that
ride in every outbound subscriber email. The token is a per-recipient
cryptographic proof of ownership — "the bearer of this token is
authorized to unsubscribe this email address" — which the unsubscribe
endpoints validate before flipping subscribers.active to false.

Without this, /api/unsubscribe was a guess-the-email mass-unsub
endpoint: anyone who knew or guessed a subscriber's email could POST
{"email":"…"} and remove them, gated only by a per-IP rate limit
that's trivially defeated.

Token shape:

	base64url-no-pad( HMAC-SHA256(key, email)[:16] )

16-byte truncation is plenty of preimage resistance (2^128) and
keeps email URLs short. The key is an operator-provided 32-byte
secret in UNSUBSCRIBE_HMAC_KEY.
*/
package unsub

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"
)

const (
	// tokenBytes is the truncated HMAC length the encoded token carries.
	// 16 bytes = 128-bit preimage resistance, well over what HMAC-SHA256
	// guarantees for the typical email-length input space.
	tokenBytes = 16
)

/*
Sign returns the URL-safe unsubscribe token for the given email. The
email is lower-cased + trimmed before hashing so a subscriber whose
DB row reads "Foo@Example.COM " can still validate the token they
got in their inbox addressed to "foo@example.com".
*/
func Sign(key []byte, email string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(normalize(email)))
	full := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(full[:tokenBytes])
}

/*
Verify returns true when token matches the expected HMAC for email.
Uses crypto/subtle.ConstantTimeCompare so a timing attacker can't
discover whether a partial token prefix is correct.

Empty key, email, or token => false. The caller's policy decides
what to do when an unsigned-mode legacy request arrives (the
endpoints reject those entirely; this function never accepts them).
*/
func Verify(key []byte, email, token string) bool {
	if len(key) == 0 || email == "" || token == "" {
		return false
	}
	want := Sign(key, email)
	return subtle.ConstantTimeCompare([]byte(want), []byte(token)) == 1
}

func normalize(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
