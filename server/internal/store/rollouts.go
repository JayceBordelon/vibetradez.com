package store

import (
	"database/sql"
	"errors"
	"fmt"
)

/*
IsRolloutSent returns true if a rollout email with the given slug has
already been delivered. Called on startup before sending — guarantees
each rollout is sent exactly once across the lifetime of the deployment.
*/
func (s *Store) IsRolloutSent(slug string) (bool, error) {
	var dummy int
	err := s.db.QueryRow(`SELECT 1 FROM sent_rollouts WHERE slug = $1`, slug).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check rollout sent: %w", err)
	}
	return true, nil
}

/*
MarkRolloutSent records the slug as delivered with the recipient count.
Called immediately after the bulk-send returns success. Idempotent via
PRIMARY KEY conflict — re-marking is a no-op rather than an error so a
race between two starting processes can't double-send.
*/
func (s *Store) MarkRolloutSent(slug string, recipientCount int) error {
	_, err := s.db.Exec(`
		INSERT INTO sent_rollouts (slug, recipient_count)
		VALUES ($1, $2)
		ON CONFLICT (slug) DO NOTHING
	`, slug, recipientCount)
	if err != nil {
		return fmt.Errorf("mark rollout sent: %w", err)
	}
	return nil
}
