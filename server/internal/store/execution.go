package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"vibetradez.com/internal/exec"
)

// ErrNoDecision is returned by GetDecision when no row matches.
var ErrNoDecision = errors.New("no decision found")

/*
InsertDecision creates the daily go/no-go row WITHOUT a token_hash —
the caller mints the confirmation token after this returns (so the
token's payload can carry the real row id) and writes the hash back
via SetDecisionTokenHash. UNIQUE(trade_date) prevents two decisions
on the same day, which is the actual single-use guarantee.
*/
func (s *Store) InsertDecision(d exec.Decision) (int, error) {
	var id int
	err := s.db.QueryRow(`
		INSERT INTO execution_decisions (
			trade_date, symbol, contract_type, strike_price, expiration,
			occ_symbol, contract_price, gpt_score, claude_score, trade_id,
			decision, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'pending', $11)
		RETURNING id
	`, d.TradeDate, d.Symbol, d.ContractType, d.StrikePrice, d.Expiration,
		d.OCCSymbol, d.ContractPrice, d.GPTScore, d.ClaudeScore, nullableInt(d.TradeID),
		d.ExpiresAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert decision: %w", err)
	}
	return id, nil
}

/*
SetDecisionTokenHash records the SHA-256 of the execute-side
confirmation token for an existing decision row. Audit trail only —
single-use enforcement is provided atomically by SetDecisionStatus's
WHERE decision='pending' guard, not by hash comparison at confirm
time. Called immediately after InsertDecision returns the row id so
the token's payload can reference the real id.
*/
func (s *Store) SetDecisionTokenHash(id int, hash string) error {
	_, err := s.db.Exec(`UPDATE execution_decisions SET token_hash = $1 WHERE id = $2`, hash, id)
	if err != nil {
		return fmt.Errorf("set token hash: %w", err)
	}
	return nil
}

// GetDecision loads a decision row by id.
func (s *Store) GetDecision(id int) (*exec.Decision, error) {
	var d exec.Decision
	var tradeID sql.NullInt64
	var decidedAt sql.NullTime
	var tokenHash sql.NullString
	err := s.db.QueryRow(`
		SELECT id, trade_date, symbol, contract_type, strike_price, expiration,
			occ_symbol, contract_price, gpt_score, claude_score, trade_id,
			token_hash, decision, decided_at, expires_at, created_at
		FROM execution_decisions WHERE id = $1
	`, id).Scan(&d.ID, &d.TradeDate, &d.Symbol, &d.ContractType, &d.StrikePrice, &d.Expiration,
		&d.OCCSymbol, &d.ContractPrice, &d.GPTScore, &d.ClaudeScore, &tradeID,
		&tokenHash, &d.Decision, &decidedAt, &d.ExpiresAt, &d.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoDecision
	}
	if err != nil {
		return nil, fmt.Errorf("get decision: %w", err)
	}
	if tradeID.Valid {
		d.TradeID = int(tradeID.Int64)
	}
	if tokenHash.Valid {
		d.TokenHash = tokenHash.String
	}
	if decidedAt.Valid {
		d.DecidedAt = &decidedAt.Time
	}
	return &d, nil
}

/*
GetDecisionByDate returns the (at most one) decision for a trade
date, regardless of status. Returns ErrNoDecision if none exists.
Used by the cancel-all kill switch to find what's currently in flight.
*/
func (s *Store) GetDecisionByDate(date string) (*exec.Decision, error) {
	var id int
	err := s.db.QueryRow(`SELECT id FROM execution_decisions WHERE trade_date = $1`, date).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoDecision
	}
	if err != nil {
		return nil, fmt.Errorf("get decision by date: %w", err)
	}
	return s.GetDecision(id)
}

/*
ForceSetDecisionStatus updates a decision's status WITHOUT the
pending-only guard that SetDecisionStatus enforces. Reserved for the
cancel-all kill switch which needs to terminate decisions already in
'execute' state. Caller is responsible for ensuring this is only
invoked from authorized paths.
*/
func (s *Store) ForceSetDecisionStatus(id int, newStatus string) error {
	_, err := s.db.Exec(`
		UPDATE execution_decisions
		SET decision = $1, decided_at = COALESCE(decided_at, NOW())
		WHERE id = $2
	`, newStatus, id)
	if err != nil {
		return fmt.Errorf("force update decision: %w", err)
	}
	return nil
}

/*
SetDecisionStatus transitions a decision's status atomically. The
transition fails (returns ErrDecisionNotPending) if the current value
isn't 'pending' — this is the single-use enforcement: a token can only
move a decision out of pending state once.
*/
func (s *Store) SetDecisionStatus(id int, newStatus string) error {
	res, err := s.db.Exec(`
		UPDATE execution_decisions
		SET decision = $1, decided_at = NOW()
		WHERE id = $2 AND decision = 'pending'
	`, newStatus, id)
	if err != nil {
		return fmt.Errorf("update decision: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrDecisionNotPending
	}
	return nil
}

/*
ErrDecisionNotPending is returned when a state transition is attempted
on a decision that has already been decided.
*/
var ErrDecisionNotPending = errors.New("decision is no longer pending")

/*
PendingDecisions returns all decisions still awaiting user action.
Used by the auto-cancel cron to find decisions whose 5-minute window
has elapsed.
*/
func (s *Store) PendingDecisions() ([]exec.Decision, error) {
	rows, err := s.db.Query(`
		SELECT id, trade_date, symbol, contract_type, strike_price, expiration,
			occ_symbol, contract_price, gpt_score, claude_score,
			COALESCE(trade_id, 0), COALESCE(token_hash, ''), decision, expires_at, created_at
		FROM execution_decisions
		WHERE decision = 'pending'
	`)
	if err != nil {
		return nil, fmt.Errorf("query pending decisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []exec.Decision
	for rows.Next() {
		var d exec.Decision
		if err := rows.Scan(&d.ID, &d.TradeDate, &d.Symbol, &d.ContractType, &d.StrikePrice, &d.Expiration,
			&d.OCCSymbol, &d.ContractPrice, &d.GPTScore, &d.ClaudeScore,
			&d.TradeID, &d.TokenHash, &d.Decision, &d.ExpiresAt, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending decision: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

/*
InsertExecution records an order submission (paper or live). Returns
the new row id. The caller is responsible for setting Status correctly
based on the trader's response.
*/
func (s *Store) InsertExecution(e exec.Execution) (int, error) {
	var id int
	err := s.db.QueryRow(`
		INSERT INTO executions (
			decision_id, mode, side, schwab_order_id, status,
			fill_price, filled_quantity, requested_quantity,
			filled_at, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, e.DecisionID, e.Mode, e.Side, e.SchwabOrderID, e.Status,
		e.FillPrice, e.FilledQuantity, e.RequestedQuantity,
		e.FilledAt, e.ErrorMessage).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert execution: %w", err)
	}
	return id, nil
}

/*
UpdateExecutionStatus updates fill state on an existing execution row.
Used as orders progress from working → filled (or canceled / failed).
*/
func (s *Store) UpdateExecutionStatus(id int, status string, fillPrice *float64, filledQty int, errMsg string) error {
	_, err := s.db.Exec(`
		UPDATE executions
		SET status = $1, fill_price = COALESCE($2, fill_price), filled_quantity = $3,
		    error_message = CASE WHEN $4 = '' THEN error_message ELSE $4 END,
		    filled_at = CASE WHEN $1 = 'filled' AND filled_at IS NULL THEN NOW() ELSE filled_at END
		WHERE id = $5
	`, status, fillPrice, filledQty, errMsg, id)
	if err != nil {
		return fmt.Errorf("update execution: %w", err)
	}
	return nil
}

// GetExecution loads one execution row by id.
func (s *Store) GetExecution(id int) (*exec.Execution, error) {
	var e exec.Execution
	var schwabOrderID sql.NullString
	var fillPrice sql.NullFloat64
	var filledAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, decision_id, mode, side, schwab_order_id, status,
			fill_price, filled_quantity, requested_quantity,
			submitted_at, filled_at, error_message, created_at
		FROM executions WHERE id = $1
	`, id).Scan(&e.ID, &e.DecisionID, &e.Mode, &e.Side, &schwabOrderID, &e.Status,
		&fillPrice, &e.FilledQuantity, &e.RequestedQuantity,
		&e.SubmittedAt, &filledAt, &e.ErrorMessage, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("no execution with id %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get execution: %w", err)
	}
	if schwabOrderID.Valid {
		v := schwabOrderID.String
		e.SchwabOrderID = &v
	}
	if fillPrice.Valid {
		v := fillPrice.Float64
		e.FillPrice = &v
	}
	if filledAt.Valid {
		e.FilledAt = &filledAt.Time
	}
	return &e, nil
}

/*
OpenPositionsForDate returns decisions for the given trade_date that
have a filled open execution but no filled close execution. Used by
the 3:55pm cron to find what needs to be closed.
*/
func (s *Store) OpenPositionsForDate(tradeDate string) ([]exec.Decision, error) {
	rows, err := s.db.Query(`
		SELECT d.id, d.trade_date, d.symbol, d.contract_type, d.strike_price, d.expiration,
			d.occ_symbol, d.contract_price, d.gpt_score, d.claude_score,
			COALESCE(d.trade_id, 0), COALESCE(d.token_hash, ''), d.decision, d.expires_at, d.created_at
		FROM execution_decisions d
		WHERE d.trade_date = $1
		  AND d.decision = 'execute'
		  AND EXISTS (
			SELECT 1 FROM executions e
			WHERE e.decision_id = d.id AND e.side = 'open' AND e.status = 'filled'
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM executions e
			WHERE e.decision_id = d.id AND e.side = 'close' AND e.status = 'filled'
		  )
	`, tradeDate)
	if err != nil {
		return nil, fmt.Errorf("query open positions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []exec.Decision
	for rows.Next() {
		var d exec.Decision
		if err := rows.Scan(&d.ID, &d.TradeDate, &d.Symbol, &d.ContractType, &d.StrikePrice, &d.Expiration,
			&d.OCCSymbol, &d.ContractPrice, &d.GPTScore, &d.ClaudeScore,
			&d.TradeID, &d.TokenHash, &d.Decision, &d.ExpiresAt, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan open position: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

/*
OpenExecutionForDecision returns the most recent open-side execution
for a decision (filled or otherwise). Used by the close cron to
recover the actual entry fill price for accurate realized-P&L
computation in live mode (where slippage means the open fill can
diverge from decision.ContractPrice).
*/
func (s *Store) OpenExecutionForDecision(decisionID int) (*exec.Execution, error) {
	var e exec.Execution
	var schwabOrderID sql.NullString
	var fillPrice sql.NullFloat64
	var filledAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, decision_id, mode, side, schwab_order_id, status,
			fill_price, filled_quantity, requested_quantity,
			submitted_at, filled_at, error_message, created_at
		FROM executions
		WHERE decision_id = $1 AND side = 'open'
		ORDER BY id DESC
		LIMIT 1
	`, decisionID).Scan(&e.ID, &e.DecisionID, &e.Mode, &e.Side, &schwabOrderID, &e.Status,
		&fillPrice, &e.FilledQuantity, &e.RequestedQuantity,
		&e.SubmittedAt, &filledAt, &e.ErrorMessage, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("no open execution for decision %d", decisionID)
	}
	if err != nil {
		return nil, fmt.Errorf("get open execution: %w", err)
	}
	if schwabOrderID.Valid {
		v := schwabOrderID.String
		e.SchwabOrderID = &v
	}
	if fillPrice.Valid {
		v := fillPrice.Float64
		e.FillPrice = &v
	}
	if filledAt.Valid {
		e.FilledAt = &filledAt.Time
	}
	return &e, nil
}

/*
LiveExecutionsForDecision returns every execution row for a decision
that's NOT in a terminal state — i.e. still pending or working at the
broker. Used by the cancel-all kill switch to find what to cancel.
*/
func (s *Store) LiveExecutionsForDecision(decisionID int) ([]exec.Execution, error) {
	rows, err := s.db.Query(`
		SELECT id, decision_id, mode, side, schwab_order_id, status,
			fill_price, filled_quantity, requested_quantity,
			submitted_at, filled_at, error_message, created_at
		FROM executions
		WHERE decision_id = $1 AND status IN ('pending', 'working')
		ORDER BY id ASC
	`, decisionID)
	if err != nil {
		return nil, fmt.Errorf("query live executions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []exec.Execution
	for rows.Next() {
		var e exec.Execution
		var schwabOrderID sql.NullString
		var fillPrice sql.NullFloat64
		var filledAt sql.NullTime
		if err := rows.Scan(&e.ID, &e.DecisionID, &e.Mode, &e.Side, &schwabOrderID, &e.Status,
			&fillPrice, &e.FilledQuantity, &e.RequestedQuantity,
			&e.SubmittedAt, &filledAt, &e.ErrorMessage, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan live execution: %w", err)
		}
		if schwabOrderID.Valid {
			v := schwabOrderID.String
			e.SchwabOrderID = &v
		}
		if fillPrice.Valid {
			v := fillPrice.Float64
			e.FillPrice = &v
		}
		if filledAt.Valid {
			e.FilledAt = &filledAt.Time
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

/*
nullableInt converts a zero int to a SQL NULL so the trade_id FK is
stored as NULL when the caller doesn't have a backing trade row yet.
*/
func nullableInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

/*
ExecutionView is the lightweight projection surfaced to the public
dashboard/history/trade-detail UI when Jayce has taken a position
(paper or live) on a trade. Joins execution_decisions + the open
execution row + the optional close execution row into a single shape
the frontend can render a badge from. State is derived server-side
so the client never has to reason about partial fills or the close
cron's lifecycle.
*/
type ExecutionView struct {
	Mode         string     `json:"mode"`  // paper | live
	State        string     `json:"state"` // holding | closed | failed
	Symbol       string     `json:"symbol"`
	ContractType string     `json:"contract_type"`
	StrikePrice  float64    `json:"strike_price"`
	OpenPrice    float64    `json:"open_price"`            // 0 if not yet filled
	ClosePrice   float64    `json:"close_price"`           // 0 if not yet closed
	RealizedPnL  float64    `json:"realized_pnl"`          // (close - open) * 100 * 1; 0 until closed
	ExecutedAt   *time.Time `json:"executed_at,omitempty"` // when open filled
	ClosedAt     *time.Time `json:"closed_at,omitempty"`   // when close filled
}

/*
GetExecutionForDate returns the execution view for a single trade
date, or nil if no decision was confirmed that day. Paper and live
are both surfaced — the Mode field carries the distinction. Pending,
declined, and timeout decisions do NOT surface (no position was
actually taken, so there's nothing to be transparent about).
*/
func (s *Store) GetExecutionForDate(date string) (*ExecutionView, error) {
	row := s.db.QueryRow(`
		SELECT
			d.symbol, d.contract_type, d.strike_price,
			openX.mode, openX.status,
			COALESCE(openX.fill_price, 0), openX.filled_at,
			COALESCE(closeX.fill_price, 0), closeX.filled_at, closeX.status
		FROM execution_decisions d
		INNER JOIN executions openX
			ON openX.decision_id = d.id AND openX.side = 'open'
		LEFT JOIN LATERAL (
			SELECT * FROM executions
			WHERE decision_id = d.id AND side = 'close'
			ORDER BY id DESC LIMIT 1
		) closeX ON true
		WHERE d.trade_date = $1 AND d.decision = 'execute'
		ORDER BY openX.id ASC
		LIMIT 1
	`, date)
	v, err := scanExecutionView(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return v, err
}

/*
GetExecutionsForDateRange returns a map of trade_date → ExecutionView
for the history/week view. Only dates with confirmed executions appear
in the map; days where no qualifying pick existed are simply absent.
*/
func (s *Store) GetExecutionsForDateRange(start, end string) (map[string]*ExecutionView, error) {
	rows, err := s.db.Query(`
		SELECT
			d.trade_date,
			d.symbol, d.contract_type, d.strike_price,
			openX.mode, openX.status,
			COALESCE(openX.fill_price, 0), openX.filled_at,
			COALESCE(closeX.fill_price, 0), closeX.filled_at, closeX.status
		FROM execution_decisions d
		INNER JOIN executions openX
			ON openX.decision_id = d.id AND openX.side = 'open'
		LEFT JOIN LATERAL (
			SELECT * FROM executions
			WHERE decision_id = d.id AND side = 'close'
			ORDER BY id DESC LIMIT 1
		) closeX ON true
		WHERE d.trade_date >= $1 AND d.trade_date <= $2 AND d.decision = 'execute'
		ORDER BY d.trade_date ASC, openX.id ASC
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("query executions range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]*ExecutionView)
	for rows.Next() {
		var date string
		v := &ExecutionView{}
		var openStatus string
		var executedAt, closedAt sql.NullTime
		var closeStatus sql.NullString
		if err := rows.Scan(
			&date,
			&v.Symbol, &v.ContractType, &v.StrikePrice,
			&v.Mode, &openStatus,
			&v.OpenPrice, &executedAt,
			&v.ClosePrice, &closedAt, &closeStatus,
		); err != nil {
			return nil, fmt.Errorf("scan execution range row: %w", err)
		}
		v.State = deriveExecutionState(openStatus, closeStatus)
		if executedAt.Valid {
			t := executedAt.Time
			v.ExecutedAt = &t
		}
		if closedAt.Valid {
			t := closedAt.Time
			v.ClosedAt = &t
		}
		if v.State == "closed" && v.OpenPrice > 0 && v.ClosePrice > 0 {
			v.RealizedPnL = (v.ClosePrice - v.OpenPrice) * 100
		}
		/*
			Only surface dates where a real position was taken (skip
			failed/pending). The state derivation already filtered out
			non-execute decisions via the WHERE clause; here we drop the
			row if the open never filled.
		*/
		if v.State == "" {
			continue
		}
		out[date] = v
	}
	return out, rows.Err()
}

/*
scanExecutionView reads a single ExecutionView row from a *sql.Row.
Shared between the date and range queries.
*/
func scanExecutionView(row *sql.Row) (*ExecutionView, error) {
	v := &ExecutionView{}
	var openStatus string
	var executedAt, closedAt sql.NullTime
	var closeStatus sql.NullString
	if err := row.Scan(
		&v.Symbol, &v.ContractType, &v.StrikePrice,
		&v.Mode, &openStatus,
		&v.OpenPrice, &executedAt,
		&v.ClosePrice, &closedAt, &closeStatus,
	); err != nil {
		return nil, err
	}
	v.State = deriveExecutionState(openStatus, closeStatus)
	if v.State == "" {
		return nil, sql.ErrNoRows
	}
	if executedAt.Valid {
		t := executedAt.Time
		v.ExecutedAt = &t
	}
	if closedAt.Valid {
		t := closedAt.Time
		v.ClosedAt = &t
	}
	if v.State == "closed" && v.OpenPrice > 0 && v.ClosePrice > 0 {
		v.RealizedPnL = (v.ClosePrice - v.OpenPrice) * 100
	}
	return v, nil
}

/*
deriveExecutionState collapses the open/close status pair into the
single string the frontend renders. Returns empty string when the
open never reached a terminal-or-filled state — caller treats that
as "no position to surface".
*/
func deriveExecutionState(openStatus string, closeStatus sql.NullString) string {
	switch openStatus {
	case "filled":
		// Open succeeded. Did close also succeed?
		if closeStatus.Valid && closeStatus.String == "filled" {
			return "closed"
		}
		return "holding"
	case "failed", "rejected":
		return "failed"
	default:
		// 'pending' or 'working' — surface nothing yet.
		return ""
	}
}
