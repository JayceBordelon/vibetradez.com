package store

import (
	"database/sql"
	"errors"
	"fmt"
)

/*
IsRolloutSent returns true if a rollout email with the given slug has
already been delivered. Retained for compatibility but the live
runner uses ClaimRollout instead — read-before-write isn't race-safe
when two processes boot concurrently (blue/green deploy, manual
`compose up` racing a leftover).
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
ClaimRollout atomically reserves a rollout slug for the current
process. Returns (true, nil) when this process owns the slug and
should proceed with the send; (false, nil) when another process
already claimed it. The previous shape — IsRolloutSent → SendTradeEmail
→ MarkRolloutSent — let two concurrent boots both observe sent=false
and both bulk-email every subscriber before either one wrote the
mark. INSERT ... ON CONFLICT DO NOTHING RETURNING is the
race-safe primitive: exactly one INSERT succeeds.

The trade-off is that a process which claims and then fails to send
leaves a "claimed but undelivered" row; the slug becomes permanently
inert until the operator manually deletes it. The audit favored
double-send-safety over send-failure-recovery, and the call site
logs prominently when a send fails after claim so the operator can
diagnose.
*/
func (s *Store) ClaimRollout(slug string, recipientCount int) (bool, error) {
	var claimed string
	err := s.db.QueryRow(`
		INSERT INTO sent_rollouts (slug, recipient_count)
		VALUES ($1, $2)
		ON CONFLICT (slug) DO NOTHING
		RETURNING slug
	`, slug, recipientCount).Scan(&claimed)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim rollout: %w", err)
	}
	return true, nil
}

/*
MarkRolloutSent is retained for compatibility with any caller still
using the legacy check-send-mark flow. Prefer ClaimRollout for new
code; this method does NOT race-safely guard against double-send.
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
