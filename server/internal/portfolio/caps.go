package portfolio

import (
	"fmt"
	"strings"
)

/*
Caps is the hard-cap risk policy. Every percentage is a fraction of LIVE
account equity read at the start of the session, so the policy scales with
the account rather than being pinned to a starting balance. These are
POLICY, not advisory: with a maximize-account-value mandate the agent is
assumed to run at or near every cap, so these values are the actual risk
the operator is accepting.

All caps are enforced at the tool layer (tools.go) via the Check* methods
below, and re-validated at the broker entry point (exec) so a buggy
tool-layer change cannot widen them. This struct is the single source of
truth for the numbers; CapsSummary renders them into the agent prompt so the
prose and the enforced values never drift.
*/
type Caps struct {
	// MaxPerUnderlyingPct caps total exposure (equity plus option premium)
	// to any single underlying as a fraction of equity. Allows real
	// conviction, blocks all-in on one name.
	MaxPerUnderlyingPct float64

	// MaxOptionsSleevePct caps the total option premium held at once as a
	// fraction of equity. Options are the leverage sleeve, bounded so a
	// volatility crush cannot wipe the account.
	MaxOptionsSleevePct float64

	// MaxPerOrderPct caps a single order's notional as a fraction of
	// equity, so one order cannot deploy the whole book.
	MaxPerOrderPct float64

	// DrawdownHaltPct halts all new buys (de-risk only) once account equity
	// is down this fraction from its high-water mark.
	DrawdownHaltPct float64

	// Liquidity floor. A buy whose instrument fails any of these is refused.
	MinStockPrice         float64 // underlying must trade at or above this
	MinUnderlyingMktCap   float64 // underlying market cap floor, USD
	MinOptionOpenInterest int     // option OI floor (options only)
	MinOptionVolume       int     // option day-volume floor (options only)
	MaxSpreadPct          float64 // widest acceptable (ask-bid)/mark

	// MinHeldOptionDTE is the days-to-expiration floor below which the risk
	// cron force-closes or rolls a held option, to avoid the overnight
	// theta/gamma cliff. Not a buy-time gate; consumed by the risk cron.
	MinHeldOptionDTE int
}

/*
DefaultCaps returns the v2 launch policy from the design doc, expressed as
percentages of equity so it holds at any account size:

  - 40% max exposure per underlying
  - 50% max total option premium (the leverage sleeve)
  - 30% max per single order
  - new buys halt at -35% from the high-water mark
  - liquidity floor: stock >= $5, market cap >= $2B, option OI >= 500 and
    volume >= 100, spread <= 10%
  - force close/roll held options under 7 DTE

The daily-deployment cap (75% of settled cash per session) is applied when
building the Snapshot's DeploymentBudget, not stored here, because it keys
off session-start cash rather than equity.
*/
func DefaultCaps() Caps {
	return Caps{
		MaxPerUnderlyingPct:   0.40,
		MaxOptionsSleevePct:   0.50,
		MaxPerOrderPct:        0.30,
		DrawdownHaltPct:       0.35,
		MinStockPrice:         5.00,
		MinUnderlyingMktCap:   2_000_000_000,
		MinOptionOpenInterest: 500,
		MinOptionVolume:       100,
		MaxSpreadPct:          0.10,
		MinHeldOptionDTE:      7,
	}
}

// DefaultDailyDeploymentPct is the fraction of session-start settled cash
// the agent may commit to NEW buys in one session. Kept separate from Caps
// because it keys off cash, not equity. Used when constructing a Snapshot.
const DefaultDailyDeploymentPct = 0.75

// floatTol absorbs floating-point noise so a move sized exactly to a cap
// (e.g. notional == budget) is allowed rather than rejected by a rounding
// hair.
const floatTol = 1e-6

/*
CapError is a single rejected-by-policy outcome. Code is a stable
machine-readable tag (useful for the dashboard / tests), Message is the
human- and model-readable explanation the tool returns verbatim so the
model can react (size down, pick a different name, or hold).
*/
type CapError struct {
	Code    string
	Message string
}

func (e *CapError) Error() string { return e.Message }

func capErr(code, format string, a ...any) *CapError {
	return &CapError{Code: code, Message: fmt.Sprintf(format, a...)}
}

/*
DrawdownHalted reports whether the drawdown breaker has tripped: account
equity is at or below high-water times (1 - DrawdownHaltPct). When true,
the tool layer refuses every buy and allows only de-risking moves. A
zero/negative high-water mark (fresh account, no history) never trips.
*/
func (c Caps) DrawdownHalted(s Snapshot) bool {
	if s.HighWaterMark <= 0 {
		return false
	}
	floor := s.HighWaterMark * (1 - c.DrawdownHaltPct)
	return s.Equity <= floor+floatTol
}

/*
underlyingExposure sums the current market value of every position keyed
to the given underlying (equity shares plus any option premium on that
name). This is what the concentration cap measures a new buy against.
*/
func underlyingExposure(s Snapshot, underlying string) float64 {
	u := strings.ToUpper(strings.TrimSpace(underlying))
	var total float64
	for _, p := range s.Positions {
		if strings.ToUpper(strings.TrimSpace(p.Underlying)) == u {
			total += p.MarkValue
		}
	}
	return total
}

// optionsSleeveValue sums the current market value of every option
// position, for the leverage-sleeve cap.
func optionsSleeveValue(s Snapshot) float64 {
	var total float64
	for _, p := range s.Positions {
		if p.AssetType == AssetOption {
			total += p.MarkValue
		}
	}
	return total
}

/*
CheckBuy runs every buy-side cap against a proposed move and the current
snapshot, returning the first violation (or nil if the move is allowed).
Order of checks is intentional: the cheapest, most decisive gates first
(drawdown breaker, then liquidity, then the dollar caps) so the model gets
the most actionable refusal.

The caller must have already filled move.Liquidity from live market data
and move.Notional with the committed dollar amount. CheckBuy mutates
nothing.
*/
func (c Caps) CheckBuy(s Snapshot, m Move) *CapError {
	if m.Action != ActionBuyEquity && m.Action != ActionBuyOption {
		return capErr("not_a_buy", "CheckBuy called with non-buy action %q", m.Action)
	}
	if m.Notional <= 0 {
		return capErr("bad_notional", "order notional must be > 0, got %.2f", m.Notional)
	}

	// 1. Drawdown breaker: no new risk while halted.
	if c.DrawdownHalted(s) {
		return capErr("drawdown_halt",
			"new buys are halted: account equity $%.2f is at or below the drawdown floor (high-water $%.2f minus %.0f%%). Only de-risking moves are allowed.",
			s.Equity, s.HighWaterMark, c.DrawdownHaltPct*100)
	}

	// 2. Liquidity floor.
	if le := c.checkLiquidity(m); le != nil {
		return le
	}

	// 3. Settled-cash rule (cash account: cannot spend unsettled proceeds).
	if m.Notional > s.SettledCash+floatTol {
		return capErr("settled_cash",
			"order notional $%.2f exceeds settled cash $%.2f. Unsettled proceeds cannot be redeployed until they settle (T+1).",
			m.Notional, s.SettledCash)
	}

	// 4. Daily new-deployment budget.
	if s.DeployedThisSession+m.Notional > s.DeploymentBudget+floatTol {
		return capErr("daily_deployment",
			"order notional $%.2f would push session deployment to $%.2f, over the daily budget $%.2f (already deployed $%.2f). Build the position over more sessions or size down.",
			m.Notional, s.DeployedThisSession+m.Notional, s.DeploymentBudget, s.DeployedThisSession)
	}

	// 5. Per-order notional cap.
	perOrderCap := s.Equity * c.MaxPerOrderPct
	if m.Notional > perOrderCap+floatTol {
		return capErr("per_order",
			"order notional $%.2f exceeds the per-order cap $%.2f (%.0f%% of $%.2f equity). Size down.",
			m.Notional, perOrderCap, c.MaxPerOrderPct*100, s.Equity)
	}

	// 6. Concentration cap (existing exposure to this underlying plus the new buy).
	concentrationCap := s.Equity * c.MaxPerUnderlyingPct
	projected := underlyingExposure(s, m.Underlying) + m.Notional
	if projected > concentrationCap+floatTol {
		return capErr("concentration",
			"this buy would put $%.2f of exposure on %s, over the per-name cap $%.2f (%.0f%% of $%.2f equity). Trim or pick a different name.",
			projected, strings.ToUpper(m.Underlying), concentrationCap, c.MaxPerUnderlyingPct*100, s.Equity)
	}

	// 7. Options leverage sleeve cap (options buys only).
	if m.AssetType == AssetOption {
		sleeveCap := s.Equity * c.MaxOptionsSleevePct
		projectedSleeve := optionsSleeveValue(s) + m.Notional
		if projectedSleeve > sleeveCap+floatTol {
			return capErr("options_sleeve",
				"this buy would put $%.2f of option premium at risk, over the options-sleeve cap $%.2f (%.0f%% of $%.2f equity). Use equity for this exposure or size down.",
				projectedSleeve, sleeveCap, c.MaxOptionsSleevePct*100, s.Equity)
		}
	}

	return nil
}

/*
checkLiquidity enforces the tradability floor on a buy. Stock price and
market-cap gates apply to every buy (the underlying must be liquid);
OI/volume gates apply to option buys only.
*/
func (c Caps) checkLiquidity(m Move) *CapError {
	l := m.Liquidity
	if l.StockPrice < c.MinStockPrice {
		return capErr("liquidity_price",
			"%s underlying at $%.2f is below the $%.2f minimum stock price.",
			strings.ToUpper(m.Underlying), l.StockPrice, c.MinStockPrice)
	}
	if l.UnderlyingMktCap < c.MinUnderlyingMktCap {
		return capErr("liquidity_mktcap",
			"%s market cap $%.0f is below the $%.0f minimum.",
			strings.ToUpper(m.Underlying), l.UnderlyingMktCap, c.MinUnderlyingMktCap)
	}
	if l.SpreadPct > c.MaxSpreadPct+floatTol {
		return capErr("liquidity_spread",
			"quoted spread %.1f%% is wider than the %.1f%% maximum.",
			l.SpreadPct*100, c.MaxSpreadPct*100)
	}
	if m.AssetType == AssetOption {
		if l.OptionOpenInterest < c.MinOptionOpenInterest {
			return capErr("liquidity_oi",
				"option open interest %d is below the %d minimum.",
				l.OptionOpenInterest, c.MinOptionOpenInterest)
		}
		if l.OptionVolume < c.MinOptionVolume {
			return capErr("liquidity_volume",
				"option day volume %d is below the %d minimum.",
				l.OptionVolume, c.MinOptionVolume)
		}
	}
	return nil
}

/*
CheckSell validates a de-risking move: the position must exist and the
quantity to sell cannot exceed what is held. Sells are never blocked by the
risk caps (reducing exposure is always allowed, including while the
drawdown breaker is tripped). Returns nil if the sell is valid.
*/
func (c Caps) CheckSell(s Snapshot, m Move) *CapError {
	if m.Action != ActionSellEquity && m.Action != ActionSellOption {
		return capErr("not_a_sell", "CheckSell called with non-sell action %q", m.Action)
	}
	if m.Quantity <= 0 {
		return capErr("bad_quantity", "sell quantity must be > 0, got %.4g", m.Quantity)
	}
	want := strings.ToUpper(strings.TrimSpace(m.Symbol))
	for _, p := range s.Positions {
		if strings.ToUpper(strings.TrimSpace(p.Symbol)) != want {
			continue
		}
		if m.Quantity > p.Quantity+floatTol {
			return capErr("oversell",
				"cannot sell %.4g of %s, only %.4g held.",
				m.Quantity, want, p.Quantity)
		}
		return nil
	}
	return capErr("no_position", "no open position in %s to sell.", want)
}

/*
HeldOptionsBelowDTE returns the option positions whose DTE is under the
MinHeldOptionDTE floor, in input order. The intraday risk cron uses this to
force a close or roll before the overnight theta/gamma cliff. Pure read.
*/
func (c Caps) HeldOptionsBelowDTE(s Snapshot) []Position {
	var out []Position
	for _, p := range s.Positions {
		if p.AssetType == AssetOption && p.DTE < c.MinHeldOptionDTE {
			out = append(out, p)
		}
	}
	return out
}
