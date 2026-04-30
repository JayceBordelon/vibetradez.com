package exec

import "vibetradez.com/internal/trades"

/*
MaxContractPremium is the hard upper bound on a single contract's
premium for live-mode auto-execution. The cap is enforced uniformly
to keep the live failure mode contained: worst-case daily loss is
MaxContractPremium * 100 = $500 per filled contract. Premium quoted
in this codebase is per-share, options are 100 shares, so $5 premium
= $500 of capital exposure. Adjust both this constant and the prompt
language together if the cap ever changes.

Paper mode also enforces the cap so paper P&L tracking mirrors what
live would have actually done.
*/
const MaxContractPremium = 5.00

/*
QualifyingPick returns the trade that should be auto-executed today.
The qualification rule is intentionally simple now that the email
confirmation step is gone:
  - Claude ranked the trade #1 for the day
  - the contract premium is at or below MaxContractPremium

Claude's conviction score is intentionally NOT a gate, the rank-1
slot is the only signal. If no rank-1 trade meets the cap the day is
skipped (no order placed).
*/
func QualifyingPick(picks []trades.Trade) (*trades.Trade, bool) {
	for i := range picks {
		t := &picks[i]
		if t.Rank != 1 {
			continue
		}
		if t.EstimatedPrice > MaxContractPremium {
			continue
		}
		return t, true
	}
	return nil, false
}
