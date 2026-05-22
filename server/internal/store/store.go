package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"vibetradez.com/internal/trades"

	_ "github.com/lib/pq"
)

/*
ErrTradesAlreadyExecuted is returned by SaveMorningTrades when the
date already has executions referencing its trade rows. The morning
cron must not rewrite trade ids out from under the executor — those
rows reflect real broker activity (paper or live) and the executions
table's FK would dangle. Callers should treat this as "the cron
already ran today" and bail out of the rest of the morning flow.
*/
var ErrTradesAlreadyExecuted = errors.New("morning trades already have executions for this date")

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
		CREATE TABLE IF NOT EXISTS trades (
			id SERIAL PRIMARY KEY,
			date TEXT NOT NULL,
			symbol TEXT NOT NULL,
			contract_type TEXT NOT NULL,
			strike_price DOUBLE PRECISION NOT NULL,
			expiration TEXT NOT NULL,
			dte INTEGER NOT NULL,
			estimated_price DOUBLE PRECISION NOT NULL,
			thesis TEXT NOT NULL DEFAULT '',
			sentiment_score DOUBLE PRECISION NOT NULL DEFAULT 0,
			current_price DOUBLE PRECISION NOT NULL DEFAULT 0,
			target_price DOUBLE PRECISION NOT NULL DEFAULT 0,
			stop_loss DOUBLE PRECISION NOT NULL DEFAULT 0,
			risk_level TEXT NOT NULL DEFAULT '',
			catalyst TEXT NOT NULL DEFAULT '',
			mention_count INTEGER NOT NULL DEFAULT 0,
			rank INTEGER NOT NULL DEFAULT 0,
			score INTEGER NOT NULL DEFAULT 0,
			rationale TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		/*
		Rename pre-refactor claude_* columns to drop the prefix so historical
		Claude rationales survive the OpenAI removal. Wrapped in a DO block so
		each rename is idempotent on already-migrated rows.
		*/
		DO $$
		BEGIN
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='trades' AND column_name='claude_score')
				AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='trades' AND column_name='score') THEN
				ALTER TABLE trades RENAME COLUMN claude_score TO score;
			END IF;
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='trades' AND column_name='claude_rationale')
				AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='trades' AND column_name='rationale') THEN
				ALTER TABLE trades RENAME COLUMN claude_rationale TO rationale;
			END IF;
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='trades' AND column_name='claude_model')
				AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='trades' AND column_name='model') THEN
				ALTER TABLE trades RENAME COLUMN claude_model TO model;
			END IF;
		END $$;

		ALTER TABLE trades ADD COLUMN IF NOT EXISTS score INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE trades ADD COLUMN IF NOT EXISTS rationale TEXT NOT NULL DEFAULT '';
		ALTER TABLE trades ADD COLUMN IF NOT EXISTS model TEXT NOT NULL DEFAULT '';

		/*
		Decouple contract-selection from ticker-selection. The picker (9:25 ET,
		pre-bell) saves symbol + direction + contract intent (target_otm_pct,
		min_dte) only. The executor (9:30:00 ET, post-bell) resolves each pick
		against the now-live option chain, writing strike_price / expiration /
		dte / estimated_price / stop_loss in place. Until that resolution
		runs, those five columns are NULL.

		Pre-bell chain quotes are frozen at yesterday's 4:00 PM ET close (US
		listed options don't trade pre-market today), so the 9:25 picker had
		no way to price a real contract. This is the schema half of the fix.
		*/
		ALTER TABLE trades ALTER COLUMN strike_price DROP NOT NULL;
		ALTER TABLE trades ALTER COLUMN expiration DROP NOT NULL;
		ALTER TABLE trades ALTER COLUMN dte DROP NOT NULL;
		ALTER TABLE trades ALTER COLUMN estimated_price DROP NOT NULL;
		ALTER TABLE trades ALTER COLUMN stop_loss DROP NOT NULL;
		ALTER TABLE trades ADD COLUMN IF NOT EXISTS target_otm_pct DOUBLE PRECISION;
		ALTER TABLE trades ADD COLUMN IF NOT EXISTS min_dte INTEGER;

		ALTER TABLE trades DROP COLUMN IF EXISTS gpt_score;
		ALTER TABLE trades DROP COLUMN IF EXISTS gpt_rationale;
		ALTER TABLE trades DROP COLUMN IF EXISTS gpt_model;
		ALTER TABLE trades DROP COLUMN IF EXISTS gpt_rank;
		ALTER TABLE trades DROP COLUMN IF EXISTS gpt_verdict;
		ALTER TABLE trades DROP COLUMN IF EXISTS claude_verdict;
		ALTER TABLE trades DROP COLUMN IF EXISTS claude_rank;
		ALTER TABLE trades DROP COLUMN IF EXISTS combined_score;
		ALTER TABLE trades DROP COLUMN IF EXISTS picked_by_openai;
		ALTER TABLE trades DROP COLUMN IF EXISTS picked_by_claude;

		CREATE INDEX IF NOT EXISTS idx_trades_date ON trades(date);

		CREATE TABLE IF NOT EXISTS summaries (
			id SERIAL PRIMARY KEY,
			date TEXT NOT NULL,
			symbol TEXT NOT NULL,
			contract_type TEXT NOT NULL,
			strike_price DOUBLE PRECISION NOT NULL,
			expiration TEXT NOT NULL,
			entry_price DOUBLE PRECISION NOT NULL,
			closing_price DOUBLE PRECISION NOT NULL,
			stock_open DOUBLE PRECISION NOT NULL,
			stock_close DOUBLE PRECISION NOT NULL,
			notes TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_summaries_date ON summaries(date);

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

		/*
		Auto-execution pipeline. The cron fires the basket of qualifying
		picks every weekday at 9:30 ET, no user confirmation step. The
		selector fires one contract per rank 1..3 (per-contract premium
		cap as a safety filter), so a row in this table represents one
		order lifecycle (open or close). requested_quantity is always 1
		under the current selector but may be > 1 on legacy rows that
		pre-date the top-3-only rewrite.
		*/
		CREATE TABLE IF NOT EXISTS executions (
			id                  SERIAL PRIMARY KEY,
			trade_id            INTEGER REFERENCES trades(id),
			mode                TEXT NOT NULL,
			side                TEXT NOT NULL,
			schwab_order_id     TEXT,
			status              TEXT NOT NULL,
			fill_price          DOUBLE PRECISION,
			filled_quantity     INTEGER NOT NULL DEFAULT 0,
			requested_quantity  INTEGER NOT NULL,
			submitted_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			filled_at           TIMESTAMPTZ,
			error_message       TEXT NOT NULL DEFAULT '',
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		/*
		Migrate the legacy executions schema (decision_id reference) to
		the new trade_id reference. Drop the FK first so subsequent
		work isn't blocked by the constraint checker, then ensure
		trade_id exists, copy any decision_id values into trade_id
		(safe even if both columns coexist in a snapshot from a
		partially-migrated state), and only THEN drop decision_id.
		The earlier shape had a rename branch that could lose data if
		both columns ever existed simultaneously; this shape preserves
		any non-null decision_id row regardless of how the DB arrived
		at the pre-migration state. Safe to run on a fresh DB; every
		step is idempotent.
		*/
		ALTER TABLE executions DROP CONSTRAINT IF EXISTS executions_decision_id_fkey;
		ALTER TABLE executions ADD COLUMN IF NOT EXISTS trade_id INTEGER;
		DO $$
		BEGIN
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='executions' AND column_name='decision_id') THEN
				UPDATE executions SET trade_id = decision_id WHERE trade_id IS NULL AND decision_id IS NOT NULL;
			END IF;
		END $$;
		ALTER TABLE executions DROP COLUMN IF EXISTS decision_id;
		DROP INDEX IF EXISTS idx_executions_decision_id;
		DROP TABLE IF EXISTS execution_decisions;

		CREATE INDEX IF NOT EXISTS idx_executions_trade_id ON executions(trade_id);
		CREATE INDEX IF NOT EXISTS idx_executions_open_pending
			ON executions(status) WHERE status IN ('pending','working');

		CREATE TABLE IF NOT EXISTS sent_rollouts (
			slug             TEXT PRIMARY KEY,
			sent_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			recipient_count  INTEGER NOT NULL
		);
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
	_, _ = s.db.Exec("DELETE FROM trades")
	_, _ = s.db.Exec("DELETE FROM summaries")
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

// --- Trade methods ---

/*
SaveMorningTrades replaces all rows for `date` with `tradeList` and
populates each Trade's ID field with the inserted row id so the
caller can pass it to the executor.

If any executions exist for the date's existing trade rows, returns
ErrTradesAlreadyExecuted instead of rewriting them. The morning cron
firing twice (RUN_ON_START=true left on, container restart at 9:29:55
ET, DST edge) would otherwise wipe the trade ids the executor's
in-flight rows point at, breaking the dashboard view of broker
truth. The caller is expected to log + bail; today's picks are
already in flight.
*/
func (s *Store) SaveMorningTrades(date string, tradeList []trades.Trade) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	/*
		Per-date Postgres advisory lock — closes the cross-process race
		the executions-exist guard alone cannot close. Default tx
		isolation is READ COMMITTED, so two concurrent
		SaveMorningTrades calls (blue/green deploy, leftover compose,
		RUN_ON_START race) would both COUNT=0 and both proceed to
		DELETE/INSERT. pg_try_advisory_xact_lock returns false when
		another tx already holds the lock; we treat that as
		ErrTradesAlreadyExecuted so the caller short-circuits the
		entire morning flow (email + executor) the same way it does
		for the post-execution case.

		Lock key derived via hashtext() of a per-date string so two
		different dates can save concurrently. The lock is xact-scoped
		so it releases automatically on COMMIT or ROLLBACK — no manual
		unlock path to leak.
	*/
	var locked bool
	if err := tx.QueryRow(`SELECT pg_try_advisory_xact_lock(hashtext($1))`, "vt_morning_trades:"+date).Scan(&locked); err != nil {
		return fmt.Errorf("failed to acquire morning-trades advisory lock: %w", err)
	}
	if !locked {
		return ErrTradesAlreadyExecuted
	}

	var execCount int
	if err := tx.QueryRow(`
		SELECT COUNT(*) FROM executions e
		INNER JOIN trades t ON t.id = e.trade_id
		WHERE t.date = $1
	`, date).Scan(&execCount); err != nil {
		return fmt.Errorf("failed to check executions for date: %w", err)
	}
	if execCount > 0 {
		return ErrTradesAlreadyExecuted
	}

	if _, err := tx.Exec("DELETE FROM trades WHERE date = $1", date); err != nil {
		return fmt.Errorf("failed to clear existing trades: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO trades (
			date, symbol, contract_type, strike_price, expiration, dte,
			estimated_price, thesis, sentiment_score, current_price,
			target_price, stop_loss, risk_level,
			catalyst, mention_count, rank,
			score, rationale, model,
			target_otm_pct, min_dte
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		RETURNING id
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for i := range tradeList {
		t := &tradeList[i]
		err := stmt.QueryRow(
			date, t.Symbol, t.ContractType,
			nullableFloat(t.StrikePrice), nullableString(t.Expiration), nullableIntPtr(t.DTE),
			nullableFloat(t.EstimatedPrice), t.Thesis, t.SentimentScore, t.CurrentPrice,
			t.TargetPrice, nullableFloat(t.StopLoss), t.RiskLevel,
			t.Catalyst, t.MentionCount, t.Rank,
			t.Score, t.Rationale, t.Model,
			t.TargetOTMPct, t.MinDTE,
		).Scan(&t.ID)
		if err != nil {
			return fmt.Errorf("failed to insert trade %s: %w", t.Symbol, err)
		}
	}

	return tx.Commit()
}

func (s *Store) GetMorningTrades(date string) ([]trades.Trade, error) {
	rows, err := s.db.Query(`
		SELECT id, symbol, contract_type, strike_price, expiration, dte,
			estimated_price, thesis, sentiment_score, current_price,
			target_price, stop_loss, risk_level,
			catalyst, mention_count, rank,
			score, rationale, model,
			target_otm_pct, min_dte
		FROM trades WHERE date = $1 ORDER BY rank, id
	`, date)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []trades.Trade
	for rows.Next() {
		t, err := scanTradeRow(rows.Scan, false)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trade row: %w", err)
		}
		result = append(result, t)
	}

	return result, rows.Err()
}

func (s *Store) SaveEODSummaries(date string, summaryList []trades.TradeSummary) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("DELETE FROM summaries WHERE date = $1", date); err != nil {
		return fmt.Errorf("failed to clear existing summaries: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO summaries (
			date, symbol, contract_type, strike_price, expiration,
			entry_price, closing_price, stock_open, stock_close, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, s := range summaryList {
		_, err := stmt.Exec(
			date, s.Symbol, s.ContractType, s.StrikePrice, s.Expiration,
			s.EntryPrice, s.ClosingPrice, s.StockOpen, s.StockClose, s.Notes,
		)
		if err != nil {
			return fmt.Errorf("failed to insert summary %s: %w", s.Symbol, err)
		}
	}

	return tx.Commit()
}

func (s *Store) GetLatestTradeDate() (string, error) {
	var date string
	err := s.db.QueryRow("SELECT date FROM trades ORDER BY date DESC LIMIT 1").Scan(&date)
	if err != nil {
		return "", fmt.Errorf("no trades found: %w", err)
	}
	return date, nil
}

func (s *Store) GetTradeDates(limit int) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT date FROM trades ORDER BY date DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query trade dates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("failed to scan date: %w", err)
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}

func (s *Store) GetTradesForDateRange(startDate, endDate string) (map[string][]trades.Trade, error) {
	rows, err := s.db.Query(`
		SELECT date, id, symbol, contract_type, strike_price, expiration, dte,
			estimated_price, thesis, sentiment_score, current_price,
			target_price, stop_loss, risk_level,
			catalyst, mention_count, rank,
			score, rationale, model,
			target_otm_pct, min_dte
		FROM trades WHERE date >= $1 AND date <= $2 ORDER BY date, rank, id
	`, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]trades.Trade)
	for rows.Next() {
		var date string
		t, err := scanTradeRow(rows.Scan, true, &date)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trade row: %w", err)
		}
		result[date] = append(result[date], t)
	}
	return result, rows.Err()
}

/*
ResolveTradeContract is called by the at-open executor once it has
walked the live option chain and snapped a pick's intent (target_otm_pct,
min_dte) to a concrete contract. Writes the five formerly-NULL columns in
place so the dashboard, the morning email, and downstream selector +
order-builder paths can read them.

Single UPDATE with all five fields so partial failures can't leave a row
half-resolved (e.g. strike set but estimated_price still null).
*/
func (s *Store) ResolveTradeContract(tradeID int, strike float64, expiration string, dte int, estimatedPrice, stopLoss float64) error {
	_, err := s.db.Exec(`
		UPDATE trades
		SET strike_price = $2,
		    expiration = $3,
		    dte = $4,
		    estimated_price = $5,
		    stop_loss = $6
		WHERE id = $1
	`, tradeID, strike, expiration, dte, estimatedPrice, stopLoss)
	if err != nil {
		return fmt.Errorf("failed to resolve trade %d contract: %w", tradeID, err)
	}
	return nil
}

/*
scanTradeRow is the shared scan body for GetMorningTrades and
GetTradesForDateRange. The date prefix is optional: when withDate is true,
the first variadic arg must be a *string the scanner writes into. Pointer
fields (StrikePrice, Expiration, DTE, EstimatedPrice, StopLoss) come back
nil when the row is pre-resolution (picker saved intent only; executor
hasn't run yet).
*/
func scanTradeRow(scan func(...any) error, withDate bool, datePtr ...*string) (trades.Trade, error) {
	var t trades.Trade
	var strikeNull sql.NullFloat64
	var expirationNull sql.NullString
	var dteNull sql.NullInt64
	var estPriceNull sql.NullFloat64
	var stopLossNull sql.NullFloat64
	var targetOTMPctNull sql.NullFloat64
	var minDTENull sql.NullInt64

	dest := []any{
		&t.ID, &t.Symbol, &t.ContractType, &strikeNull, &expirationNull, &dteNull,
		&estPriceNull, &t.Thesis, &t.SentimentScore, &t.CurrentPrice,
		&t.TargetPrice, &stopLossNull, &t.RiskLevel,
		&t.Catalyst, &t.MentionCount, &t.Rank,
		&t.Score, &t.Rationale, &t.Model,
		&targetOTMPctNull, &minDTENull,
	}
	if withDate {
		if len(datePtr) != 1 {
			return t, fmt.Errorf("scanTradeRow: withDate=true requires exactly one datePtr")
		}
		dest = append([]any{datePtr[0]}, dest...)
	}

	if err := scan(dest...); err != nil {
		return t, err
	}

	if strikeNull.Valid {
		v := strikeNull.Float64
		t.StrikePrice = &v
	}
	if expirationNull.Valid {
		v := expirationNull.String
		t.Expiration = &v
	}
	if dteNull.Valid {
		v := int(dteNull.Int64)
		t.DTE = &v
	}
	if estPriceNull.Valid {
		v := estPriceNull.Float64
		t.EstimatedPrice = &v
	}
	if stopLossNull.Valid {
		v := stopLossNull.Float64
		t.StopLoss = &v
	}
	if targetOTMPctNull.Valid {
		t.TargetOTMPct = targetOTMPctNull.Float64
	}
	if minDTENull.Valid {
		t.MinDTE = int(minDTENull.Int64)
	}
	return t, nil
}

func nullableFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableString(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableIntPtr(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func (s *Store) GetSummariesForDateRange(startDate, endDate string) (map[string][]trades.TradeSummary, error) {
	rows, err := s.db.Query(`
		SELECT date, symbol, contract_type, strike_price, expiration,
			entry_price, closing_price, stock_open, stock_close, notes
		FROM summaries WHERE date >= $1 AND date <= $2 ORDER BY date, id
	`, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query summaries range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]trades.TradeSummary)
	for rows.Next() {
		var date string
		var ts trades.TradeSummary
		err := rows.Scan(
			&date, &ts.Symbol, &ts.ContractType, &ts.StrikePrice, &ts.Expiration,
			&ts.EntryPrice, &ts.ClosingPrice, &ts.StockOpen, &ts.StockClose, &ts.Notes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan summary row: %w", err)
		}
		result[date] = append(result[date], ts)
	}
	return result, rows.Err()
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

// --- EOD summary methods ---

func (s *Store) GetEODSummaries(date string) ([]trades.TradeSummary, error) {
	rows, err := s.db.Query(`
		SELECT symbol, contract_type, strike_price, expiration,
			entry_price, closing_price, stock_open, stock_close, notes
		FROM summaries WHERE date = $1 ORDER BY id
	`, date)
	if err != nil {
		return nil, fmt.Errorf("failed to query summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []trades.TradeSummary
	for rows.Next() {
		var s trades.TradeSummary
		err := rows.Scan(
			&s.Symbol, &s.ContractType, &s.StrikePrice, &s.Expiration,
			&s.EntryPrice, &s.ClosingPrice, &s.StockOpen, &s.StockClose, &s.Notes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan summary row: %w", err)
		}
		result = append(result, s)
	}

	return result, rows.Err()
}
