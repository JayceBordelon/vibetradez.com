package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	CronScheduleOpen   string
	CronScheduleClose  string
	CronScheduleWeekly string
	ResendAPIKey       string
	OpenAIAPIKey       string
	OpenAIModel        string
	AnthropicAPIKey    string
	AnthropicModel     string
	EmailRecipients    []string // Fallback: seed subscribers from env on first run
	EmailFrom          string
	DatabaseURL        string
	ServerPort         string
	SchwabAppKey       string
	SchwabSecret       string
	SchwabCallbackURL  string
	// Auth service (auth.jaycebordelon.com) client credentials. Trading
	// server delegates sign-in to the centralized auth service and talks
	// to it over HTTP for token exchange + session introspection.
	AuthBaseURL       string // e.g. http://auth-service:8081 (internal)
	AuthPublicURL     string // e.g. https://auth.jaycebordelon.com (browser-facing)
	AuthClientID      string
	AuthClientSecret  string
	AuthRedirectURI   string // consumer callback URL (must be registered at auth service)
	SessionCookieName string
	SessionTTLDays    int
}

// DefaultOpenAIModel and DefaultAnthropicModel must be refreshed from the
// official Go SDK documentation each time work touches the trade analyzer
// or validator. They should always point at the latest production model
// available in their respective SDKs at the time of the edit. See CLAUDE.md
// "Model version refresh" for the policy.
const (
	DefaultOpenAIModel    = "gpt-5.4"
	DefaultAnthropicModel = "claude-opus-4-7"
)

// modelDisplayNames maps API model identifiers to human-friendly labels
// used in emails and log output. Update this map whenever a new default
// model is added above.
var modelDisplayNames = map[string]string{
	"gpt-5.4":           "GPT-5.4",
	"gpt-4o":            "GPT-4o",
	"claude-opus-4-7":   "Claude Opus 4.7",
	"claude-opus-4-6":   "Claude Opus 4.6",
	"claude-sonnet-4-6": "Claude Sonnet 4.6",
}

// ModelDisplayName returns a human-friendly label for the given API model
// identifier. Falls back to the raw identifier for unknown models.
func ModelDisplayName(model string) string {
	if name, ok := modelDisplayNames[model]; ok {
		return name
	}
	return model
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// mustEnv aborts startup if the named env var is missing or empty. Required
// config MUST fail fast so a container with broken env never serves traffic.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}

func Load() *Config {
	// Required: service refuses to start without these. Keep the list in sync
	// with the .env.example / docker-compose env blocks.
	databaseURL := mustEnv("DATABASE_URL")
	resendKey := mustEnv("RESEND_API_KEY")
	openaiKey := mustEnv("OPENAI_API_KEY")
	authBaseURL := mustEnv("VT_AUTH_BASE_URL")
	authClientID := mustEnv("VT_AUTH_CLIENT_ID")
	authClientSecret := mustEnv("VT_AUTH_CLIENT_SECRET")
	authRedirectURI := mustEnv("VT_AUTH_REDIRECT_URI")

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

	return &Config{
		CronScheduleOpen:   getEnvOrDefault("CRON_SCHEDULE_OPEN", "25 9 * * 1-5"),
		CronScheduleClose:  getEnvOrDefault("CRON_SCHEDULE_CLOSE", "5 16 * * 1-5"),
		CronScheduleWeekly: getEnvOrDefault("CRON_SCHEDULE_WEEKLY", "30 16 * * 5"),
		ResendAPIKey:       resendKey,
		OpenAIAPIKey:       openaiKey,
		OpenAIModel:        getEnvOrDefault("OPENAI_MODEL", DefaultOpenAIModel),
		// Anthropic validator is optional — empty key disables Claude picking.
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:  getEnvOrDefault("ANTHROPIC_MODEL", DefaultAnthropicModel),
		EmailRecipients: recipients,
		EmailFrom:       getEnvOrDefault("EMAIL_FROM", "Vibe Tradez <trades@vibetradez.com>"),
		DatabaseURL:     databaseURL,
		ServerPort:      getEnvOrDefault("SERVER_PORT", "8080"),
		// Schwab market data is optional — live quotes degrade gracefully when
		// keys are unset. Leave this dual-var pair free-form.
		SchwabAppKey:      os.Getenv("SCHWAB_APP_KEY"),
		SchwabSecret:      os.Getenv("SCHWAB_SECRET"),
		SchwabCallbackURL: getEnvOrDefault("SCHWAB_CALLBACK_URL", "https://vibetradez.com/auth/callback"),
		AuthBaseURL:       authBaseURL,
		AuthPublicURL:     getEnvOrDefault("VT_AUTH_PUBLIC_URL", "https://auth.jaycebordelon.com"),
		AuthClientID:      authClientID,
		AuthClientSecret:  authClientSecret,
		AuthRedirectURI:   authRedirectURI,
		SessionCookieName: getEnvOrDefault("SESSION_COOKIE_NAME", "vt_session"),
		SessionTTLDays:    sessionTTLDays,
	}
}
