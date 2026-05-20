// Package calendar holds the NYSE/NASDAQ trading calendar and the
// market-hours math the trading server uses to gate every market-side
// cron + endpoint. The calendar lives here (vs. inlined in
// cmd/scanner/main.go) so the streaming hub + SSE / market-status
// handlers can ask the same questions ("is the market open right
// now?", "when does it next open / close?") without re-implementing
// the holiday list.
//
// Validated against NYSE's official 2025-2027 schedule at
// nyse.com/markets/hours-calendars. Update yearly each Q4 from the
// new published schedule.
package calendar

import "time"

// US Market Holidays (NYSE/NASDAQ fully closed).
var Holidays = map[string]string{
	"2025-01-01": "New Year's Day",
	"2025-01-20": "MLK Day",
	"2025-02-17": "Presidents Day",
	"2025-04-18": "Good Friday",
	"2025-05-26": "Memorial Day",
	"2025-06-19": "Juneteenth",
	"2025-07-04": "Independence Day",
	"2025-09-01": "Labor Day",
	"2025-11-27": "Thanksgiving",
	"2025-12-25": "Christmas",
	"2026-01-01": "New Year's Day",
	"2026-01-19": "MLK Day",
	"2026-02-16": "Presidents Day",
	"2026-04-03": "Good Friday",
	"2026-05-25": "Memorial Day",
	"2026-06-19": "Juneteenth",
	"2026-09-07": "Labor Day",
	"2026-11-26": "Thanksgiving",
	"2026-12-25": "Christmas",
	// 2027 NYSE schedule.
	"2027-01-01": "New Year's Day",
	"2027-01-18": "MLK Day",
	"2027-02-15": "Presidents Day",
	"2027-03-26": "Good Friday",
	"2027-05-31": "Memorial Day",
	"2027-06-18": "Juneteenth (Observed)",
	"2027-09-06": "Labor Day",
	"2027-11-25": "Thanksgiving",
}

/*
HalfDays are 1:00 PM ET early-close trading days. Observed July 4 +
Christmas Eve + day after Thanksgiving are the standard NYSE half-day
set; precise dates vary by which weekday the actual holiday lands on.
*/
var HalfDays = map[string]string{
	"2025-11-28": "Day after Thanksgiving",
	"2025-12-24": "Christmas Eve",
	"2026-07-03": "Independence Day (Observed)",
	"2026-11-27": "Day after Thanksgiving",
	"2026-12-24": "Christmas Eve",
	"2027-07-05": "Independence Day (Observed)",
	"2027-11-26": "Day after Thanksgiving",
	"2027-12-24": "Christmas Day (Observed)",
}

// ETLocation is America/New_York. Cached at package init since
// time.LoadLocation panics on a bad tz database and we'd rather fail
// fast at boot than at the first cron tick.
var ETLocation *time.Location

func init() {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic("calendar: time.LoadLocation(\"America/New_York\") failed: " + err.Error())
	}
	ETLocation = loc
}

// IsHoliday returns the holiday name for a given YYYY-MM-DD in ET,
// or "" if not a holiday.
func IsHoliday(dateET string) string {
	return Holidays[dateET]
}

// IsHalfDay returns the half-day name for a given YYYY-MM-DD in ET,
// or "" if not a half-day.
func IsHalfDay(dateET string) string {
	return HalfDays[dateET]
}

/*
IsTradingDay returns true when dateET (YYYY-MM-DD) is a weekday and
not a full-closure holiday. Half-days ARE trading days (with an early
close); this function returns true for them. Use IsHalfDay separately
to distinguish 13:00 vs 16:00 close.
*/
func IsTradingDay(dateET string) bool {
	t, err := time.ParseInLocation("2006-01-02", dateET, ETLocation)
	if err != nil {
		return false
	}
	wd := t.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	if _, ok := Holidays[dateET]; ok {
		return false
	}
	return true
}

/*
Status describes the market's current state at a given moment. open
is true during the 9:30-16:00 ET (or 9:30-13:00 ET on half-days)
window of a trading day. NextOpen / NextClose are populated regardless
so the dashboard can render an honest countdown in either state.
Session is a coarse label: "premarket" (trading day, before 9:30),
"open", "afterhours" (trading day, after close), or "closed"
(weekend / holiday).
*/
type Status struct {
	Open      bool
	NextOpen  time.Time
	NextClose time.Time
	Session   string
}

// CloseMinute returns the close-of-day minute-of-day in ET: 960 (16:00)
// for a normal day, 780 (13:00) for a half-day. -1 if not a trading day.
func CloseMinute(dateET string) int {
	if !IsTradingDay(dateET) {
		return -1
	}
	if _, ok := HalfDays[dateET]; ok {
		return 13 * 60
	}
	return 16 * 60
}

const openMinute = 9*60 + 30

/*
CurrentStatus returns the live market status at the given instant
(typically time.Now()). The result is computed in ET regardless of the
input's timezone.
*/
func CurrentStatus(now time.Time) Status {
	et := now.In(ETLocation)
	today := et.Format("2006-01-02")
	mod := et.Hour()*60 + et.Minute()

	var s Status
	if IsTradingDay(today) {
		close := CloseMinute(today)
		if mod >= openMinute && mod < close {
			s.Open = true
			s.Session = "open"
			s.NextOpen = nextOpen(et)
			s.NextClose = atMinute(et, close)
			return s
		}
		if mod < openMinute {
			s.Session = "premarket"
			s.NextOpen = atMinute(et, openMinute)
			s.NextClose = atMinute(et, close)
			return s
		}
		// After close on a trading day.
		s.Session = "afterhours"
	} else {
		s.Session = "closed"
	}
	s.NextOpen = nextOpen(et)
	if !s.NextOpen.IsZero() {
		nextDay := s.NextOpen.Format("2006-01-02")
		s.NextClose = atMinute(s.NextOpen, CloseMinute(nextDay))
	}
	return s
}

func atMinute(et time.Time, mod int) time.Time {
	return time.Date(et.Year(), et.Month(), et.Day(), mod/60, mod%60, 0, 0, ETLocation)
}

func nextOpen(et time.Time) time.Time {
	today := et.Format("2006-01-02")
	mod := et.Hour()*60 + et.Minute()
	if IsTradingDay(today) && mod < openMinute {
		return atMinute(et, openMinute)
	}
	// Walk forward day-by-day until a trading day. Bounded to 10 days
	// (the longest plausible holiday gap is Christmas + weekend).
	for i := 1; i <= 10; i++ {
		d := et.AddDate(0, 0, i)
		ds := d.Format("2006-01-02")
		if IsTradingDay(ds) {
			return time.Date(d.Year(), d.Month(), d.Day(), openMinute/60, openMinute%60, 0, 0, ETLocation)
		}
	}
	return time.Time{}
}
