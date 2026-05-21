/*
Package auth is the trading-server's in-process Google OAuth flow.
The same binary that serves /api/* also serves /auth/google/start and
/auth/google/callback, and validates session cookies against the
sessions table directly (no HTTP hop, no /verify cache).

Connects to a dedicated AUTH_DATABASE_URL so users + sessions live in
their own pool (separable from the trading DB without a schema fight).
*/
package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID            int64     `json:"id"`
	GoogleSub     string    `json:"-"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	Name          string    `json:"name"`
	PictureURL    string    `json:"picture_url"`
	CreatedAt     time.Time `json:"-"`
	LastLoginAt   time.Time `json:"-"`
}

type Session struct {
	ID         int64
	UserID     int64
	CreatedAt  time.Time
	LastUsedAt time.Time
	ExpiresAt  time.Time
	User       User
}

func NewStore(databaseURL string) (*Store, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open auth db: %w", err)
	}
	// Every /api/* request hits attachUser → LookupSession, so the
	// pool needs headroom on traffic spikes without exhausting the
	// managed DB's per-database connection limit.
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect auth db: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate auth db: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Ping() error  { return s.db.Ping() }

/*
migrate is idempotent: CREATE TABLE IF NOT EXISTS on the three tables
this package needs (users, oauth_states, sessions). Safe to run on a
fresh Postgres (local dev, test) or against a DB that already has the
tables (production).
*/
func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			google_sub TEXT UNIQUE NOT NULL,
			email TEXT NOT NULL,
			email_verified BOOLEAN NOT NULL DEFAULT false,
			name TEXT NOT NULL DEFAULT '',
			picture_url TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_login_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

		-- oauth_states holds CSRF-protected pending Google flows
		-- (random string + return_to + TTL).
		CREATE TABLE IF NOT EXISTS oauth_states (
			state TEXT PRIMARY KEY,
			return_to TEXT NOT NULL DEFAULT '/',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL
		);

		CREATE TABLE IF NOT EXISTS sessions (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash BYTEA UNIQUE NOT NULL,
			user_agent TEXT NOT NULL DEFAULT '',
			ip_addr INET,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL,
			revoked_at TIMESTAMPTZ
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
	`)
	return err
}

// --- Users ---

func (s *Store) UpsertUser(googleSub, email string, emailVerified bool, name, pictureURL string) (int64, error) {
	var id int64
	err := s.db.QueryRow(`
		INSERT INTO users (google_sub, email, email_verified, name, picture_url, last_login_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (google_sub) DO UPDATE SET
			email = EXCLUDED.email,
			email_verified = EXCLUDED.email_verified,
			name = EXCLUDED.name,
			picture_url = EXCLUDED.picture_url,
			last_login_at = NOW()
		RETURNING id
	`, googleSub, email, emailVerified, name, pictureURL).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert user: %w", err)
	}
	return id, nil
}

// --- oauth_states (Google CSRF + return-to stash) ---

func (s *Store) CreateOAuthState(state, returnTo string, ttl time.Duration) error {
	_, err := s.db.Exec(`
		INSERT INTO oauth_states (state, return_to, expires_at)
		VALUES ($1, $2, NOW() + ($3 || ' seconds')::interval)
	`, state, returnTo, fmt.Sprintf("%d", int64(ttl.Seconds())))
	if err != nil {
		return fmt.Errorf("create oauth state: %w", err)
	}
	return nil
}

// ConsumeOAuthState deletes and returns the return_to. One-shot.
func (s *Store) ConsumeOAuthState(state string) (returnTo string, err error) {
	err = s.db.QueryRow(`
		DELETE FROM oauth_states
		WHERE state = $1 AND expires_at > NOW()
		RETURNING return_to
	`, state).Scan(&returnTo)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("oauth state not found or expired")
	}
	if err != nil {
		return "", fmt.Errorf("consume oauth state: %w", err)
	}
	return returnTo, nil
}

// --- sessions ---

func (s *Store) CreateSession(userID int64, tokenHash []byte, userAgent, ipAddr string, ttl time.Duration) error {
	var ipArg any
	if ipAddr != "" {
		ipArg = ipAddr
	}
	_, err := s.db.Exec(`
		INSERT INTO sessions (user_id, token_hash, user_agent, ip_addr, expires_at)
		VALUES ($1, $2, $3, $4, NOW() + ($5 || ' seconds')::interval)
	`, userID, tokenHash, userAgent, ipArg, fmt.Sprintf("%d", int64(ttl.Seconds())))
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *Store) LookupSession(tokenHash []byte) (*Session, error) {
	var sess Session
	err := s.db.QueryRow(`
		SELECT s.id, s.user_id, s.created_at, s.last_used_at, s.expires_at,
			u.id, u.google_sub, u.email, u.email_verified, u.name, u.picture_url, u.created_at, u.last_login_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1
			AND s.revoked_at IS NULL
			AND s.expires_at > NOW()
	`, tokenHash).Scan(
		&sess.ID, &sess.UserID, &sess.CreatedAt, &sess.LastUsedAt, &sess.ExpiresAt,
		&sess.User.ID, &sess.User.GoogleSub, &sess.User.Email, &sess.User.EmailVerified,
		&sess.User.Name, &sess.User.PictureURL, &sess.User.CreatedAt, &sess.User.LastLoginAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup session: %w", err)
	}
	return &sess, nil
}

func (s *Store) TouchSession(sessionID int64) error {
	_, err := s.db.Exec(`
		UPDATE sessions SET last_used_at = NOW()
		WHERE id = $1 AND last_used_at < NOW() - INTERVAL '1 hour'
	`, sessionID)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

func (s *Store) RevokeSession(sessionID int64) error {
	_, err := s.db.Exec(`UPDATE sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, sessionID)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (s *Store) SweepExpired() (sessions, states int64, err error) {
	res, err := s.db.Exec(`
		DELETE FROM sessions
		WHERE expires_at < NOW() - INTERVAL '7 days'
			OR (revoked_at IS NOT NULL AND revoked_at < NOW() - INTERVAL '7 days')
	`)
	if err != nil {
		return 0, 0, fmt.Errorf("sweep sessions: %w", err)
	}
	sessions, _ = res.RowsAffected()

	res, err = s.db.Exec(`DELETE FROM oauth_states WHERE expires_at < NOW()`)
	if err != nil {
		return sessions, 0, fmt.Errorf("sweep oauth_states: %w", err)
	}
	states, _ = res.RowsAffected()
	return sessions, states, nil
}
