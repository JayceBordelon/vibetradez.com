package config

import (
	"encoding/base64"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ResendAPIKey      string
	AnthropicAPIKey   string
	AnthropicModel    string
	EmailRecipients   []string
	EmailFrom         string
	DatabaseURL       string
	ServerPort        string
	SchwabAppKey      string
	SchwabSecret      string
	SchwabCallbackURL string
	/*
		Token-at-rest encryption key for Schwab persisted access/refresh
		tokens. Loaded from SCHWAB_TOKEN_ENCRYPTION_KEY (base64-encoded
		32 bytes — generate with `openssl rand -base64 32`). Previously
		the schwab client derived its AES key from sha256(SchwabSecret),
		which collapsed the encryption boundary onto the credential the
		token authenticates against — a DB backup + leaked secret would
		yield usable refresh tokens. The new env var is independent
		from SchwabSecret and can be rotated separately.

		Required only when SchwabAppKey + SchwabSecret are set; the
		market-data-only mode that runs without OAuth doesn't need it.
	*/
	SchwabTokenKey []byte
	/*
		HMAC key for unsubscribe-link tokens embedded in every outbound
		subscriber email. Loaded from UNSUBSCRIBE_HMAC_KEY (base64-encoded
		32 bytes — `openssl rand -base64 32`). Without it /api/unsubscribe
		was a guess-the-email mass-unsub vector. Required (mustEnv) — a
		subscriber-email pipeline without working unsub links is a CAN-SPAM
		exposure, so we refuse to boot instead of silently sending mail
		with dead links.
	*/
	UnsubscribeHMACKey []byte
	/*
		Optional list of previous UNSUBSCRIBE_HMAC_KEY values, comma-
		separated base64-encoded 32-byte secrets. Used for graceful
		rotation: the primary key signs new email links; previous keys
		still validate outstanding links in subscriber inboxes. Operator
		retires a previous key by removing it from
		UNSUBSCRIBE_HMAC_KEY_PREVIOUS (no code change required).

		Unset = no fallback (default). Production behavior unchanged
		until the operator adopts rotation.
	*/
	UnsubscribePrevHMACKeys [][]byte
	/*
		In-process Google OAuth. The trading-server runs the Google
		sign-in flow itself. GoogleCallbackURL must be registered as
		an authorized redirect URI in Google Cloud Console; mismatches
		400 with redirect_uri_mismatch. AuthDatabaseURL is a separate
		Postgres for the users + sessions tables.
	*/
	GoogleClientID     string
	GoogleClientSecret string
	GoogleCallbackURL  string
	AuthDatabaseURL    string
	SessionCookieName  string
	SessionTTLDays     int
	/*
		Auto-execution feature. TradingEnabled is the master switch; when
		false, the entire pipeline (selector, decision row, email, order)
		is dead code and no rows are ever written. When true, the manager
		routes every order through the LiveTrader (real money) against the
		Schwab Trader API. There is no paper mode: this account trades live.
	*/
	TradingEnabled bool
	PublicBaseURL  string
	// OperatorEmail receives operational alerts (e.g. the Schwab
	// refresh-token expiry nag), distinct from the subscriber list.
	OperatorEmail string
	/*
		Portfolio-manager cron schedules. The manager runs live whenever
		TradingEnabled is true.
	*/
	CronSchedulePortfolio   string
	CronScheduleRisk        string
	CronScheduleEODSnapshot string
}

/*
DefaultAnthropicModel must be refreshed from the official Anthropic Go SDK
documentation each time work touches the trade picker. It should always
point at the latest production Claude model available in the SDK at the
time of the edit. See CLAUDE.md "Model version refresh" for the policy.
*/
const DefaultAnthropicModel = "claude-opus-4-8"

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

/*
mustEnv aborts startup if the named env var is missing or empty. Required
config MUST fail fast so a container with broken env never serves traffic.
*/
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}

func Load() *Config {
	databaseURL := mustEnv("DATABASE_URL")
	resendKey := mustEnv("RESEND_API_KEY")
	anthropicKey := mustEnv("ANTHROPIC_API_KEY")
	authDatabaseURL := mustEnv("AUTH_DATABASE_URL")
	googleClientID := mustEnv("GOOGLE_CLIENT_ID")
	googleClientSecret := mustEnv("GOOGLE_CLIENT_SECRET")
	googleCallbackURL := getEnvOrDefault("GOOGLE_CALLBACK_URL", "https://vibetradez.com/auth/google/callback")

	var recipients []string
	if r := os.Getenv("EMAIL_RECIPIENTS"); r != "" {
		for _, email := range strings.Split(r, ",") {
			if trimmed := strings.TrimSpace(email); trimmed != "" {
				recipients = append(recipients, trimmed)
			}
		}
	}

	sessionTTLDays := 30
	if v := os.Getenv("SESSION_TTL_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sessionTTLDays = n
		}
	}

	unsubKeyRaw := mustEnv("UNSUBSCRIBE_HMAC_KEY")
	unsubKey, err := base64.StdEncoding.DecodeString(unsubKeyRaw)
	if err != nil || len(unsubKey) != 32 {
		log.Fatalf("UNSUBSCRIBE_HMAC_KEY must be base64-encoded 32 bytes (decoded_len=%d, err=%v)", len(unsubKey), err)
	}
	var unsubPrevKeys [][]byte
	if prev := os.Getenv("UNSUBSCRIBE_HMAC_KEY_PREVIOUS"); prev != "" {
		for _, raw := range strings.Split(prev, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			decoded, perr := base64.StdEncoding.DecodeString(raw)
			if perr != nil || len(decoded) != 32 {
				log.Fatalf("UNSUBSCRIBE_HMAC_KEY_PREVIOUS entry must be base64-encoded 32 bytes (decoded_len=%d, err=%v)", len(decoded), perr)
			}
			unsubPrevKeys = append(unsubPrevKeys, decoded)
		}
	}

	schwabAppKey := os.Getenv("SCHWAB_APP_KEY")
	schwabSecret := os.Getenv("SCHWAB_SECRET")
	var schwabTokenKey []byte
	if raw := os.Getenv("SCHWAB_TOKEN_ENCRYPTION_KEY"); raw != "" {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			log.Fatalf("SCHWAB_TOKEN_ENCRYPTION_KEY must be base64-encoded 32 bytes: %v", err)
		}
		if len(decoded) != 32 {
			log.Fatalf("SCHWAB_TOKEN_ENCRYPTION_KEY must decode to exactly 32 bytes (AES-256), got %d", len(decoded))
		}
		schwabTokenKey = decoded
	} else if schwabAppKey != "" && schwabSecret != "" {
		// Migration path: OAuth flow is enabled but the new env var is
		// not set yet. The schwab client falls back to the legacy
		// sha256(SchwabSecret) key for both encrypt and decrypt. This
		// keeps prod working on first deploy; setting the env var on
		// the next deploy switches to the new key with a transparent
		// re-encrypt of persisted tokens. Warn loudly so the operator
		// notices the gap.
		log.Println("WARN: SCHWAB_TOKEN_ENCRYPTION_KEY unset; falling back to legacy sha256(SCHWAB_SECRET) for token-at-rest. Run `openssl rand -base64 32` and set the env var to close the audit gap.")
	}

	return &Config{
		ResendAPIKey:    resendKey,
		AnthropicAPIKey: anthropicKey,
		AnthropicModel:  getEnvOrDefault("ANTHROPIC_MODEL", DefaultAnthropicModel),
		EmailRecipients: recipients,
		EmailFrom:       getEnvOrDefault("EMAIL_FROM", "Vibe Tradez <trades@vibetradez.com>"),
		DatabaseURL:     databaseURL,
		ServerPort:      getEnvOrDefault("SERVER_PORT", "8080"),
		/*
			Schwab market data is optional, live quotes degrade gracefully when
			keys are unset.
		*/
		SchwabAppKey:            schwabAppKey,
		SchwabSecret:            schwabSecret,
		SchwabCallbackURL:       getEnvOrDefault("SCHWAB_CALLBACK_URL", "https://vibetradez.com/auth/callback"),
		SchwabTokenKey:          schwabTokenKey,
		UnsubscribeHMACKey:      unsubKey,
		UnsubscribePrevHMACKeys: unsubPrevKeys,
		GoogleClientID:          googleClientID,
		GoogleClientSecret:      googleClientSecret,
		GoogleCallbackURL:       googleCallbackURL,
		AuthDatabaseURL:         authDatabaseURL,
		SessionCookieName:       getEnvOrDefault("SESSION_COOKIE_NAME", "vt_session"),
		SessionTTLDays:          sessionTTLDays,
		TradingEnabled:          os.Getenv("TRADING_ENABLED") == "true",
		PublicBaseURL:           getEnvOrDefault("PUBLIC_BASE_URL", "https://vibetradez.com"),
		OperatorEmail:           getEnvOrDefault("OPERATOR_EMAIL", "bordelonjayce@gmail.com"),
		// Portfolio agent fires ~12:30 ET: spreads have settled, the
		// morning's information is on the tape, and limit orders still
		// have hours to fill. The risk cron's remaining duty is the
		// 15-minute order-status reconcile during market hours. EOD
		// snapshot writes the equity curve at 16:00 ET.
		CronSchedulePortfolio:   getEnvOrDefault("CRON_SCHEDULE_PORTFOLIO", "30 12 * * 1-5"),
		CronScheduleRisk:        getEnvOrDefault("CRON_SCHEDULE_RISK", "*/15 10-15 * * 1-5"),
		CronScheduleEODSnapshot: getEnvOrDefault("CRON_SCHEDULE_EOD_SNAPSHOT", "0 16 * * 1-5"),
	}
}
