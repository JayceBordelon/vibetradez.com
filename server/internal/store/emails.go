package store

import "fmt"

/*
One-time email ledger. A one-time blast (the v2 launch announcement, and any
future one-off) claims a stable key in sent_emails so it fires exactly once
and never re-sends on a later boot, even if its trigger env flag stays set.
*/

// EmailAlreadySent reports whether a one-time email with this key has already
// been recorded as sent.
func (s *Store) EmailAlreadySent(key string) (bool, error) {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sent_emails WHERE key = $1)`, key).Scan(&exists); err != nil {
		return false, fmt.Errorf("check sent email %q: %w", key, err)
	}
	return exists, nil
}

// MarkEmailSent records that a one-time email with this key has been sent.
// Idempotent: a repeat call for the same key is a no-op.
func (s *Store) MarkEmailSent(key string) error {
	if _, err := s.db.Exec(`INSERT INTO sent_emails (key) VALUES ($1) ON CONFLICT (key) DO NOTHING`, key); err != nil {
		return fmt.Errorf("mark email sent %q: %w", key, err)
	}
	return nil
}
