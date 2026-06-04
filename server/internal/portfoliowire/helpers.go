package portfoliowire

import (
	"log"
	"sync"
	"time"
)

/*
daysToExpiration returns the calendar-day delta from today (UTC) to the
given YYYY-MM-DD expiration, anchored at midnight UTC on both ends so DST
transitions don't perturb the count. Negative when already past. Mirrors
the helper in internal/execagent so the DTE the portfolio agent sees lines
up with the rest of the system.
*/
func daysToExpiration(expiration string) int {
	exp, err := time.Parse("2006-01-02", expiration)
	if err != nil {
		return 0
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	exp = exp.UTC().Truncate(24 * time.Hour)
	return int(exp.Sub(today).Hours() / 24)
}

var marketCapUnavailableOnce sync.Once

// logMarketCapUnavailableOnce emits a single warning when the Schwab
// /instruments fundamentals call can't supply a market cap, so the
// fail-open behavior (the buy is allowed to pass the market-cap gate) is
// visible in logs without spamming a line per buy.
func logMarketCapUnavailableOnce() {
	marketCapUnavailableOnce.Do(func() {
		log.Println("portfoliowire: NOTE Schwab /instruments market cap was unavailable for at least one buy; the market-cap floor failed open for it (price, spread, and option OI/volume floors still apply).")
	})
}
