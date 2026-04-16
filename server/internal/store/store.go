package store

import (
	"database/sql"
	"fmt"
	"time"

	"vibetradez.com/internal/trades"

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
	_, err := db.Exec(`
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
			profit_target DOUBLE PRECISION NOT NULL DEFAULT 0,
			risk_level TEXT NOT NULL DEFAULT '',
			catalyst TEXT NOT NULL DEFAULT '',
			mention_count INTEGER NOT NULL DEFAULT 0,
			rank INTEGER NOT NULL DEFAULT 0,
			gpt_score INTEGER NOT NULL DEFAULT 0,
			gpt_rationale TEXT NOT NULL DEFAULT '',
			claude_score INTEGER NOT NULL DEFAULT 0,
			claude_rationale TEXT NOT NULL DEFAULT '',
			combined_score DOUBLE PRECISION NOT NULL DEFAULT 0,
			picked_by_openai BOOLEAN NOT NULL DEFAULT false,
			picked_by_claude BOOLEAN NOT NULL DEFAULT false,
			gpt_verdict TEXT NOT NULL DEFAULT '',
			claude_verdict TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		ALTER TABLE trades ADD COLUMN IF NOT EXISTS gpt_score INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE trades ADD COLUMN IF NOT EXISTS gpt_rationale TEXT NOT NULL DEFAULT '';
		ALTER TABLE trades ADD COLUMN IF NOT EXISTS claude_score INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE trades ADD COLUMN IF NOT EXISTS claude_rationale TEXT NOT NULL DEFAULT '';
		ALTER TABLE trades ADD COLUMN IF NOT EXISTS combined_score DOUBLE PRECISION NOT NULL DEFAULT 0;
		ALTER TABLE trades ADD COLUMN IF NOT EXISTS picked_by_openai BOOLEAN NOT NULL DEFAULT false;
		ALTER TABLE trades ADD COLUMN IF NOT EXISTS picked_by_claude BOOLEAN NOT NULL DEFAULT false;
		ALTER TABLE trades ADD COLUMN IF NOT EXISTS gpt_verdict TEXT NOT NULL DEFAULT '';
		ALTER TABLE trades ADD COLUMN IF NOT EXISTS claude_verdict TEXT NOT NULL DEFAULT '';
		-- Backfill existing rows: any pre-refactor trade had a non-zero
		-- gpt_score (GPT generated the picks) so it counts as picked by
		-- OpenAI. Pre-refactor Claude was a validator, not a picker, so
		-- claude_score > 0 alone does NOT imply Claude originally picked
		-- the trade — only forward-going rows from the new pipeline get
		-- picked_by_claude = true.
		UPDATE trades SET picked_by_openai = true WHERE picked_by_openai = false AND gpt_score > 0;

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
	`)
	return err
}

// DB returns the underlying *sql.DB for ad-hoc queries.
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping() error {
	return s.db.Ping()
}

// RemoveAllForTest clears all data — only for use in tests.
func (s *Store) RemoveAllForTest() {
	_, _ = s.db.Exec("DELETE FROM subscribers")
	_, _ = s.db.Exec("DELETE FROM trades")
	_, _ = s.db.Exec("DELETE FROM summaries")
}

// --- Subscriber methods ---

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

func (s *Store) SaveMorningTrades(date string, tradeList []trades.Trade) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("DELETE FROM trades WHERE date = $1", date); err != nil {
		return fmt.Errorf("failed to clear existing trades: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO trades (
			date, symbol, contract_type, strike_price, expiration, dte,
			estimated_price, thesis, sentiment_score, current_price,
			target_price, stop_loss, profit_target, risk_level,
			catalyst, mention_count, rank,
			gpt_score, gpt_rationale, claude_score, claude_rationale, combined_score,
			picked_by_openai, picked_by_claude, gpt_verdict, claude_verdict
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, t := range tradeList {
		_, err := stmt.Exec(
			date, t.Symbol, t.ContractType, t.StrikePrice, t.Expiration, t.DTE,
			t.EstimatedPrice, t.Thesis, t.SentimentScore, t.CurrentPrice,
			t.TargetPrice, t.StopLoss, t.ProfitTarget, t.RiskLevel,
			t.Catalyst, t.MentionCount, t.Rank,
			t.GPTScore, t.GPTRationale, t.ClaudeScore, t.ClaudeRationale, t.CombinedScore,
			t.PickedByOpenAI, t.PickedByClaude, t.GPTVerdict, t.ClaudeVerdict,
		)
		if err != nil {
			return fmt.Errorf("failed to insert trade %s: %w", t.Symbol, err)
		}
	}

	return tx.Commit()
}

func (s *Store) GetMorningTrades(date string) ([]trades.Trade, error) {
	rows, err := s.db.Query(`
		SELECT symbol, contract_type, strike_price, expiration, dte,
			estimated_price, thesis, sentiment_score, current_price,
			target_price, stop_loss, profit_target, risk_level,
			catalyst, mention_count, rank,
			gpt_score, gpt_rationale, claude_score, claude_rationale, combined_score,
			picked_by_openai, picked_by_claude, gpt_verdict, claude_verdict
		FROM trades WHERE date = $1 ORDER BY rank, id
	`, date)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []trades.Trade
	for rows.Next() {
		var t trades.Trade
		err := rows.Scan(
			&t.Symbol, &t.ContractType, &t.StrikePrice, &t.Expiration, &t.DTE,
			&t.EstimatedPrice, &t.Thesis, &t.SentimentScore, &t.CurrentPrice,
			&t.TargetPrice, &t.StopLoss, &t.ProfitTarget, &t.RiskLevel,
			&t.Catalyst, &t.MentionCount, &t.Rank,
			&t.GPTScore, &t.GPTRationale, &t.ClaudeScore, &t.ClaudeRationale, &t.CombinedScore,
			&t.PickedByOpenAI, &t.PickedByClaude, &t.GPTVerdict, &t.ClaudeVerdict,
		)
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
		SELECT date, symbol, contract_type, strike_price, expiration, dte,
			estimated_price, thesis, sentiment_score, current_price,
			target_price, stop_loss, profit_target, risk_level,
			catalyst, mention_count, rank,
			gpt_score, gpt_rationale, claude_score, claude_rationale, combined_score,
			picked_by_openai, picked_by_claude, gpt_verdict, claude_verdict
		FROM trades WHERE date >= $1 AND date <= $2 ORDER BY date, rank, id
	`, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]trades.Trade)
	for rows.Next() {
		var date string
		var t trades.Trade
		err := rows.Scan(
			&date, &t.Symbol, &t.ContractType, &t.StrikePrice, &t.Expiration, &t.DTE,
			&t.EstimatedPrice, &t.Thesis, &t.SentimentScore, &t.CurrentPrice,
			&t.TargetPrice, &t.StopLoss, &t.ProfitTarget, &t.RiskLevel,
			&t.Catalyst, &t.MentionCount, &t.Rank,
			&t.GPTScore, &t.GPTRationale, &t.ClaudeScore, &t.ClaudeRationale, &t.CombinedScore,
			&t.PickedByOpenAI, &t.PickedByClaude, &t.GPTVerdict, &t.ClaudeVerdict,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trade row: %w", err)
		}
		result[date] = append(result[date], t)
	}
	return result, rows.Err()
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
