package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"vibetradez.com/internal/exec"
)

/*
InsertExecution records an order submission (paper or live). Returns
the new row id. The caller is responsible for setting Status correctly
based on the trader's response.
*/
func (s *Store) InsertExecution(e exec.Execution) (int, error) {
	var id int
	err := s.db.QueryRow(`
		INSERT INTO executions (
			trade_id, mode, side, schwab_order_id, status,
			fill_price, filled_quantity, requested_quantity,
			filled_at, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, nullableInt(e.TradeID), e.Mode, e.Side, e.SchwabOrderID, e.Status,
		e.FillPrice, e.FilledQuantity, e.RequestedQuantity,
		e.FilledAt, e.ErrorMessage).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert execution: %w", err)
	}
	return id, nil
}

/*
UpdateExecutionStatus updates fill state on an existing execution row.
Used as orders progress from working -> filled (or canceled / failed).
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
	var tradeID sql.NullInt64
	err := s.db.QueryRow(`
		SELECT id, trade_id, mode, side, schwab_order_id, status,
			fill_price, filled_quantity, requested_quantity,
			submitted_at, filled_at, error_message, created_at
		FROM executions WHERE id = $1
	`, id).Scan(&e.ID, &tradeID, &e.Mode, &e.Side, &schwabOrderID, &e.Status,
		&fillPrice, &e.FilledQuantity, &e.RequestedQuantity,
		&e.SubmittedAt, &filledAt, &e.ErrorMessage, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("no execution with id %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get execution: %w", err)
	}
	if tradeID.Valid {
		e.TradeID = int(tradeID.Int64)
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
OpenPositionsForDate returns positions for the given trade_date that
have a filled open execution but no filled close execution. Used by
the 3:55pm cron to find what needs to be closed. Joins executions to
trades so the close cron has the full contract spec without a second
query. Returns exec.OpenPosition values directly (the type lives in
the exec package to keep the import graph one-directional).
*/
func (s *Store) OpenPositionsForDate(tradeDate string) ([]exec.OpenPosition, error) {
	rows, err := s.db.Query(`
		SELECT e.id, e.trade_id, e.mode, e.side, e.schwab_order_id, e.status,
			e.fill_price, e.filled_quantity, e.requested_quantity,
			e.submitted_at, e.filled_at, e.error_message, e.created_at,
			t.symbol, t.contract_type, t.strike_price, t.expiration, t.estimated_price
		FROM executions e
		INNER JOIN trades t ON t.id = e.trade_id
		WHERE t.date = $1
		  AND e.side = 'open'
		  AND e.status = 'filled'
		  AND NOT EXISTS (
			SELECT 1 FROM executions c
			WHERE c.trade_id = e.trade_id AND c.side = 'close' AND c.status = 'filled'
		  )
	`, tradeDate)
	if err != nil {
		return nil, fmt.Errorf("query open positions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []exec.OpenPosition
	for rows.Next() {
		var p exec.OpenPosition
		var schwabOrderID sql.NullString
		var fillPrice sql.NullFloat64
		var filledAt sql.NullTime
		var tradeID sql.NullInt64
		if err := rows.Scan(
			&p.Execution.ID, &tradeID, &p.Execution.Mode, &p.Execution.Side,
			&schwabOrderID, &p.Execution.Status,
			&fillPrice, &p.Execution.FilledQuantity, &p.Execution.RequestedQuantity,
			&p.Execution.SubmittedAt, &filledAt, &p.Execution.ErrorMessage, &p.Execution.CreatedAt,
			&p.Symbol, &p.ContractType, &p.StrikePrice, &p.Expiration, &p.ContractPrice,
		); err != nil {
			return nil, fmt.Errorf("scan open position: %w", err)
		}
		if tradeID.Valid {
			p.Execution.TradeID = int(tradeID.Int64)
		}
		if schwabOrderID.Valid {
			v := schwabOrderID.String
			p.Execution.SchwabOrderID = &v
		}
		if fillPrice.Valid {
			v := fillPrice.Float64
			p.Execution.FillPrice = &v
		}
		if filledAt.Valid {
			p.Execution.FilledAt = &filledAt.Time
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

/*
OpenExecutionForTrade returns the most recent open-side execution for
a trade (filled or otherwise). Used by the close cron to recover the
actual entry fill price for accurate realized-P&L computation in live
mode (where slippage means the open fill can diverge from
trade.estimated_price).
*/
func (s *Store) OpenExecutionForTrade(tradeID int) (*exec.Execution, error) {
	var e exec.Execution
	var schwabOrderID sql.NullString
	var fillPrice sql.NullFloat64
	var filledAt sql.NullTime
	var tid sql.NullInt64
	err := s.db.QueryRow(`
		SELECT id, trade_id, mode, side, schwab_order_id, status,
			fill_price, filled_quantity, requested_quantity,
			submitted_at, filled_at, error_message, created_at
		FROM executions
		WHERE trade_id = $1 AND side = 'open'
		ORDER BY id DESC
		LIMIT 1
	`, tradeID).Scan(&e.ID, &tid, &e.Mode, &e.Side, &schwabOrderID, &e.Status,
		&fillPrice, &e.FilledQuantity, &e.RequestedQuantity,
		&e.SubmittedAt, &filledAt, &e.ErrorMessage, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("no open execution for trade %d", tradeID)
	}
	if err != nil {
		return nil, fmt.Errorf("get open execution: %w", err)
	}
	if tid.Valid {
		e.TradeID = int(tid.Int64)
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
LiveExecutionsForDate returns every execution row for the given trade
date that's NOT in a terminal state, still pending or working at the
broker. Used by the cancel-all kill switch to find what to cancel.
*/
func (s *Store) LiveExecutionsForDate(tradeDate string) ([]exec.Execution, error) {
	rows, err := s.db.Query(`
		SELECT e.id, e.trade_id, e.mode, e.side, e.schwab_order_id, e.status,
			e.fill_price, e.filled_quantity, e.requested_quantity,
			e.submitted_at, e.filled_at, e.error_message, e.created_at
		FROM executions e
		INNER JOIN trades t ON t.id = e.trade_id
		WHERE t.date = $1 AND e.status IN ('pending', 'working')
		ORDER BY e.id ASC
	`, tradeDate)
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
		var tid sql.NullInt64
		if err := rows.Scan(&e.ID, &tid, &e.Mode, &e.Side, &schwabOrderID, &e.Status,
			&fillPrice, &e.FilledQuantity, &e.RequestedQuantity,
			&e.SubmittedAt, &filledAt, &e.ErrorMessage, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan live execution: %w", err)
		}
		if tid.Valid {
			e.TradeID = int(tid.Int64)
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
stored as NULL when the caller doesn't have a backing trade row yet
(should never happen post-refactor, kept as defensive backstop).
*/
func nullableInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

/*
ExecutionView is the lightweight projection surfaced to the public
dashboard/history/trade-detail UI when a position has been auto-fired
on a trade (paper or live). Joins the trade row + the open execution
+ the optional close execution row into a single shape the frontend
can render a badge from. State is derived server-side so the client
never has to reason about partial fills or the close cron's lifecycle.
*/
type ExecutionView struct {
	Mode         string     `json:"mode"`
	State        string     `json:"state"`
	Symbol       string     `json:"symbol"`
	ContractType string     `json:"contract_type"`
	StrikePrice  float64    `json:"strike_price"`
	OpenPrice    float64    `json:"open_price"`
	ClosePrice   float64    `json:"close_price"`
	RealizedPnL  float64    `json:"realized_pnl"`
	ExecutedAt   *time.Time `json:"executed_at,omitempty"`
	ClosedAt     *time.Time `json:"closed_at,omitempty"`
}

/*
GetExecutionForDate returns the execution view for a single trade
date, or nil if no auto-execution fired that day. Paper and live are
both surfaced, the Mode field carries the distinction. Failed open
executions DO surface (with state='failed') so the dashboard can show
what didn't work.
*/
func (s *Store) GetExecutionForDate(date string) (*ExecutionView, error) {
	row := s.db.QueryRow(`
		SELECT
			t.symbol, t.contract_type, t.strike_price,
			openX.mode, openX.status,
			COALESCE(openX.fill_price, 0), openX.filled_at,
			COALESCE(closeX.fill_price, 0), closeX.filled_at, closeX.status
		FROM executions openX
		INNER JOIN trades t ON t.id = openX.trade_id
		LEFT JOIN LATERAL (
			SELECT * FROM executions
			WHERE trade_id = openX.trade_id AND side = 'close'
			ORDER BY id DESC LIMIT 1
		) closeX ON true
		WHERE t.date = $1 AND openX.side = 'open'
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
GetExecutionsForDateRange returns a map of trade_date -> ExecutionView
for the history/week view. Only dates with confirmed executions appear
in the map; days with no auto-execution are simply absent.
*/
func (s *Store) GetExecutionsForDateRange(start, end string) (map[string]*ExecutionView, error) {
	rows, err := s.db.Query(`
		SELECT
			t.date,
			t.symbol, t.contract_type, t.strike_price,
			openX.mode, openX.status,
			COALESCE(openX.fill_price, 0), openX.filled_at,
			COALESCE(closeX.fill_price, 0), closeX.filled_at, closeX.status
		FROM executions openX
		INNER JOIN trades t ON t.id = openX.trade_id
		LEFT JOIN LATERAL (
			SELECT * FROM executions
			WHERE trade_id = openX.trade_id AND side = 'close'
			ORDER BY id DESC LIMIT 1
		) closeX ON true
		WHERE t.date >= $1 AND t.date <= $2 AND openX.side = 'open'
		ORDER BY t.date ASC, openX.id ASC
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
		if v.State == "" {
			continue
		}
		out[date] = v
	}
	return out, rows.Err()
}

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
open never reached a terminal-or-filled state, caller treats that
as "no position to surface".
*/
func deriveExecutionState(openStatus string, closeStatus sql.NullString) string {
	switch openStatus {
	case "filled":
		if closeStatus.Valid && closeStatus.String == "filled" {
			return "closed"
		}
		return "holding"
	case "failed", "rejected":
		return "failed"
	default:
		return ""
	}
}
