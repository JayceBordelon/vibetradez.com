package store

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"vibetradez.com/internal/trades"
)

// jsonEqual reports whether two JSON byte slices are semantically equal,
// ignoring key order and whitespace (JSONB normalizes both on storage).
func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("jsonEqual: bad JSON %s: %v", a, err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("jsonEqual: bad JSON %s: %v", b, err)
	}
	return reflect.DeepEqual(av, bv)
}

/*
testDatabaseURL is the connection string used by every test in this
package. CI exports TEST_DATABASE_URL pointing at the workflow's
ephemeral Postgres service (different user / port than my local
machine); when the env var is unset (developer running tests
locally), fall back to my dev DB.
*/
var testDatabaseURL = func() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgresql://jaycebordelon@localhost:5432/vibetradez_test?sslmode=disable"
}()

func setupTestDB(t *testing.T) *Store {
	t.Helper()
	s, err := New(testDatabaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	// Clean tables for test isolation. Executions must go before trades
	// because of the FK; subscribers and summaries are independent.
	_, _ = s.db.Exec("DELETE FROM executions")
	_, _ = s.db.Exec("DELETE FROM subscribers")
	_, _ = s.db.Exec("DELETE FROM trades")
	_, _ = s.db.Exec("DELETE FROM summaries")
	_, _ = s.db.Exec("DELETE FROM sent_rollouts")
	return s
}

func TestTranscriptRoundTripAndUpsert(t *testing.T) {
	s := setupTestDB(t)
	_, _ = s.db.Exec("DELETE FROM transcripts")

	// Missing row → (nil, nil), not an error.
	tv, err := s.GetTranscript("2026-06-03", "selection")
	if err != nil {
		t.Fatalf("GetTranscript(missing) error: %v", err)
	}
	if tv != nil {
		t.Fatalf("GetTranscript(missing) = %+v, want nil", tv)
	}

	events := []byte(`[{"round":0,"type":"text","text":"hello"}]`)
	if err := s.SaveTranscript("2026-06-03", "selection", "claude-opus-4-8", events); err != nil {
		t.Fatalf("SaveTranscript failed: %v", err)
	}

	tv, err = s.GetTranscript("2026-06-03", "selection")
	if err != nil {
		t.Fatalf("GetTranscript failed: %v", err)
	}
	if tv == nil {
		t.Fatal("GetTranscript returned nil after save")
	}
	if tv.Model != "claude-opus-4-8" || tv.Kind != "selection" || tv.Date != "2026-06-03" {
		t.Fatalf("unexpected transcript metadata: %+v", tv)
	}
	// JSONB normalizes key order + whitespace on storage, so compare the
	// parsed content, not the raw bytes.
	if !jsonEqual(t, tv.Events, events) {
		t.Fatalf("events round-trip mismatch: got %s want %s", tv.Events, events)
	}

	// Upsert on (date, kind) overwrites.
	newEvents := []byte(`[{"round":0,"type":"text","text":"updated"}]`)
	if err := s.SaveTranscript("2026-06-03", "selection", "claude-sonnet-4-6", newEvents); err != nil {
		t.Fatalf("SaveTranscript (upsert) failed: %v", err)
	}
	tv, _ = s.GetTranscript("2026-06-03", "selection")
	if tv.Model != "claude-sonnet-4-6" || !jsonEqual(t, tv.Events, newEvents) {
		t.Fatalf("upsert did not overwrite: %+v", tv)
	}

	// Different kind is an independent row.
	if err := s.SaveTranscript("2026-06-03", "execution", "claude-opus-4-8", events); err != nil {
		t.Fatalf("SaveTranscript(execution) failed: %v", err)
	}
	exec, _ := s.GetTranscript("2026-06-03", "execution")
	if exec == nil || exec.Kind != "execution" {
		t.Fatalf("execution transcript not stored independently: %+v", exec)
	}
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

/*
TestSaveMorningTrades_GuardsAgainstReRunWithExecutions covers the
audit fix: the morning cron firing twice (RUN_ON_START, container
restart at 9:29:55 ET) must not wipe trade ids that executions
reference. The second call should return ErrTradesAlreadyExecuted
and leave both tables untouched.
*/
func TestSaveMorningTrades_GuardsAgainstReRunWithExecutions(t *testing.T) {
	s := setupTestDB(t)

	first := []trades.Trade{
		mkResolvedTrade("AAPL", "CALL", 150.0, "2025-04-18", 5, 3.5, "LOW"),
		mkResolvedTrade("TSLA", "PUT", 200.0, "2025-04-18", 5, 4.0, "MEDIUM"),
	}
	if err := s.SaveMorningTrades("2025-04-13", first); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if first[0].ID == 0 {
		t.Fatal("first save did not populate ID")
	}

	// Simulate an execution row referencing the first save's trade id.
	if _, err := s.db.Exec(`
		INSERT INTO executions (trade_id, mode, side, status, requested_quantity)
		VALUES ($1, 'paper', 'open', 'filled', 1)
	`, first[0].ID); err != nil {
		t.Fatalf("seed execution: %v", err)
	}

	second := []trades.Trade{
		mkResolvedTrade("NVDA", "CALL", 700.0, "2025-04-18", 5, 6.0, "HIGH"),
	}
	err := s.SaveMorningTrades("2025-04-13", second)
	if !errors.Is(err, ErrTradesAlreadyExecuted) {
		t.Fatalf("want ErrTradesAlreadyExecuted, got: %v", err)
	}

	// Confirm the original trades survived the rejected rewrite.
	loaded, err := s.GetMorningTrades("2025-04-13")
	if err != nil {
		t.Fatalf("GetMorningTrades: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("first-save trades clobbered: got %d, want 2", len(loaded))
	}
	if loaded[0].Symbol == "NVDA" {
		t.Fatalf("first-save trades replaced with second-save trades")
	}
}

/*
TestSaveMorningTrades_AllowsRewriteWhenNoExecutions verifies the
guard isn't overzealous: re-runs on the same date are fine when no
execution rows reference the existing trades (e.g. paper-only stub
mode or pre-executor crash). The second save replaces the first.
*/
func TestSaveMorningTrades_AllowsRewriteWhenNoExecutions(t *testing.T) {
	s := setupTestDB(t)

	first := []trades.Trade{
		mkResolvedTrade("AAPL", "CALL", 150.0, "2025-04-18", 5, 3.5, "LOW"),
	}
	if err := s.SaveMorningTrades("2025-04-13", first); err != nil {
		t.Fatalf("first save: %v", err)
	}

	second := []trades.Trade{
		mkResolvedTrade("NVDA", "CALL", 700.0, "2025-04-18", 5, 6.0, "HIGH"),
	}
	if err := s.SaveMorningTrades("2025-04-13", second); err != nil {
		t.Fatalf("second save with no executions should succeed, got: %v", err)
	}

	loaded, err := s.GetMorningTrades("2025-04-13")
	if err != nil {
		t.Fatalf("GetMorningTrades: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Symbol != "NVDA" {
		t.Fatalf("second save did not replace first: %+v", loaded)
	}
}

func TestTradesPersistence(t *testing.T) {
	s := setupTestDB(t)

	tr := trades.Trade{
		Symbol:       "AAPL",
		ContractType: "CALL",
		Thesis:       "Bullish momentum",
		CurrentPrice: 148.0,
		TargetPrice:  155.0,
		RiskLevel:    "MEDIUM",
		Catalyst:     "Earnings",
		MentionCount: 42,
	}
	tr.SetResolvedContract(150.0, "2025-04-18", 5, 3.50, 145.0)
	testTrades := []trades.Trade{tr}

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
	if loaded[0].Symbol != "AAPL" || loaded[0].Strike() != 150.0 {
		t.Fatalf("unexpected trade data: %+v", loaded[0])
	}
}

// mkResolvedTrade is the test-only constructor for a fully-resolved
// pre-refactor-shape trade. Pre-refactor the Trade literal had inline
// strike/expiration/dte/estimated_price; post-refactor those are
// pointers because the picker no longer produces them. Tests that care
// about the resolved-contract pathway use this helper; tests that
// exercise pre-resolution paths build literals directly with nil
// pointer fields.
func mkResolvedTrade(symbol, kind string, strike float64, expiration string, dte int, est float64, risk string) trades.Trade {
	t := trades.Trade{Symbol: symbol, ContractType: kind, RiskLevel: risk}
	t.SetResolvedContract(strike, expiration, dte, est, est*0.5)
	return t
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
