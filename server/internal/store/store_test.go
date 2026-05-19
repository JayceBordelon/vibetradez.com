package store

import (
	"testing"

	"vibetradez.com/internal/trades"
)

const testDatabaseURL = "postgresql://jaycebordelon@localhost:5432/vibetradez_test?sslmode=disable"

func setupTestDB(t *testing.T) *Store {
	t.Helper()
	s, err := New(testDatabaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	// Clean tables for test isolation
	_, _ = s.db.Exec("DELETE FROM subscribers")
	_, _ = s.db.Exec("DELETE FROM trades")
	_, _ = s.db.Exec("DELETE FROM summaries")
	return s
}

func TestSubscriberLifecycle(t *testing.T) {
	s := setupTestDB(t)

	// Add subscriber
	if err := s.AddSubscriber("test@example.com", "Test User"); err != nil {
		t.Fatalf("AddSubscriber failed: %v", err)
	}

	// Verify active
	subs, err := s.GetActiveSubscribers()
	if err != nil {
		t.Fatalf("GetActiveSubscribers failed: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(subs))
	}
	if subs[0].Email != "test@example.com" || subs[0].Name != "Test User" {
		t.Fatalf("unexpected subscriber data: %+v", subs[0])
	}

	// Get emails
	emails, err := s.GetActiveEmails()
	if err != nil {
		t.Fatalf("GetActiveEmails failed: %v", err)
	}
	if len(emails) != 1 || emails[0] != "test@example.com" {
		t.Fatalf("unexpected emails: %v", emails)
	}

	// Unsubscribe
	if err := s.RemoveSubscriber("test@example.com"); err != nil {
		t.Fatalf("RemoveSubscriber failed: %v", err)
	}

	// Verify empty
	emails, err = s.GetActiveEmails()
	if err != nil {
		t.Fatalf("GetActiveEmails after unsubscribe failed: %v", err)
	}
	if len(emails) != 0 {
		t.Fatalf("expected 0 emails after unsubscribe, got %d", len(emails))
	}

	// Re-subscribe (upsert)
	if err := s.AddSubscriber("test@example.com", "Test User Updated"); err != nil {
		t.Fatalf("Re-subscribe failed: %v", err)
	}
	subs, _ = s.GetActiveSubscribers()
	if len(subs) != 1 || subs[0].Name != "Test User Updated" {
		t.Fatalf("re-subscribe didn't update correctly: %+v", subs)
	}
}

func TestRemoveNonexistentSubscriber(t *testing.T) {
	s := setupTestDB(t)

	err := s.RemoveSubscriber("nobody@example.com")
	if err == nil {
		t.Fatal("expected error removing nonexistent subscriber")
	}
}

/*
TestEnsureSubscriberExists_PreservesUnsubscribe guards the audit fix:
side-effect paths (SSO callback, EMAIL_RECIPIENTS startup seed) must
NOT silently re-subscribe a user who has previously opted out. The
original AddSubscriber upsert flipped active=true on every call.
*/
func TestEnsureSubscriberExists_PreservesUnsubscribe(t *testing.T) {
	s := setupTestDB(t)

	// 1. User signs up explicitly.
	if err := s.AddSubscriber("alice@example.com", "Alice"); err != nil {
		t.Fatalf("initial AddSubscriber failed: %v", err)
	}

	// 2. User unsubscribes.
	if err := s.RemoveSubscriber("alice@example.com"); err != nil {
		t.Fatalf("RemoveSubscriber failed: %v", err)
	}
	emails, _ := s.GetActiveEmails()
	if len(emails) != 0 {
		t.Fatalf("post-unsub: expected 0 active, got %d", len(emails))
	}

	// 3. Side-effect path runs (SSO sign-in, or seed at container boot).
	// EnsureSubscriberExists must NOT re-subscribe.
	if err := s.EnsureSubscriberExists("alice@example.com", "Alice"); err != nil {
		t.Fatalf("EnsureSubscriberExists failed: %v", err)
	}

	emails, _ = s.GetActiveEmails()
	if len(emails) != 0 {
		t.Fatalf("EnsureSubscriberExists revived unsubscribe: %d active (should be 0)", len(emails))
	}

	// 4. User then explicitly re-signs-up. AddSubscriber WILL revive them.
	if err := s.AddSubscriber("alice@example.com", "Alice"); err != nil {
		t.Fatalf("explicit re-AddSubscriber failed: %v", err)
	}
	emails, _ = s.GetActiveEmails()
	if len(emails) != 1 {
		t.Fatalf("explicit re-signup did not revive: %d active (should be 1)", len(emails))
	}
}

/*
TestEnsureSubscriberExists_NewUserBecomesActive: for a user we have
never seen before, EnsureSubscriberExists inserts them as active=true
(default state for new rows is opt-in, since they're being added by
us explicitly through a side-effect path that has reasonable
expectation they want the mails — e.g. EMAIL_RECIPIENTS).
*/
func TestEnsureSubscriberExists_NewUserBecomesActive(t *testing.T) {
	s := setupTestDB(t)

	if err := s.EnsureSubscriberExists("bob@example.com", "Bob"); err != nil {
		t.Fatalf("EnsureSubscriberExists failed: %v", err)
	}

	emails, _ := s.GetActiveEmails()
	if len(emails) != 1 || emails[0] != "bob@example.com" {
		t.Fatalf("new user not active: %v", emails)
	}
}

/*
TestEnsureSubscriberExists_PreservesName: if a user already has a
non-empty name on file, a seed run with empty name must not clobber
it. Only fills in name when previously empty.
*/
func TestEnsureSubscriberExists_PreservesName(t *testing.T) {
	s := setupTestDB(t)

	if err := s.AddSubscriber("carol@example.com", "Carol Authored"); err != nil {
		t.Fatalf("AddSubscriber failed: %v", err)
	}

	// Seed path with empty name — must NOT overwrite.
	if err := s.EnsureSubscriberExists("carol@example.com", ""); err != nil {
		t.Fatalf("EnsureSubscriberExists failed: %v", err)
	}

	subs, _ := s.GetActiveSubscribers()
	if len(subs) != 1 || subs[0].Name != "Carol Authored" {
		t.Fatalf("name clobbered by empty seed: %+v", subs)
	}
}

func TestTradesPersistence(t *testing.T) {
	s := setupTestDB(t)

	testTrades := []trades.Trade{
		{
			Symbol:         "AAPL",
			ContractType:   "CALL",
			StrikePrice:    150.0,
			Expiration:     "2025-04-18",
			DTE:            5,
			EstimatedPrice: 3.50,
			Thesis:         "Bullish momentum",
			SentimentScore: 0.85,
			CurrentPrice:   148.0,
			TargetPrice:    155.0,
			StopLoss:       145.0,
			RiskLevel:      "MEDIUM",
			Catalyst:       "Earnings",
			MentionCount:   42,
		},
	}

	if err := s.SaveMorningTrades("2025-04-13", testTrades); err != nil {
		t.Fatalf("SaveMorningTrades failed: %v", err)
	}

	loaded, err := s.GetMorningTrades("2025-04-13")
	if err != nil {
		t.Fatalf("GetMorningTrades failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(loaded))
	}
	if loaded[0].Symbol != "AAPL" || loaded[0].StrikePrice != 150.0 {
		t.Fatalf("unexpected trade data: %+v", loaded[0])
	}
}

func TestSummariesPersistence(t *testing.T) {
	s := setupTestDB(t)

	summaries := []trades.TradeSummary{
		{
			Symbol:       "TSLA",
			ContractType: "PUT",
			StrikePrice:  200.0,
			Expiration:   "2025-04-18",
			EntryPrice:   5.0,
			ClosingPrice: 7.50,
			StockOpen:    205.0,
			StockClose:   195.0,
			Notes:        "Hit target",
		},
	}

	if err := s.SaveEODSummaries("2025-04-13", summaries); err != nil {
		t.Fatalf("SaveEODSummaries failed: %v", err)
	}

	loaded, err := s.GetEODSummaries("2025-04-13")
	if err != nil {
		t.Fatalf("GetEODSummaries failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(loaded))
	}
	if loaded[0].Symbol != "TSLA" || loaded[0].ClosingPrice != 7.50 {
		t.Fatalf("unexpected summary data: %+v", loaded[0])
	}
}
