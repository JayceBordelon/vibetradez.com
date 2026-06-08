package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

type Subscriber struct {
	ID             int
	Email          string
	Name           string
	Active         bool
	CreatedAt      time.Time
	UnsubscribedAt *time.Time
}

func New(databaseURL string) (*Store, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	/*
		database/sql defaults to unlimited open connections. DO Managed
		Postgres has a hard per-database cap (25 on the cheapest tier),
		and the auth-service ping-on-every-request path means a burst of
		API requests + the live-quotes poll + a cron tick can saturate
		that cap and stall every other query. SetConnMaxLifetime also
		ensures stale TCP that survived DO pooler restarts gets cycled
		instead of returning "read: connection reset by peer" for hours.
	*/
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	/*
		Serialize concurrent migrate() invocations against the same DB
		via a session-scoped advisory lock. Postgres' CREATE TABLE IF
		NOT EXISTS check is NOT atomic with the implicit row-type
		creation: two concurrent CREATE TABLE statements for the same
		not-yet-existing table can both pass the existence check and
		then collide on pg_type_typname_nsp_index. The DO Managed DB +
		single shared test database in CI make this race visible
		whenever go test runs multiple packages in parallel (each
		store.New call hits migrate()).

		Picking a fixed int64 lock key based on a hash of the literal
		"vibetradez-migrate" so it's stable across deploys and not
		going to collide with any other advisory locks the codebase
		might use in the future. pg_advisory_unlock fires on conn
		release, but we explicitly unlock at the end for clarity.
	*/
	const migrateLockKey int64 = 7430241128549138 // hash("vibetradez-migrate") truncated to int64
	conn, err := db.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("acquire conn for migration lock: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_lock($1)", migrateLockKey); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrateLockKey)
	}()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS subscribers (
			id SERIAL PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			unsubscribed_at TIMESTAMPTZ
		);

		CREATE INDEX IF NOT EXISTS idx_subscribers_active ON subscribers(active);

		CREATE TABLE IF NOT EXISTS oauth_tokens (
			id SERIAL PRIMARY KEY,
			provider TEXT NOT NULL UNIQUE,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ DEFAULT NOW()
		);

		/*
		Schwab refresh tokens have a hard 7-day lifetime from the original
		consent. The access-token refresh path does NOT rotate them, so
		issued_at is set only on ExchangeCode and preserved across refreshes.
		reauth_nag_sent_on dedupes the daily warning email so we don't
		spam the inbox if the operator ignores the first nag.
		*/
		ALTER TABLE oauth_tokens ADD COLUMN IF NOT EXISTS refresh_token_issued_at TIMESTAMPTZ;
		ALTER TABLE oauth_tokens ADD COLUMN IF NOT EXISTS reauth_nag_sent_on DATE;

		ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS auth_user_id BIGINT;
		CREATE INDEX IF NOT EXISTS idx_subscribers_auth_user_id ON subscribers(auth_user_id);

		DROP TABLE IF EXISTS sessions;
		DROP TABLE IF EXISTS oauth_states;
		ALTER TABLE subscribers DROP COLUMN IF EXISTS user_id;
		DROP TABLE IF EXISTS users;


		CREATE TABLE IF NOT EXISTS transcripts (
			id          SERIAL PRIMARY KEY,
			date        TEXT NOT NULL,
			kind        TEXT NOT NULL,
			model       TEXT NOT NULL DEFAULT '',
			events      JSONB NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (date, kind)
		);

		CREATE INDEX IF NOT EXISTS idx_transcripts_date ON transcripts(date);
		ALTER TABLE transcripts ADD COLUMN IF NOT EXISTS usage JSONB NOT NULL DEFAULT '{}';
		ALTER TABLE transcripts ADD COLUMN IF NOT EXISTS duration_ms BIGINT NOT NULL DEFAULT 0;

		/*
		── Portfolio-manager tables ──
		These back the autonomous portfolio manager (internal/portfolio).
		See docs/DESIGN-portfolio-manager-v2.md.
		*/

		/*
		portfolio_decisions: one row per move the daily agent (or the
		intraday risk cron) committed, INCLUDING holds. Order fields are
		null for holds. This is the asset-class-agnostic generalization of
		the options-only executions table — equity and options share it.
		source distinguishes the daily agent from the risk cron's
		de-risking actions. execution_id mirrors the in-process id the tool
		layer returned (0 for holds).
		*/
		CREATE TABLE IF NOT EXISTS portfolio_decisions (
			id               SERIAL PRIMARY KEY,
			uuid             TEXT NOT NULL DEFAULT gen_random_uuid()::text,
			date             TEXT NOT NULL,
			source           TEXT NOT NULL DEFAULT 'agent',
			action           TEXT NOT NULL,
			asset_type       TEXT NOT NULL DEFAULT '',
			symbol           TEXT NOT NULL DEFAULT '',
			underlying       TEXT NOT NULL DEFAULT '',
			contract_type    TEXT NOT NULL DEFAULT '',
			strike           DOUBLE PRECISION,
			expiration       TEXT,
			quantity         DOUBLE PRECISION NOT NULL DEFAULT 0,
			limit_price      DOUBLE PRECISION NOT NULL DEFAULT 0,
			notional         DOUBLE PRECISION NOT NULL DEFAULT 0,
			mode             TEXT NOT NULL DEFAULT 'live',
			schwab_order_id  TEXT NOT NULL DEFAULT '',
			status           TEXT NOT NULL DEFAULT '',
			fill_price       DOUBLE PRECISION,
			execution_id     INTEGER NOT NULL DEFAULT 0,
			rationale        TEXT NOT NULL DEFAULT '',
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		ALTER TABLE portfolio_decisions ADD COLUMN IF NOT EXISTS uuid TEXT NOT NULL DEFAULT gen_random_uuid()::text;
		CREATE INDEX IF NOT EXISTS idx_portfolio_decisions_date ON portfolio_decisions(date);
		CREATE UNIQUE INDEX IF NOT EXISTS uq_portfolio_decisions_uuid ON portfolio_decisions(uuid);

		/*
		portfolio_sessions: one row per day capturing the agent's overall
		stance note (its written read of the book it left). Upserted by
		date so a re-run replaces the prior stance for the day.
		*/
		CREATE TABLE IF NOT EXISTS portfolio_sessions (
			date        TEXT PRIMARY KEY,
			mode        TEXT NOT NULL DEFAULT 'live',
			stance       TEXT NOT NULL DEFAULT '',
			summary      TEXT NOT NULL DEFAULT '',
			action_items TEXT NOT NULL DEFAULT '',
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		ALTER TABLE portfolio_sessions ADD COLUMN IF NOT EXISTS summary TEXT NOT NULL DEFAULT '';
		ALTER TABLE portfolio_sessions ADD COLUMN IF NOT EXISTS action_items TEXT NOT NULL DEFAULT '';

		/*
		portfolio_equity_curve: one row per day with the account's total
		equity, cash breakdown, running high-water mark, and the SPY close
		for the same day so the dashboard can plot the book against
		buy-and-hold SPY. Upserted by date (the 16:00 EOD snapshot cron
		writes it). The high-water mark is persisted rather than recomputed
		so the drawdown breaker has a stable reference even if early curve
		rows are pruned.
		*/
		CREATE TABLE IF NOT EXISTS portfolio_equity_curve (
			date             TEXT PRIMARY KEY,
			account_equity   DOUBLE PRECISION NOT NULL DEFAULT 0,
			settled_cash     DOUBLE PRECISION NOT NULL DEFAULT 0,
			unsettled_cash   DOUBLE PRECISION NOT NULL DEFAULT 0,
			high_water_mark  DOUBLE PRECISION NOT NULL DEFAULT 0,
			spy_close        DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		/*
		portfolio_positions: the held book snapshotted at the EOD cron, one
		set of rows per date. The dashboard reads the live broker book first
		and falls back to the latest snapshot here when the broker has no
		positions (broker unreachable or flat), so the holdings table still
		reflects the last known book.
		*/
		CREATE TABLE IF NOT EXISTS portfolio_positions (
			id             SERIAL PRIMARY KEY,
			date           TEXT NOT NULL,
			symbol         TEXT NOT NULL,
			underlying     TEXT NOT NULL DEFAULT '',
			asset_type     TEXT NOT NULL DEFAULT '',
			contract_type  TEXT NOT NULL DEFAULT '',
			strike         DOUBLE PRECISION,
			expiration     TEXT,
			quantity       DOUBLE PRECISION NOT NULL DEFAULT 0,
			market_value   DOUBLE PRECISION NOT NULL DEFAULT 0,
			cost_basis     DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_portfolio_positions_date ON portfolio_positions(date);
	`)
	return err
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping() error {
	return s.db.Ping()
}

// RemoveAllForTest clears all data, only for use in tests.
func (s *Store) RemoveAllForTest() {
	_, _ = s.db.Exec("DELETE FROM subscribers")
	_, _ = s.db.Exec("DELETE FROM transcripts")
	_, _ = s.db.Exec("DELETE FROM portfolio_decisions")
	_, _ = s.db.Exec("DELETE FROM portfolio_sessions")
	_, _ = s.db.Exec("DELETE FROM portfolio_equity_curve")
}

// --- Subscriber methods ---

/*
AddSubscriber is for EXPLICIT signup intent (form submit on the
website, /api/subscribe handler). It force-reactivates a previously
unsubscribed row: the user is asking to be re-added. Resets name to
the value supplied so a re-signup updates the display name.

Do NOT call this from a side-effect path (SSO login, EMAIL_RECIPIENTS
boot seed) — see EnsureSubscriberExists, which preserves opt-outs.
*/
func (s *Store) AddSubscriber(email, name string) error {
	_, err := s.db.Exec(`
		INSERT INTO subscribers (email, name, active)
		VALUES ($1, $2, true)
		ON CONFLICT (email) DO UPDATE SET
			name = EXCLUDED.name,
			active = true,
			unsubscribed_at = NULL
	`, email, name)
	if err != nil {
		return fmt.Errorf("failed to add subscriber: %w", err)
	}
	return nil
}

/*
EnsureSubscriberExists registers a subscriber for the first time but
RESPECTS any prior unsubscribe. Used by side-effect paths where the
user is not explicitly opting in:

  - EMAIL_RECIPIENTS startup seed (cmd/scanner/main.go)
  - SSO callback (a sign-in is not consent to be emailed)

On INSERT (new row) the user is created active=true. On CONFLICT the
existing active / unsubscribed_at state is preserved; only name is
refreshed when we have a non-empty name and the previous one was
empty (avoids clobbering a user-chosen display name with a seed
empty string).
*/
func (s *Store) EnsureSubscriberExists(email, name string) error {
	_, err := s.db.Exec(`
		INSERT INTO subscribers (email, name, active)
		VALUES ($1, $2, true)
		ON CONFLICT (email) DO UPDATE SET
			name = CASE
				WHEN subscribers.name = '' AND EXCLUDED.name <> '' THEN EXCLUDED.name
				ELSE subscribers.name
			END
	`, email, name)
	if err != nil {
		return fmt.Errorf("failed to ensure subscriber exists: %w", err)
	}
	return nil
}

func (s *Store) RemoveSubscriber(email string) error {
	result, err := s.db.Exec(`
		UPDATE subscribers SET active = false, unsubscribed_at = NOW()
		WHERE email = $1 AND active = true
	`, email)
	if err != nil {
		return fmt.Errorf("failed to remove subscriber: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("subscriber not found or already unsubscribed")
	}
	return nil
}

func (s *Store) GetActiveSubscribers() ([]Subscriber, error) {
	rows, err := s.db.Query(`
		SELECT id, email, name, active, created_at
		FROM subscribers WHERE active = true ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscribers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []Subscriber
	for rows.Next() {
		var sub Subscriber
		if err := rows.Scan(&sub.ID, &sub.Email, &sub.Name, &sub.Active, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan subscriber: %w", err)
		}
		result = append(result, sub)
	}
	return result, rows.Err()
}

func (s *Store) GetActiveEmails() ([]string, error) {
	subs, err := s.GetActiveSubscribers()
	if err != nil {
		return nil, err
	}
	emails := make([]string, len(subs))
	for i, sub := range subs {
		emails[i] = sub.Email
	}
	return emails, nil
}

func nullableFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// --- OAuth token methods ---

func (s *Store) SaveOAuthToken(provider, accessToken, refreshToken string, expiresAt time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO oauth_tokens (provider, access_token, refresh_token, expires_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (provider) DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			expires_at = EXCLUDED.expires_at,
			updated_at = NOW()
	`, provider, accessToken, refreshToken, expiresAt)
	return err
}

func (s *Store) GetOAuthToken(provider string) (accessToken, refreshToken string, expiresAt time.Time, err error) {
	err = s.db.QueryRow(`
		SELECT access_token, refresh_token, expires_at
		FROM oauth_tokens WHERE provider = $1
	`, provider).Scan(&accessToken, &refreshToken, &expiresAt)
	return
}

/*
MarkRefreshTokenIssued stamps the refresh-token mint time. Called only
from the OAuth authorization-code exchange (a fresh consent), never
from the access-token refresh path. The expiry-warning cron diffs
NOW() against this column to decide when to nag the operator.
*/
func (s *Store) MarkRefreshTokenIssued(provider string, issuedAt time.Time) error {
	_, err := s.db.Exec(`
		UPDATE oauth_tokens
		SET refresh_token_issued_at = $2,
		    reauth_nag_sent_on = NULL
		WHERE provider = $1
	`, provider, issuedAt)
	return err
}

/*
GetRefreshTokenIssuedAt returns the refresh-token mint time, the date
of the last re-auth nag (DATE in ET, or zero if never sent), and
ok=false when no row exists or issued_at is NULL (legacy rows pre
this migration). The cron skips the nag when ok=false.
*/
func (s *Store) GetRefreshTokenIssuedAt(provider string) (issuedAt time.Time, lastNagSentOn time.Time, ok bool, err error) {
	var issuedAtNull sql.NullTime
	var lastNagNull sql.NullTime
	err = s.db.QueryRow(`
		SELECT refresh_token_issued_at, reauth_nag_sent_on
		FROM oauth_tokens WHERE provider = $1
	`, provider).Scan(&issuedAtNull, &lastNagNull)
	if err == sql.ErrNoRows {
		return time.Time{}, time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	if !issuedAtNull.Valid {
		return time.Time{}, time.Time{}, false, nil
	}
	if lastNagNull.Valid {
		lastNagSentOn = lastNagNull.Time
	}
	return issuedAtNull.Time, lastNagSentOn, true, nil
}

func (s *Store) MarkReauthNagSent(provider string, sentOn time.Time) error {
	_, err := s.db.Exec(`
		UPDATE oauth_tokens SET reauth_nag_sent_on = $2 WHERE provider = $1
	`, provider, sentOn)
	return err
}

/*
LinkSubscriberAuthUser attaches an upstream auth user id to any
subscriber row matching this email that isn't linked yet. Does NOT
touch active or unsubscribed_at, users who previously opted out
stay opted out.
*/
func (s *Store) LinkSubscriberAuthUser(authUserID int64, email string) error {
	_, err := s.db.Exec(`
		UPDATE subscribers SET auth_user_id = $1
		WHERE email = $2 AND auth_user_id IS NULL
	`, authUserID, email)
	if err != nil {
		return fmt.Errorf("failed to link subscriber auth_user_id: %w", err)
	}
	return nil
}

// --- Transcript methods ---

/*
TranscriptView is one stored model conversation for a date + kind.
Events is the raw JSONB array (ordered transcript events) handed back
to the API layer verbatim — the store doesn't import the transcript
package, it just persists and returns the bytes.
*/
type TranscriptView struct {
	Date       string
	Kind       string
	Model      string
	Events     json.RawMessage
	Usage      json.RawMessage
	DurationMS int64
	CreatedAt  time.Time
}

/*
SaveTranscript upserts the captured conversation for a (date, kind).
A re-run of a day's cron overwrites the prior transcript so the row
always reflects the latest run. eventsJSON must be a JSON array (the
ordered transcript events); usageJSON is the session token-usage object;
durationMS is the session wall-clock; model is the model id that produced
it.
*/
func (s *Store) SaveTranscript(date, kind, model string, eventsJSON, usageJSON []byte, durationMS int64) error {
	if len(usageJSON) == 0 {
		usageJSON = []byte("{}")
	}
	_, err := s.db.Exec(`
		INSERT INTO transcripts (date, kind, model, events, usage, duration_ms, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (date, kind) DO UPDATE SET
			model = EXCLUDED.model,
			events = EXCLUDED.events,
			usage = EXCLUDED.usage,
			duration_ms = EXCLUDED.duration_ms,
			created_at = NOW()
	`, date, kind, model, eventsJSON, usageJSON, durationMS)
	if err != nil {
		return fmt.Errorf("failed to save %s transcript for %s: %w", kind, date, err)
	}
	return nil
}

/*
GetTranscript returns the stored transcript for a (date, kind), or
(nil, nil) when no row exists so the caller can render an empty state
instead of treating a missing transcript as an error.
*/
func (s *Store) GetTranscript(date, kind string) (*TranscriptView, error) {
	var tv TranscriptView
	err := s.db.QueryRow(`
		SELECT date, kind, model, events, usage, duration_ms, created_at
		FROM transcripts WHERE date = $1 AND kind = $2
	`, date, kind).Scan(&tv.Date, &tv.Kind, &tv.Model, &tv.Events, &tv.Usage, &tv.DurationMS, &tv.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query %s transcript for %s: %w", kind, date, err)
	}
	return &tv, nil
}
