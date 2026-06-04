package store

import "testing"

func TestSentEmailsLedger(t *testing.T) {
	s := setupTestDB(t)
	_, _ = s.db.Exec("DELETE FROM sent_emails")

	const key = "test_one_time_blast"

	if sent, err := s.EmailAlreadySent(key); err != nil {
		t.Fatalf("check before mark: %v", err)
	} else if sent {
		t.Fatal("key should not be marked sent yet")
	}

	if err := s.MarkEmailSent(key); err != nil {
		t.Fatalf("mark sent: %v", err)
	}

	if sent, err := s.EmailAlreadySent(key); err != nil {
		t.Fatalf("check after mark: %v", err)
	} else if !sent {
		t.Fatal("key should be marked sent after MarkEmailSent")
	}

	// Idempotent: a repeat claim must not error and must not duplicate.
	if err := s.MarkEmailSent(key); err != nil {
		t.Fatalf("second mark should be a no-op, got: %v", err)
	}
	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM sent_emails WHERE key = $1", key).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 ledger row for key, got %d", n)
	}
}
