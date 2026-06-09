package portfoliowire

import (
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
