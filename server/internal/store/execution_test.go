package store

import (
	"database/sql"
	"testing"
)

/*
TestDeriveExecutionState_CloseFailedStrandedAtBroker guards the
audit fix: a position whose open filled but whose close attempt
ended in failed / rejected / canceled is no longer collapsed into
"holding" — the dashboard now surfaces "close_failed" so the
operator knows the position is stranded at the broker after the
3:55 retry-cancel-replace window exhausted.
*/
func TestDeriveExecutionState_CloseFailedStrandedAtBroker(t *testing.T) {
	cases := []struct {
		openStatus  string
		closeStatus sql.NullString
		want        string
	}{
		// Open + no close yet → holding.
		{"filled", sql.NullString{Valid: false}, "holding"},
		// Open + close filled → closed.
		{"filled", sql.NullString{Valid: true, String: "filled"}, "closed"},
		// Open + close in-flight → still holding.
		{"filled", sql.NullString{Valid: true, String: "pending"}, "holding"},
		{"filled", sql.NullString{Valid: true, String: "working"}, "holding"},
		// THE FIX: close attempt reached a non-success terminal.
		{"filled", sql.NullString{Valid: true, String: "failed"}, "close_failed"},
		{"filled", sql.NullString{Valid: true, String: "rejected"}, "close_failed"},
		{"filled", sql.NullString{Valid: true, String: "canceled"}, "close_failed"},
		// Open never filled.
		{"working", sql.NullString{}, "submitted"},
		{"pending", sql.NullString{}, "submitted"},
		{"failed", sql.NullString{}, "failed"},
		{"rejected", sql.NullString{}, "failed"},
		// Unknown open status → empty (don't surface).
		{"", sql.NullString{}, ""},
		{"garbage", sql.NullString{}, ""},
	}

	for _, tc := range cases {
		got := deriveExecutionState(tc.openStatus, tc.closeStatus)
		if got != tc.want {
			t.Errorf("deriveExecutionState(%q, %+v) = %q, want %q", tc.openStatus, tc.closeStatus, got, tc.want)
		}
	}
}
