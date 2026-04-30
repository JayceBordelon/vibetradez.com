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
	AnthropicAPIKey    string
	AnthropicModel     string
	EmailRecipients    []string
	EmailFrom          string
	DatabaseURL        string
	ServerPort         string
	SchwabAppKey       string
	SchwabSecret       string
	SchwabCallbackURL  string
	/*
		Auth service (auth.jaycebordelon.com) client credentials. Trading
		server delegates sign-in to the centralized auth service and talks
		to it over HTTP for token exchange + session introspection.
	*/
	AuthBaseURL       string
	AuthPublicURL     string
	AuthClientID      string
	AuthClientSecret  string
	AuthRedirectURI   string
	SessionCookieName string
	SessionTTLDays    int
	/*
		Auto-execution feature. TradingEnabled is the master switch; when
		false, the entire pipeline (selector, decision row, email, order)
		is dead code and no rows are ever written. TradingMode chooses
		between PaperTrader (synthetic fills, never touches Schwab Trader
		API) and LiveTrader (real money). Default is paper, and "anything
		not literally 'live'" resolves to paper, there is no fallback to
		live on misconfiguration.
	*/
	TradingEnabled     bool
	TradingMode        string
	ExecutionRecipient string
	PublicBaseURL      string
}

/*
DefaultAnthropicModel must be refreshed from the official Anthropic Go SDK
documentation each time work touches the trade picker. It should always
point at the latest production Claude model available in the SDK at the
time of the edit. See CLAUDE.md "Model version refresh" for the policy.
*/
const DefaultAnthropicModel = "claude-opus-4-7"

/*
CurrentModelLabel is the user-facing label for the picker model. Emails,
logs, and the React app reference this constant instead of the versioned
identifier so that bumping the default doesn't require a copy sweep.
*/
const CurrentModelLabel = "Claude Latest"

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
		AnthropicAPIKey:    anthropicKey,
		AnthropicModel:     getEnvOrDefault("ANTHROPIC_MODEL", DefaultAnthropicModel),
		EmailRecipients:    recipients,
		EmailFrom:          getEnvOrDefault("EMAIL_FROM", "Vibe Tradez <trades@vibetradez.com>"),
		DatabaseURL:        databaseURL,
		ServerPort:         getEnvOrDefault("SERVER_PORT", "8080"),
		/*
			Schwab market data is optional, live quotes degrade gracefully when
			keys are unset.
		*/
		SchwabAppKey:       os.Getenv("SCHWAB_APP_KEY"),
		SchwabSecret:       os.Getenv("SCHWAB_SECRET"),
		SchwabCallbackURL:  getEnvOrDefault("SCHWAB_CALLBACK_URL", "https://vibetradez.com/auth/callback"),
		AuthBaseURL:        authBaseURL,
		AuthPublicURL:      getEnvOrDefault("VT_AUTH_PUBLIC_URL", "https://auth.jaycebordelon.com"),
		AuthClientID:       authClientID,
		AuthClientSecret:   authClientSecret,
		AuthRedirectURI:    authRedirectURI,
		SessionCookieName:  getEnvOrDefault("SESSION_COOKIE_NAME", "vt_session"),
		SessionTTLDays:     sessionTTLDays,
		TradingEnabled:     os.Getenv("TRADING_ENABLED") == "true",
		TradingMode:        resolveTradingMode(os.Getenv("TRADING_MODE")),
		ExecutionRecipient: getEnvOrDefault("EXECUTION_RECIPIENT", "bordelonjayce@gmail.com"),
		PublicBaseURL:      getEnvOrDefault("PUBLIC_BASE_URL", "https://vibetradez.com"),
	}
}

/*
resolveTradingMode collapses anything other than the literal string
"live" to "paper". This is intentional, a typo or empty env var must
never accidentally route to real-money execution.
*/
func resolveTradingMode(v string) string {
	if v == "live" {
		return "live"
	}
	return "paper"
}
