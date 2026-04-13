package config

import (
	"log"
	"os"
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
	AdminKey           string
}

// DefaultOpenAIModel and DefaultAnthropicModel must be refreshed from the
// official Go SDK documentation each time work touches the trade analyzer
// or validator. They should always point at the latest production model
// available in their respective SDKs at the time of the edit. See CLAUDE.md
// "Model version refresh" for the policy.
const (
	DefaultOpenAIModel    = "gpt-5.4"
	DefaultAnthropicModel = "claude-opus-4-6"
)

// modelDisplayNames maps API model identifiers to human-friendly labels
// used in emails and log output. Update this map whenever a new default
// model is added above.
var modelDisplayNames = map[string]string{
	"gpt-5.4":           "GPT-5.4",
	"gpt-4o":            "GPT-4o",
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

func Load() *Config {
	cronOpen := os.Getenv("CRON_SCHEDULE_OPEN")
	if cronOpen == "" {
		cronOpen = "25 9 * * 1-5"
	}

	cronClose := os.Getenv("CRON_SCHEDULE_CLOSE")
	if cronClose == "" {
		cronClose = "5 16 * * 1-5"
	}

	cronWeekly := os.Getenv("CRON_SCHEDULE_WEEKLY")
	if cronWeekly == "" {
		cronWeekly = "30 16 * * 5" // Friday 4:30 PM ET (after EOD analysis at 4:05)
	}

	emailFrom := os.Getenv("EMAIL_FROM")
	if emailFrom == "" {
		emailFrom = "Vibe Tradez <trades@vibetradez.com>"
	}

	var recipients []string
	if r := os.Getenv("EMAIL_RECIPIENTS"); r != "" {
		for _, email := range strings.Split(r, ",") {
			if trimmed := strings.TrimSpace(email); trimmed != "" {
				recipients = append(recipients, trimmed)
			}
		}
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	schwabCallback := os.Getenv("SCHWAB_CALLBACK_URL")
	if schwabCallback == "" {
		schwabCallback = "https://vibetradez.com/auth/callback"
	}

	return &Config{
		CronScheduleOpen:   cronOpen,
		CronScheduleClose:  cronClose,
		CronScheduleWeekly: cronWeekly,
		ResendAPIKey:       os.Getenv("RESEND_API_KEY"),
		OpenAIAPIKey:       os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:        getEnvOrDefault("OPENAI_MODEL", DefaultOpenAIModel),
		AnthropicAPIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:     getEnvOrDefault("ANTHROPIC_MODEL", DefaultAnthropicModel),
		EmailRecipients:    recipients,
		EmailFrom:          emailFrom,
		DatabaseURL:        databaseURL,
		ServerPort:         serverPort,
		SchwabAppKey:       os.Getenv("SCHWAB_APP_KEY"),
		SchwabSecret:       os.Getenv("SCHWAB_SECRET"),
		SchwabCallbackURL:  schwabCallback,
		AdminKey:           os.Getenv("ADMIN_KEY"),
	}
}
