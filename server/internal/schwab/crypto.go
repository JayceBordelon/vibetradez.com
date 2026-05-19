package schwab

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

/*
legacyKeyFromSecret produces the 32-byte AES key the schwab client
USED to encrypt with — sha256(SCHWAB_SECRET). Retained for the
transparent-migration path: tokens persisted before
SCHWAB_TOKEN_ENCRYPTION_KEY was introduced are decrypted with this
key, then re-encrypted with the new dedicated key on next persist.
Do not use for fresh encryption — the new EncryptWithKey takes the
operator-provided 32-byte token key directly.
*/
func legacyKeyFromSecret(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

/*
EncryptWithKey encrypts plaintext with AES-256-GCM using a 32-byte
operator-provided key (SCHWAB_TOKEN_ENCRYPTION_KEY). Returns
base64-encoded ciphertext.
*/
func EncryptWithKey(plaintext string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("schwab token encryption key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

/*
DecryptWithKey decrypts a base64-encoded AES-256-GCM ciphertext using
the 32-byte key.
*/
func DecryptWithKey(encoded string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("schwab token encryption key must be 32 bytes, got %d", len(key))
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

/*
Encrypt + Decrypt are the legacy variants kept for backwards
compatibility during the SCHWAB_TOKEN_ENCRYPTION_KEY migration. New
code MUST call EncryptWithKey / DecryptWithKey with the dedicated
key. These helpers decrypt rows persisted before the migration; the
schwab Client's loader uses DecryptWithKey first and only falls
back to Decrypt when the new key fails (then re-encrypts with the
new key on first persist).
*/
func Encrypt(plaintext, secret string) (string, error) {
	return EncryptWithKey(plaintext, legacyKeyFromSecret(secret))
}

func Decrypt(encoded, secret string) (string, error) {
	return DecryptWithKey(encoded, legacyKeyFromSecret(secret))
}
