/*
Package exec implements the guarded auto-execution pipeline:
pick-of-day selection, HMAC-signed confirmation tokens, paper/live
order placement, and the 3:55pm ET mandatory close. The package is
dormant unless TRADING_ENABLED=true; even then, all order paths route
through the PaperTrader implementation unless TRADING_MODE=live.
*/
package exec

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Action discriminates the two CTAs in the confirmation email.
type Action string

const (
	ActionExecute Action = "execute"
	ActionDecline Action = "decline"
)

/*
tokenPayload is the JSON serialized into the first half of a signed
token. nonce gives every (decision_id, action) pair a unique signature
so two emails with the same decision can't share a token, and exp lets
verifiers reject expired tokens without a DB hit.
*/
type tokenPayload struct {
	DecisionID int    `json:"d"`
	Action     Action `json:"a"`
	Nonce      string `json:"n"` // hex-encoded 16 bytes
	ExpiresAt  int64  `json:"e"` // unix seconds
}

/*
Mint produces a single-use signed token for (decisionID, action) that
expires at expiresAt. The returned string is URL-safe and contains no
secret material — only the payload + an HMAC-SHA256 tag. The hash of
the token (TokenHash) must be persisted on the decision row to enforce
single-use semantics; the plaintext token must NEVER be persisted.
*/
func Mint(decisionID int, action Action, expiresAt time.Time, secret []byte) (string, error) {
	if action != ActionExecute && action != ActionDecline {
		return "", fmt.Errorf("invalid action %q", action)
	}
	if len(secret) < 32 {
		return "", errors.New("secret must be at least 32 bytes")
	}

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}

	payload := tokenPayload{
		DecisionID: decisionID,
		Action:     action,
		Nonce:      hex.EncodeToString(nonce),
		ExpiresAt:  expiresAt.Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(raw)
	tag := mac.Sum(nil)

	enc := base64.RawURLEncoding
	return enc.EncodeToString(raw) + "." + enc.EncodeToString(tag), nil
}

/*
Verify parses and validates a token. It checks the HMAC tag with
constant-time comparison, the action whitelist, and the embedded
expiry. It does NOT check single-use status — that must be done by
the caller against the persisted token_hash on the decision row.
*/
func Verify(token string, secret []byte) (decisionID int, action Action, err error) {
	if len(secret) < 32 {
		return 0, "", errors.New("secret must be at least 32 bytes")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, "", errors.New("malformed token")
	}
	enc := base64.RawURLEncoding
	raw, err := enc.DecodeString(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("decode payload: %w", err)
	}
	tag, err := enc.DecodeString(parts[1])
	if err != nil {
		return 0, "", fmt.Errorf("decode tag: %w", err)
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(raw)
	expected := mac.Sum(nil)
	if !hmac.Equal(tag, expected) {
		return 0, "", errors.New("signature mismatch")
	}

	var payload tokenPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0, "", fmt.Errorf("unmarshal payload: %w", err)
	}
	if payload.Action != ActionExecute && payload.Action != ActionDecline {
		return 0, "", fmt.Errorf("invalid action %q", payload.Action)
	}
	if time.Now().Unix() > payload.ExpiresAt {
		return 0, "", errors.New("token expired")
	}
	return payload.DecisionID, payload.Action, nil
}

/*
TokenHash is the value persisted on the decision row to detect token
reuse without storing the token itself. Two distinct tokens for the
same decision (e.g. the execute and decline tokens) hash differently
so each is independently single-use.
*/
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
