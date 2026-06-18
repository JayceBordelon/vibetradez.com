package portfoliowire

import (
	"time"

	"vibetradez.com/internal/calendar"
)

/*
daysToExpiration returns the calendar-day delta from today to the given
YYYY-MM-DD expiration, anchored at midnight in ET on both ends. ET (not
UTC) is the whole system's trading clock, so this matches the DTE the
holdings API reports (server.dteFromExpiration); anchoring in UTC made the
agent's DTE one day short whenever a session ran after ~20:00 ET (UTC had
already rolled to the next date). Negative when already past.
*/
func daysToExpiration(expiration string) int {
	exp, err := time.ParseInLocation("2006-01-02", expiration, calendar.ETLocation)
	if err != nil {
		return 0
	}
	now := time.Now().In(calendar.ETLocation)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, calendar.ETLocation)
	return int(exp.Sub(today).Hours() / 24)
}
