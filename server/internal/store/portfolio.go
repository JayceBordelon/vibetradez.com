package store

import (
	"database/sql"
	"fmt"
)

/*
PortfolioDecisionRow is one persisted move (or hold) the v2 portfolio
manager committed. It is the store-layer shape, deliberately decoupled
from internal/portfolio's Decision type so the store package does not
import portfolio: the cron adapter (task 4) maps between them. Strike and
FillPrice are pointers because the columns are nullable (holds and equity
moves leave them null).
*/
type PortfolioDecisionRow struct {
	ID            int
	Uuid          string // stable URL-safe id; anchors the derived trade's detail route
	Date          string
	Source        string // "agent" | "risk_cron"
	Action        string // buy_equity | sell_equity | buy_option | sell_option | hold
	AssetType     string // EQUITY | OPTION | "" for holds
	Symbol        string
	Underlying    string
	ContractType  string
	Strike        *float64
	Expiration    string
	Quantity      float64
	LimitPrice    float64
	Notional      float64
	Mode          string // always "live"
	SchwabOrderID string
	Status        string
	FillPrice     *float64
	ExecutionID   int
	Rationale     string
}

/*
EquityCurvePoint is one day of the account's equity curve plus the SPY
close for benchmarking. Upserted by date by the 16:00 EOD snapshot cron.
*/
type EquityCurvePoint struct {
	Date          string
	AccountEquity float64
	SettledCash   float64
	UnsettledCash float64
	HighWaterMark float64
	SPYClose      float64
}

/*
PortfolioPositionRow is one held position in a daily book snapshot. Strike
is a pointer because it's null for equity. CostBasis is the total cost of
the position; MarketValue is its current mark.
*/
type PortfolioPositionRow struct {
	Date         string
	Symbol       string
	Underlying   string
	AssetType    string
	ContractType string
	Strike       *float64
	Expiration   string
	Quantity     float64
	MarketValue  float64
	CostBasis    float64
}

/*
SavePositionsSnapshot replaces the book snapshot for a date (delete then
insert) so a re-run of the EOD cron overwrites cleanly.
*/
func (s *Store) SavePositionsSnapshot(date string, rows []PortfolioPositionRow) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin positions snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("DELETE FROM portfolio_positions WHERE date = $1", date); err != nil {
		return fmt.Errorf("clear positions snapshot: %w", err)
	}
	for _, r := range rows {
		if _, err := tx.Exec(`
			INSERT INTO portfolio_positions
				(date, symbol, underlying, asset_type, contract_type, strike, expiration, quantity, market_value, cost_basis)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			date, r.Symbol, r.Underlying, r.AssetType, r.ContractType,
			nullableFloat(r.Strike), nullableEmptyText(r.Expiration), r.Quantity, r.MarketValue, r.CostBasis,
		); err != nil {
			return fmt.Errorf("insert position %s: %w", r.Symbol, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit positions snapshot: %w", err)
	}
	return nil
}

/*
GetLatestPositions returns the most recent book snapshot (all rows for the
max date), or an empty slice when none exists.
*/
func (s *Store) GetLatestPositions() ([]PortfolioPositionRow, error) {
	rows, err := s.db.Query(`
		SELECT date, symbol, underlying, asset_type, contract_type, strike, expiration, quantity, market_value, cost_basis
		FROM portfolio_positions
		WHERE date = (SELECT MAX(date) FROM portfolio_positions)
		ORDER BY market_value DESC`)
	if err != nil {
		return nil, fmt.Errorf("query latest positions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []PortfolioPositionRow
	for rows.Next() {
		var p PortfolioPositionRow
		var strike sql.NullFloat64
		var expiration sql.NullString
		if err := rows.Scan(&p.Date, &p.Symbol, &p.Underlying, &p.AssetType, &p.ContractType,
			&strike, &expiration, &p.Quantity, &p.MarketValue, &p.CostBasis); err != nil {
			return nil, fmt.Errorf("scan position: %w", err)
		}
		if strike.Valid {
			p.Strike = &strike.Float64
		}
		if expiration.Valid {
			p.Expiration = expiration.String
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

/*
SavePortfolioDecision inserts one committed move/hold and returns its row
id. Source defaults to "agent" and Mode to "live" when blank so callers
can omit them in the common case.
*/
func (s *Store) SavePortfolioDecision(d PortfolioDecisionRow) (int, error) {
	if d.Source == "" {
		d.Source = "agent"
	}
	if d.Mode == "" {
		d.Mode = "live"
	}
	var id int
	err := s.db.QueryRow(`
		INSERT INTO portfolio_decisions
			(date, source, action, asset_type, symbol, underlying, contract_type,
			 strike, expiration, quantity, limit_price, notional, mode,
			 schwab_order_id, status, fill_price, execution_id, rationale)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING id`,
		d.Date, d.Source, d.Action, d.AssetType, d.Symbol, d.Underlying, d.ContractType,
		nullableFloat(d.Strike), nullableEmptyText(d.Expiration), d.Quantity, d.LimitPrice, d.Notional, d.Mode,
		d.SchwabOrderID, d.Status, nullableFloat(d.FillPrice), d.ExecutionID, d.Rationale,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("save portfolio decision: %w", err)
	}
	return id, nil
}

/*
GetPortfolioDecisions returns every decision recorded for a date, oldest
first (commit order within the day, then by id).
*/
func (s *Store) GetPortfolioDecisions(date string) ([]PortfolioDecisionRow, error) {
	rows, err := s.db.Query(`
		SELECT id, uuid, date, source, action, asset_type, symbol, underlying, contract_type,
		       strike, expiration, quantity, limit_price, notional, mode,
		       schwab_order_id, status, fill_price, execution_id, rationale
		FROM portfolio_decisions
		WHERE date = $1
		ORDER BY created_at ASC, id ASC`, date)
	if err != nil {
		return nil, fmt.Errorf("query portfolio decisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []PortfolioDecisionRow
	for rows.Next() {
		var d PortfolioDecisionRow
		var strike, fillPrice sql.NullFloat64
		var expiration sql.NullString
		if err := rows.Scan(
			&d.ID, &d.Uuid, &d.Date, &d.Source, &d.Action, &d.AssetType, &d.Symbol, &d.Underlying, &d.ContractType,
			&strike, &expiration, &d.Quantity, &d.LimitPrice, &d.Notional, &d.Mode,
			&d.SchwabOrderID, &d.Status, &fillPrice, &d.ExecutionID, &d.Rationale,
		); err != nil {
			return nil, fmt.Errorf("scan portfolio decision: %w", err)
		}
		if strike.Valid {
			d.Strike = &strike.Float64
		}
		if fillPrice.Valid {
			d.FillPrice = &fillPrice.Float64
		}
		if expiration.Valid {
			d.Expiration = expiration.String
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

/*
RecentPortfolioDecisions returns the most recent decisions across all
days, newest first, capped at limit. Powers the agent's get_recent_decisions
tool so a hold-across-days manager can see its own recent moves and keep a
coherent thesis instead of re-deriving each session.
*/
func (s *Store) RecentPortfolioDecisions(limit int) ([]PortfolioDecisionRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT id, uuid, date, source, action, asset_type, symbol, underlying, contract_type,
		       strike, expiration, quantity, limit_price, notional, mode,
		       schwab_order_id, status, fill_price, execution_id, rationale
		FROM portfolio_decisions
		ORDER BY created_at DESC, id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent portfolio decisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []PortfolioDecisionRow
	for rows.Next() {
		var d PortfolioDecisionRow
		var strike, fillPrice sql.NullFloat64
		var expiration sql.NullString
		if err := rows.Scan(
			&d.ID, &d.Uuid, &d.Date, &d.Source, &d.Action, &d.AssetType, &d.Symbol, &d.Underlying, &d.ContractType,
			&strike, &expiration, &d.Quantity, &d.LimitPrice, &d.Notional, &d.Mode,
			&d.SchwabOrderID, &d.Status, &fillPrice, &d.ExecutionID, &d.Rationale,
		); err != nil {
			return nil, fmt.Errorf("scan recent portfolio decision: %w", err)
		}
		if strike.Valid {
			d.Strike = &strike.Float64
		}
		if fillPrice.Valid {
			d.FillPrice = &fillPrice.Float64
		}
		if expiration.Valid {
			d.Expiration = expiration.String
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

/*
AllPortfolioDecisions returns every recorded decision in chronological
order (date asc, then id asc). The closed-trade derivation walks this
forward to pair buys with the sells that flatten them, so order matters.
*/
func (s *Store) AllPortfolioDecisions() ([]PortfolioDecisionRow, error) {
	rows, err := s.db.Query(`
		SELECT id, uuid, date, source, action, asset_type, symbol, underlying, contract_type,
		       strike, expiration, quantity, limit_price, notional, mode,
		       schwab_order_id, status, fill_price, execution_id, rationale
		FROM portfolio_decisions
		ORDER BY date ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query all portfolio decisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []PortfolioDecisionRow
	for rows.Next() {
		var d PortfolioDecisionRow
		var strike, fillPrice sql.NullFloat64
		var expiration sql.NullString
		if err := rows.Scan(
			&d.ID, &d.Uuid, &d.Date, &d.Source, &d.Action, &d.AssetType, &d.Symbol, &d.Underlying, &d.ContractType,
			&strike, &expiration, &d.Quantity, &d.LimitPrice, &d.Notional, &d.Mode,
			&d.SchwabOrderID, &d.Status, &fillPrice, &d.ExecutionID, &d.Rationale,
		); err != nil {
			return nil, fmt.Errorf("scan portfolio decision: %w", err)
		}
		if strike.Valid {
			d.Strike = &strike.Float64
		}
		if fillPrice.Valid {
			d.FillPrice = &fillPrice.Float64
		}
		if expiration.Valid {
			d.Expiration = expiration.String
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

/*
PortfolioStanceRow is one day's recorded stance note.
*/
type PortfolioStanceRow struct {
	Date   string
	Mode   string
	Stance string
}

/*
RecentPortfolioStances returns the most recent daily stance notes, newest
first, capped at limit.
*/
func (s *Store) RecentPortfolioStances(limit int) ([]PortfolioStanceRow, error) {
	if limit <= 0 {
		limit = 7
	}
	rows, err := s.db.Query(`
		SELECT date, mode, stance
		FROM portfolio_sessions
		ORDER BY date DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent portfolio stances: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []PortfolioStanceRow
	for rows.Next() {
		var r PortfolioStanceRow
		if err := rows.Scan(&r.Date, &r.Mode, &r.Stance); err != nil {
			return nil, fmt.Errorf("scan recent portfolio stance: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

/*
SavePortfolioSession upserts the day's stance note + plain-language session
summary (from the write_summary documentation tool) by date.
*/
func (s *Store) SavePortfolioSession(date, mode, stance, summary, actionItems string) error {
	if mode == "" {
		mode = "live"
	}
	_, err := s.db.Exec(`
		INSERT INTO portfolio_sessions (date, mode, stance, summary, action_items, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (date) DO UPDATE SET mode = EXCLUDED.mode, stance = EXCLUDED.stance, summary = EXCLUDED.summary, action_items = EXCLUDED.action_items, updated_at = NOW()`,
		date, mode, stance, summary, actionItems)
	if err != nil {
		return fmt.Errorf("save portfolio session: %w", err)
	}
	return nil
}

/*
GetPortfolioSession returns the mode, stance, synopsis (summary), and the
next-session action items for a date. ok is false when no session was
recorded that day.
*/
func (s *Store) GetPortfolioSession(date string) (mode, stance, summary, actionItems string, ok bool, err error) {
	row := s.db.QueryRow(`SELECT mode, stance, summary, action_items FROM portfolio_sessions WHERE date = $1`, date)
	switch err := row.Scan(&mode, &stance, &summary, &actionItems); err {
	case nil:
		return mode, stance, summary, actionItems, true, nil
	case sql.ErrNoRows:
		return "", "", "", "", false, nil
	default:
		return "", "", "", "", false, fmt.Errorf("get portfolio session: %w", err)
	}
}

/*
LatestPortfolioSession returns the most recent session's synopsis + action
items, newest by date. At session start (before today's row is written) this
is the prior trading day's plan, which the agent reads to continue where it
left off. ok is false when no session has ever been recorded.
*/
func (s *Store) LatestPortfolioSession() (synopsis, actionItems string, ok bool, err error) {
	row := s.db.QueryRow(`SELECT summary, action_items FROM portfolio_sessions ORDER BY date DESC LIMIT 1`)
	switch err := row.Scan(&synopsis, &actionItems); err {
	case nil:
		return synopsis, actionItems, true, nil
	case sql.ErrNoRows:
		return "", "", false, nil
	default:
		return "", "", false, fmt.Errorf("latest portfolio session: %w", err)
	}
}

/*
SaveEquityCurvePoint upserts one day of the equity curve by date.
*/
func (s *Store) SaveEquityCurvePoint(p EquityCurvePoint) error {
	_, err := s.db.Exec(`
		INSERT INTO portfolio_equity_curve
			(date, account_equity, settled_cash, unsettled_cash, high_water_mark, spy_close)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (date) DO UPDATE SET
			account_equity  = EXCLUDED.account_equity,
			settled_cash    = EXCLUDED.settled_cash,
			unsettled_cash  = EXCLUDED.unsettled_cash,
			high_water_mark = EXCLUDED.high_water_mark,
			spy_close       = EXCLUDED.spy_close`,
		p.Date, p.AccountEquity, p.SettledCash, p.UnsettledCash, p.HighWaterMark, p.SPYClose)
	if err != nil {
		return fmt.Errorf("save equity curve point: %w", err)
	}
	return nil
}

/*
GetEquityCurve returns the equity-curve points in [startDate, endDate]
inclusive, oldest first.
*/
func (s *Store) GetEquityCurve(startDate, endDate string) ([]EquityCurvePoint, error) {
	rows, err := s.db.Query(`
		SELECT date, account_equity, settled_cash, unsettled_cash, high_water_mark, spy_close
		FROM portfolio_equity_curve
		WHERE date >= $1 AND date <= $2
		ORDER BY date ASC`, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("query equity curve: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []EquityCurvePoint
	for rows.Next() {
		var p EquityCurvePoint
		if err := rows.Scan(&p.Date, &p.AccountEquity, &p.SettledCash, &p.UnsettledCash, &p.HighWaterMark, &p.SPYClose); err != nil {
			return nil, fmt.Errorf("scan equity curve point: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

/*
GetHighWaterMark returns the peak account equity ever recorded on the
curve (the max of every day's account_equity and its persisted
high_water_mark). The drawdown breaker reads this at session start.
Returns 0 with ok=false when the curve is empty (fresh account), which the
caller treats as "no drawdown halt yet".
*/
func (s *Store) GetHighWaterMark() (mark float64, ok bool, err error) {
	var hwm sql.NullFloat64
	row := s.db.QueryRow(`SELECT GREATEST(MAX(account_equity), MAX(high_water_mark)) FROM portfolio_equity_curve`)
	if err := row.Scan(&hwm); err != nil {
		return 0, false, fmt.Errorf("get high-water mark: %w", err)
	}
	if !hwm.Valid {
		return 0, false, nil
	}
	return hwm.Float64, true, nil
}

/*
nullableEmptyText maps an empty string to a SQL NULL so optional text
columns (expiration on equity/hold rows) store NULL rather than ”.
*/
func nullableEmptyText(s string) any {
	if s == "" {
		return nil
	}
	return s
}
