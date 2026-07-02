package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"vibetradez.com/internal/auth"
	"vibetradez.com/internal/calendar"
	"vibetradez.com/internal/config"
	"vibetradez.com/internal/email"
	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/marketnews"
	"vibetradez.com/internal/portfolio"
	"vibetradez.com/internal/portfoliowire"
	"vibetradez.com/internal/quotes"
	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/server"
	"vibetradez.com/internal/store"
	"vibetradez.com/internal/templates"
	"vibetradez.com/internal/transcript"
	"vibetradez.com/internal/unsub"

	"github.com/robfig/cron/v3"
)

// Market calendar moved to internal/calendar. The streaming hub + SSE
// handlers consume the same source. Update the lists there yearly each
// Q4 from NYSE's published schedule.

/*
isMarketOpen reports whether the market is in its live session RIGHT NOW
(9:30 ET to the day's close, where the close is 13:00 on half-days and
16:00 otherwise). It is time-of-day and half-day aware via
calendar.CurrentStatus, so the session and risk-sweep crons don't fire
against a closed market (e.g. a risk sweep at 13:15-15:45 on a half-day
that closed at 13:00). The second return is a short reason for the skip log.

Do NOT gate the EOD snapshot on this: that cron fires at 16:00, at or after
the close, so it must use isTradingDay instead.
*/
func isMarketOpen() (bool, string) {
	now := time.Now()
	if calendar.CurrentStatus(now).Open {
		return true, ""
	}
	today := now.In(calendar.ETLocation).Format("2006-01-02")
	if holiday := calendar.IsHoliday(today); holiday != "" {
		return false, holiday
	}
	if !calendar.IsTradingDay(today) {
		return false, "Weekend"
	}
	if hd := calendar.IsHalfDay(today); hd != "" {
		return false, "Outside half-day session (1:00 PM ET close, " + hd + ")"
	}
	return false, "Outside session (9:30 AM-4:00 PM ET)"
}

/*
isTradingDay reports whether today is a NYSE trading day in ET (weekday and
not a full-closure holiday), ignoring time of day. Used by the EOD snapshot
cron, which fires at 16:00 (after the close) and so cannot use isMarketOpen.
*/
func isTradingDay() (bool, string) {
	today := time.Now().In(calendar.ETLocation).Format("2006-01-02")
	if holiday := calendar.IsHoliday(today); holiday != "" {
		return false, holiday
	}
	if !calendar.IsTradingDay(today) {
		return false, "Weekend"
	}
	return true, ""
}

func todayDate() string {
	loc, _ := time.LoadLocation("America/New_York")
	return time.Now().In(loc).Format("2006-01-02")
}

/*
checkClockSkew probes Cloudflare's HTTP Date header (NTP-disciplined
within the millisecond) and compares against the local clock. Logs a
warning if drift exceeds 5 seconds. Run from a goroutine on startup so
a slow probe doesn't delay boot. Failures (network, parse) are silent.
*/
func checkClockSkew() {
	const maxAcceptableSkew = 5 * time.Second
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("HEAD", "https://1.1.1.1", nil)
	if err != nil {
		return
	}
	beforeReq := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("clock-skew probe: HEAD failed: %v (skipping)", err)
		return
	}
	rtt := time.Since(beforeReq)
	defer func() { _ = resp.Body.Close() }()

	dateHeader := resp.Header.Get("Date")
	if dateHeader == "" {
		log.Printf("clock-skew probe: no Date header in response (skipping)")
		return
	}
	remote, err := http.ParseTime(dateHeader)
	if err != nil {
		log.Printf("clock-skew probe: parse Date %q: %v (skipping)", dateHeader, err)
		return
	}
	estimatedRemoteAtReceive := remote.Add(rtt / 2)
	skew := time.Since(estimatedRemoteAtReceive)
	if skew < 0 {
		skew = -skew
	}
	if skew > maxAcceptableSkew {
		log.Printf("clock-skew WARNING: local clock differs from cloudflare by %s (threshold %s). The portfolio crons (the open/midday/pre-close sessions, the every-15-min risk sweep, and the 16:00 ET EOD snapshot) WILL fire at the wrong wall-clock time", skew.Truncate(time.Second), maxAcceptableSkew)
	} else {
		log.Printf("clock-skew probe: local clock within %s of cloudflare (rtt=%s)", skew.Truncate(time.Millisecond), rtt.Truncate(time.Millisecond))
	}
}

/*
isLocalStubKey detects the placeholder API keys used by the local Docker
stack so the cron can be safely skipped without making real API calls.
*/
/*
unsubURLBuilder returns the per-recipient unsubscribe URL builder
used by every subscriber-facing email send. Tokens are deterministic
(same email + same key always produces the same URL), so links from
old emails stay valid forever — exactly the property a one-click
unsubscribe needs.
*/
func unsubURLBuilder(cfg *config.Config) func(string) string {
	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	return func(email string) string {
		tok := unsub.Sign(cfg.UnsubscribeHMACKey, email)
		return fmt.Sprintf("%s/auth/unsubscribe?e=%s&t=%s",
			base, url.QueryEscape(email), url.QueryEscape(tok))
	}
}

func isLocalStubKey(k string) bool {
	if k == "" {
		return false
	}
	switch {
	case len(k) >= 5 && k[:5] == "stub-":
		return true
	case len(k) >= 8 && k[:8] == "sk_local":
		return true
	case len(k) >= 8 && k[:8] == "sk-local":
		return true
	}
	return false
}

func main() {
	cfg := config.Load()

	db, err := store.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if len(cfg.EmailRecipients) > 0 {
		for _, email := range cfg.EmailRecipients {
			// EnsureSubscriberExists preserves prior unsubscribes;
			// AddSubscriber would silently re-subscribe opted-out
			// addresses on every container restart.
			if err := db.EnsureSubscriberExists(email, ""); err != nil {
				log.Printf("Warning: failed to seed subscriber %s: %v", email, err)
			}
		}
		log.Printf("Seeded %d subscribers from EMAIL_RECIPIENTS", len(cfg.EmailRecipients))
	}

	var schwabClient *schwab.Client
	if cfg.SchwabAppKey != "" && cfg.SchwabSecret != "" {
		schwabClient = schwab.NewClient(cfg.SchwabAppKey, cfg.SchwabSecret, cfg.SchwabCallbackURL, cfg.SchwabTokenKey, db)
		if schwabClient.IsConnected() {
			log.Println("Schwab: connected (tokens loaded)")
		} else {
			log.Printf("Schwab: configured but not authorized, visit https://vibetradez.com/auth/schwab to connect")
		}
	} else {
		log.Println("Schwab: not configured (SCHWAB_APP_KEY / SCHWAB_SECRET not set)")
	}

	// Open the dedicated AUTH_DATABASE_URL pool (users + sessions),
	// wire it to the Google OAuth client, and hand the resulting
	// *auth.Service to the HTTP server.
	authStore, err := auth.NewStore(cfg.AuthDatabaseURL)
	if err != nil {
		log.Fatalf("auth store: %v", err)
	}
	defer func() { _ = authStore.Close() }()
	googleClient := auth.NewGoogleClient(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleCallbackURL)
	authService := auth.New(authStore, googleClient, auth.Options{
		SessionTTL: time.Duration(cfg.SessionTTLDays) * 24 * time.Hour,
	})

	emailClient := email.NewClient(cfg.ResendAPIKey)

	/*
		Broker service. This account trades live: every order routes through
		the Schwab Trader API with real money. It needs both TRADING_ENABLED
		and a configured Schwab client (no client = no broker to trade
		through, e.g. local dev), otherwise the executor stays nil and the
		dashboard serves the last snapshot.
	*/
	var executor *exec.Service
	switch {
	case cfg.TradingEnabled && schwabClient != nil:
		trader := exec.NewLiveTrader(schwabClient)
		log.Printf("execution: LIVE, real-money orders will be placed against the Schwab Trader API")
		executor = exec.NewService(trader, exec.ServiceConfig{
			SchwabAccountHash: trader.AccountHash,
		})
	case cfg.TradingEnabled:
		log.Printf("execution: TRADING_ENABLED but Schwab is not configured, broker NOT wired (no live orders possible)")
	}

	/*
		The autonomous portfolio manager. Built whenever the broker is wired
		(TRADING_ENABLED) and trades live with real money.
	*/
	var (
		portfolioAgent  *portfolio.Agent
		portfolioReader *portfoliowire.Reader
	)
	if executor != nil {
		portfolioReader = portfoliowire.NewReader(schwabClient, executor, db, marketnews.NewClient())
		portfolioExecutor := portfoliowire.NewExecutor(executor)
		portfolioAgent = portfolio.NewAgent(cfg.AnthropicAPIKey, cfg.AnthropicModel, portfolioReader, portfolioExecutor)
		portfolioAgent.SetRecapSender(&recapEmailSender{cfg: cfg, db: db, client: emailClient})
		log.Printf("portfolio: manager ready (model=%s, mode=live)", cfg.AnthropicModel)
	} else {
		log.Printf("portfolio: manager NOT started (needs TRADING_ENABLED=true and a configured Schwab client to trade live)")
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("Failed to load timezone: %v", err)
	}
	c := cron.New(cron.WithLocation(loc))

	/*
		Portfolio crons: three decision sessions a day (open ~9:45, midday
		~12:30, pre-close ~15:30 ET), the intraday risk sweep (every 15 min,
		market hours), and the EOD equity-curve snapshot (16:00 ET).
	*/
	if portfolioAgent != nil {
		// One job per intraday slot. Each is gated on the live market window,
		// so the pre-close slot is correctly skipped on half-days (13:00
		// close) where 15:30 is already after hours.
		portfolioSlots := []struct {
			schedule string
			slot     string
		}{
			{cfg.CronSchedulePortfolioOpen, "open"},
			{cfg.CronSchedulePortfolioMidday, "midday"},
			{cfg.CronSchedulePortfolioClose, "close"},
		}
		riskJob := func() {
			if open, _ := isMarketOpen(); !open {
				return
			}
			runPortfolioRisk(db, executor)
		}
		eodSnapshotJob := func() {
			// Gated on the trading DAY, not the live session: this cron fires
			// at 16:00, at or after the close, so isMarketOpen would always
			// skip it. We still want the snapshot on every trading day
			// (including half-days, where the close was 13:00).
			if ok, reason := isTradingDay(); !ok {
				log.Printf("Skipping portfolio EOD snapshot: not a trading day (%s)", reason)
				return
			}
			// Settle the decision log's order statuses for the day before
			// the snapshot + recap email read it.
			func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				reconcileOrderStatuses(ctx, db, executor)
			}()
			runPortfolioEODSnapshot(db, portfolioReader)
		}
		for _, ps := range portfolioSlots {
			ps := ps
			job := func() {
				if open, reason := isMarketOpen(); !open {
					log.Printf("Skipping portfolio %s session: market closed (%s)", ps.slot, reason)
					return
				}
				runPortfolioSession(db, portfolioAgent, ps.slot)
			}
			if _, err := c.AddFunc(ps.schedule, job); err != nil {
				log.Fatalf("Failed to add portfolio %s session cron job: %v", ps.slot, err)
			}
		}
		if _, err := c.AddFunc(cfg.CronScheduleRisk, riskJob); err != nil {
			log.Fatalf("Failed to add portfolio risk cron job: %v", err)
		}
		if _, err := c.AddFunc(cfg.CronScheduleEODSnapshot, eodSnapshotJob); err != nil {
			log.Fatalf("Failed to add portfolio EOD snapshot cron job: %v", err)
		}
		log.Printf("portfolio: crons registered (open=%s, midday=%s, close=%s, risk=%s, eod=%s)", cfg.CronSchedulePortfolioOpen, cfg.CronSchedulePortfolioMidday, cfg.CronSchedulePortfolioClose, cfg.CronScheduleRisk, cfg.CronScheduleEODSnapshot)
	}

	// Daily Schwab refresh-token expiry warning (12:00 ET).
	if _, err := c.AddFunc("0 12 * * *", func() {
		checkSchwabReauth(cfg, db, emailClient)
	}); err != nil {
		log.Fatalf("Failed to add Schwab re-auth nag cron: %v", err)
	}

	c.Start()

	sessionTTL := time.Duration(cfg.SessionTTLDays) * 24 * time.Hour
	srv := server.New(db, schwabClient, authService, emailClient, cfg.EmailFrom, cfg.AnthropicAPIKey, cfg.AnthropicModel, cfg.SessionCookieName, sessionTTL, cfg.ServerPort, executor, cfg.UnsubscribeHMACKey, cfg.UnsubscribePrevHMACKeys, cfg.PublicBaseURL)

	/*
		Streaming quotes: fan the Schwab WebSocket out to dashboard SSE
		subscribers so the UI re-prices the book tick by tick. Needs both a
		Schwab client (the stream) and the executor (the held book that
		decides which symbols to subscribe). The hub starts the upstream
		stream on the first watcher and stops it when the last one leaves.
	*/
	if schwabClient != nil && executor != nil {
		hub := quotes.New(
			func() quotes.TickSource { return schwab.NewStreamClient(schwabClient) },
			func(ctx context.Context) ([]string, []string) {
				positions, err := executor.GetPositionsAgent(ctx)
				if err != nil {
					log.Printf("quotes hub: positions for symbol set: %v", err)
					return nil, nil
				}
				var equities, options []string
				seen := map[string]bool{}
				for _, p := range positions {
					if u := strings.ToUpper(strings.TrimSpace(p.Underlying)); u != "" && !seen[u] {
						seen[u] = true
						equities = append(equities, u)
					}
					if p.AssetType == "OPTION" {
						options = append(options, p.Symbol)
					}
				}
				return equities, options
			})
		srv.SetQuotesHub(hub)
	}
	go srv.Start()

	log.Printf("VibeTradez portfolio manager started (API :%s)", cfg.ServerPort)
	if portfolioAgent != nil {
		log.Printf("Portfolio session schedule: open=%s midday=%s close=%s (ET, mode=live)", cfg.CronSchedulePortfolioOpen, cfg.CronSchedulePortfolioMidday, cfg.CronSchedulePortfolioClose)
	}

	go checkClockSkew()

	if subs, err := db.GetActiveSubscribers(); err == nil {
		log.Printf("Active subscribers: %d", len(subs))
	}

	if os.Getenv("RUN_ON_START") == "true" && portfolioAgent != nil {
		log.Println("Running initial portfolio session...")
		runPortfolioSession(db, portfolioAgent, "midday")
	}

	// One-time product updates. Each is self-gating via the sent_emails
	// ledger, so it fires exactly once (the first boot with subscribers)
	// and never again, no trigger flag needed.
	sendAnalysisWindowUpdate(cfg, db, emailClient)
	sendLiveTradingUpdate(cfg, db, emailClient)
	sendOptionsOnlyUpdate(cfg, db, emailClient)
	sendPersonaUpdate(cfg, db, emailClient)
	sendFableReturnUpdate(cfg, db, emailClient)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	c.Stop()
}

/*
checkSchwabReauth emails the operator when the Schwab refresh token
is within 2 days of Schwab's hard 7-day cap. ExecutionRecipient only;
dedupes per ET calendar date via the reauth_nag_sent_on column so the
operator gets at most one nag per day no matter how the cron fires
or how often the container restarts. Skips silently when no issuance
timestamp is on file (legacy row pre-migration, or no token at all).
*/
func checkSchwabReauth(cfg *config.Config, db *store.Store, emailClient *email.Client) {
	const (
		schwabRefreshTokenLifetime = 7 * 24 * time.Hour
		warnThreshold              = 5 * 24 * time.Hour
	)

	if isLocalStubKey(cfg.ResendAPIKey) {
		return
	}
	if cfg.OperatorEmail == "" {
		return
	}

	issuedAt, lastNag, ok, err := db.GetRefreshTokenIssuedAt("schwab")
	if err != nil {
		log.Printf("schwab-reauth: failed to read issuance: %v", err)
		return
	}
	if !ok {
		return
	}

	age := time.Since(issuedAt)
	if age < warnThreshold {
		return
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("schwab-reauth: failed to load ET timezone: %v", err)
		return
	}
	todayET := time.Now().In(loc)
	// lastNag is the stored ET date read back as midnight UTC (DATE column),
	// so compare its UTC calendar day against today's ET day directly — do
	// NOT convert it back into ET, which would shift it to the prior day and
	// defeat the once-per-day dedupe. See store.MarkReauthNagSent.
	if !lastNag.IsZero() && sameDate(lastNag, todayET) {
		return
	}

	daysOld := int(age.Hours() / 24)
	remaining := schwabRefreshTokenLifetime - age
	daysRemaining := int(remaining.Hours() / 24)
	if remaining < 0 {
		daysRemaining = 0
	}

	expiresAt := issuedAt.Add(schwabRefreshTokenLifetime).In(loc)
	html, err := templates.RenderSchwabReauth(templates.SchwabReauthData{
		Subject:       "Schwab re-authorization needed",
		Date:          todayET.Format("Monday, Jan 2, 2006 3:04 PM"),
		IssuedAt:      issuedAt.In(loc).Format("Mon, Jan 2, 2006 3:04 PM MST"),
		ExpiresAt:     expiresAt.Format("Mon, Jan 2, 2006 3:04 PM MST"),
		DaysOld:       daysOld,
		DaysRemaining: daysRemaining,
		ReauthURL:     "https://vibetradez.com/auth/schwab",
	})
	if err != nil {
		log.Printf("schwab-reauth: render failed: %v", err)
		return
	}

	subject := fmt.Sprintf("VibeTradez: Schwab re-auth needed (%d day(s) remaining)", daysRemaining)
	if err := emailClient.SendTradeEmail(cfg.EmailFrom, []string{cfg.OperatorEmail}, subject, html); err != nil {
		log.Printf("schwab-reauth: send failed: %v", err)
		return
	}
	if err := db.MarkReauthNagSent("schwab", todayET); err != nil {
		log.Printf("schwab-reauth: failed to record nag sent: %v", err)
	}
	log.Printf("schwab-reauth: nag sent to %s (age=%dd remaining=%dd)", cfg.OperatorEmail, daysOld, daysRemaining)
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func getRecipients(db *store.Store) []string {
	emails, err := db.GetActiveEmails()
	if err != nil {
		log.Printf("Error getting subscribers: %v", err)
		return nil
	}
	return emails
}

/*
saveTranscript persists a captured model conversation for the
dashboard's /transcript/<date>/<kind> view. Best-effort: a nil
transcript (capture disabled / no run) and any marshal or DB error are
logged and swallowed so a transcript write can never break a trading
session, the EOD snapshot, or the recap. kind is "portfolio" for the
merged daily session (the only kind written now).
*/
func saveTranscript(db *store.Store, date, kind string, tr *transcript.Transcript) {
	if tr == nil {
		return
	}
	eventsJSON, err := json.Marshal(tr.Events)
	if err != nil {
		log.Printf("transcript: marshal %s for %s failed: %v", kind, date, err)
		return
	}
	usageJSON, err := json.Marshal(tr.Usage)
	if err != nil {
		log.Printf("transcript: marshal %s usage for %s failed: %v", kind, date, err)
		usageJSON = []byte("{}")
	}
	if err := db.SaveTranscript(date, kind, tr.Model, eventsJSON, usageJSON, tr.DurationMS); err != nil {
		log.Printf("transcript: save %s for %s failed: %v", kind, date, err)
		return
	}
	log.Printf("transcript: saved %s for %s (%d events, %d output tokens, %dms)", kind, date, len(tr.Events), tr.Usage.OutputTokens, tr.DurationMS)
}

/*
saveSessionTranscript persists one intraday session's transcript into the
day's single "portfolio" row. The three daily sessions (open / midday /
close) are merged in order so /transcripts/<date> renders the whole day as
one continuous reasoning log, each session preceded by a labeled separator.
The first session of the day merges into an empty prior, which just labels
it. Best-effort: a read or unmarshal failure falls back to the merge against
whatever prior state we recovered, so a transcript is never lost.
*/
func saveSessionTranscript(db *store.Store, date, slot string, tr *transcript.Transcript) {
	if tr == nil {
		return
	}
	sep := portfolioSessionSeparator(slot)
	var prior transcript.Transcript
	if view, err := db.GetTranscript(date, "portfolio"); err != nil {
		log.Printf("transcript: read prior portfolio for %s failed: %v", date, err)
	} else if view != nil {
		prior.Model = view.Model
		prior.DurationMS = view.DurationMS
		if uerr := json.Unmarshal(view.Events, &prior.Events); uerr != nil {
			log.Printf("transcript: unmarshal prior portfolio events for %s failed: %v", date, uerr)
		}
		if len(view.Usage) > 0 {
			if uerr := json.Unmarshal(view.Usage, &prior.Usage); uerr != nil {
				log.Printf("transcript: unmarshal prior portfolio usage for %s failed: %v", date, uerr)
			}
		}
	}
	merged := transcript.Merge(prior, *tr, sep)
	saveTranscript(db, date, "portfolio", &merged)
}

// portfolioSessionSeparator labels each intraday session inside the merged
// daily transcript so a reader can see where the open / midday / pre-close
// passes begin.
func portfolioSessionSeparator(slot string) string {
	switch slot {
	case "open":
		return "Open session · 9:45 AM ET"
	case "close":
		return "Pre-close session · 3:30 PM ET"
	default:
		return "Midday session · 12:30 PM ET"
	}
}
