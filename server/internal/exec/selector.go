package exec

import "vibetradez.com/internal/trades"

/*
MaxContractPremium is the hard upper bound on a single contract's
premium for auto-execution. Quarter of the existing $200 model-prompt
cap; the lower number reflects "we're auto-trading this so the cap
must be aggressive". Worst-case daily loss = MaxContractPremium ×
MaxContracts × 100 (dollars per option contract multiplier) per
position, but with the daily cap of 1 position via UNIQUE(trade_date)
the worst-case daily loss is just MaxContractPremium × 100 = $50000
for a $500 contract... wait, premium is per share, options are 100
shares, so $500 premium = $500 × 100 = $50,000? No — premium quoted
in this codebase is the CONTRACT price already (the dashboard renders
it as "Buy: $4.20" etc.), which is typically displayed as price-per-
share but represents the per-contract premium when multiplied by 100.
We treat EstimatedPrice as the per-share premium and the contract
multiplier of 100 turns $5 premium into $500 of capital at risk. So
MaxContractPremium = 5.00 (per-share) ≡ $500 capital exposure per
contract. Adjust both this constant and the prompt language together
if the cap ever changes.
*/
const MaxContractPremium = 5.00

/*
QualifyingPick returns the trade that should be auto-executed today,
if any. The qualification rule is intentionally narrow:
  - both models picked the same ticker (same Symbol)
  - both models picked the same direction (same ContractType)
  - both models ranked it #1 (GPTRank == 1 AND ClaudeRank == 1)
  - the contract premium is at or below MaxContractPremium ($5/share = $500/contract)

If no trade meets all four criteria, the function returns (nil, false)
and the day is skipped — no email, no order, no DB row.
*/
func QualifyingPick(merged []trades.Trade) (*trades.Trade, bool) {
	for i := range merged {
		t := &merged[i]
		if !t.PickedByOpenAI || !t.PickedByClaude {
			continue
		}
		if t.GPTRank != 1 || t.ClaudeRank != 1 {
			continue
		}
		if t.EstimatedPrice > MaxContractPremium {
			continue
		}
		return t, true
	}
	return nil, false
}
