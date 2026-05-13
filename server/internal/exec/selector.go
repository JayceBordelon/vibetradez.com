package exec

import "vibetradez.com/internal/trades"

/*
MaxContractPremium is the per-contract premium ceiling for the
ranks-2-and-3 basket picks. Premium is quoted per-share, options are
100 shares, so $5 premium = $500 of capital exposure on a single
contract. Rank-1 has its own higher cap (MaxRank1ContractPremium)
because the operator prioritizes filling rank-1 every day even when
the contract is pricier than would be acceptable for a tail pick.

Paper mode honors both caps so paper P&L tracking mirrors what live
would have actually done.
*/
const MaxContractPremium = 5.00

/*
MaxRank1ContractPremium is the elevated per-contract premium ceiling
for the rank-1 pick only. $10 per share = $1,000 of capital exposure
on a single contract. The operator's stated goal is "always fill
rank-1, within reason"; this caps "within reason" at $1k/contract so
a Claude mispricing or a runaway-spread morning still has a bounded
worst case. Ranks 2 and 3 keep the tighter MaxContractPremium cap so
the daily failure mode doesn't change for the tail of the basket.
*/
const MaxRank1ContractPremium = 10.00

/*
MaxDailyBasketUSD is the hard daily cap on cumulative open exposure
across the ranks-2-and-3 portion of the basket. Rank-1 is EXEMPT
from this cap — it's bounded only by MaxRank1ContractPremium and
Schwab's live available cash — so a fully-priced rank-1 + a full
$500 tail can land on the same morning without one starving the
other. The intent here is "guarantee rank-1; cap the tail".
*/
const MaxDailyBasketUSD = 500.00

/*
MaxBasketRank caps which ranks are eligible for the basket. Only
picks ranked 1..MaxBasketRank are considered, in rank order. Rank-4
and below are ignored even if budget remains, because the executor's
fairness story is "execute Claude's top conviction picks" — the tail
is for the published email, not for capital.
*/
const MaxBasketRank = 3

/*
PerContractPremiumCap returns the per-share premium ceiling for the
given rank. Rank-1 uses MaxRank1ContractPremium; every other rank
uses MaxContractPremium. Exposed so the executor's reprice path can
recompute the right ceiling for a still-WORKING position without
re-implementing the rank → cap mapping.
*/
func PerContractPremiumCap(rank int) float64 {
	if rank == 1 {
		return MaxRank1ContractPremium
	}
	return MaxContractPremium
}

/*
QualifyingPicks returns the basket of trades to auto-execute today,
in submission order (rank-ascending). The selector walks ranks
1..MaxBasketRank greedily:

  - reject any pick whose modeled premium exceeds its rank-aware cap
    (rank-1 → MaxRank1ContractPremium, ranks 2-3 → MaxContractPremium)
  - rank-1 is included whenever its premium fits its own cap and is
    positive; it does NOT consume the MaxDailyBasketUSD pool
  - ranks 2 and 3 share the MaxDailyBasketUSD pool — skip-and-
    continue when one pick is too big, so a $250 rank-2 doesn't
    blackball a $90 rank-3

Returns an empty slice when nothing qualifies. Cost accounting here
uses Claude's modeled premium; the service does a second cash-
availability check at order-place time against Schwab's reported
availableFundsNonMarginableTrade and the actual ask-derived limit
price, so a quote drift between morning analysis and 9:25 cron
submission can still skip a contract.

Score is intentionally NOT a gate. Rank order is the only signal.
*/
func QualifyingPicks(picks []trades.Trade) []trades.Trade {
	if len(picks) == 0 {
		return nil
	}
	byRank := make(map[int]*trades.Trade, MaxBasketRank)
	for i := range picks {
		t := &picks[i]
		if t.Rank < 1 || t.Rank > MaxBasketRank {
			continue
		}
		byRank[t.Rank] = t
	}
	out := make([]trades.Trade, 0, MaxBasketRank)
	remainingBasketUSD := MaxDailyBasketUSD
	for r := 1; r <= MaxBasketRank; r++ {
		t := byRank[r]
		if t == nil {
			continue
		}
		cap := PerContractPremiumCap(r)
		if t.EstimatedPrice <= 0 || t.EstimatedPrice > cap {
			continue
		}
		costUSD := t.EstimatedPrice * 100
		if r == 1 {
			out = append(out, *t)
			continue
		}
		if costUSD > remainingBasketUSD {
			continue
		}
		out = append(out, *t)
		remainingBasketUSD -= costUSD
	}
	return out
}
