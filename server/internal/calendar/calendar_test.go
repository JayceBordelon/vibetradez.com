package calendar

import (
	"testing"
	"time"
)

func TestIsTradingDay(t *testing.T) {
	cases := []struct {
		date string
		want bool
		why  string
	}{
		{"2026-05-20", true, "regular Wednesday"},
		{"2026-05-23", false, "Saturday"},
		{"2026-05-24", false, "Sunday"},
		{"2026-05-25", false, "Memorial Day holiday"},
		{"2026-07-03", false, "Jul 4 2026 is Sat, so Independence Day observed is a FULL closure"},
		{"2026-11-26", false, "Thanksgiving holiday"},
		{"2026-11-27", true, "Day after Thanksgiving is a half-day, still a trading day"},
		{"2026-12-24", true, "Christmas Eve is a half-day, still a trading day"},
		{"2027-12-24", false, "Dec 25 2027 is Sat, so Christmas observed is a FULL closure"},
		{"2027-12-25", false, "Christmas Day (Saturday)"},
	}
	for _, tc := range cases {
		t.Run(tc.date, func(t *testing.T) {
			if got := IsTradingDay(tc.date); got != tc.want {
				t.Errorf("IsTradingDay(%s) = %v, want %v (%s)", tc.date, got, tc.want, tc.why)
			}
		})
	}
}

func TestCloseMinute(t *testing.T) {
	cases := []struct {
		date string
		want int
	}{
		{"2026-05-20", 960}, // 16:00
		{"2026-12-24", 780}, // 13:00 Christmas Eve half-day
		{"2026-07-03", -1},  // full closure (Independence Day observed, Jul 4 is Sat)
		{"2027-12-24", -1},  // full closure (Christmas observed, Dec 25 is Sat)
		{"2026-05-25", -1},  // holiday
		{"2026-05-23", -1},  // Saturday
	}
	for _, tc := range cases {
		t.Run(tc.date, func(t *testing.T) {
			if got := CloseMinute(tc.date); got != tc.want {
				t.Errorf("CloseMinute(%s) = %d, want %d", tc.date, got, tc.want)
			}
		})
	}
}

func TestCurrentStatus(t *testing.T) {
	mustTime := func(s string) time.Time {
		t.Helper()
		v, err := time.ParseInLocation("2006-01-02 15:04", s, ETLocation)
		if err != nil {
			t.Fatalf("parse %s: %v", s, err)
		}
		return v
	}

	cases := []struct {
		name        string
		now         time.Time
		wantOpen    bool
		wantSession string
	}{
		{"midday Wed", mustTime("2026-05-20 12:00"), true, "open"},
		{"9:25 ET premarket", mustTime("2026-05-20 09:25"), false, "premarket"},
		{"9:30 ET open", mustTime("2026-05-20 09:30"), true, "open"},
		{"16:00 ET close edge (closed)", mustTime("2026-05-20 16:00"), false, "afterhours"},
		{"half-day 13:00 ET close edge", mustTime("2026-12-24 13:00"), false, "afterhours"},
		{"half-day 12:30 ET still open", mustTime("2026-12-24 12:30"), true, "open"},
		{"Saturday", mustTime("2026-05-23 12:00"), false, "closed"},
		{"holiday (Memorial Day)", mustTime("2026-05-25 12:00"), false, "closed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := CurrentStatus(tc.now)
			if st.Open != tc.wantOpen {
				t.Errorf("Open: got %v, want %v", st.Open, tc.wantOpen)
			}
			if st.Session != tc.wantSession {
				t.Errorf("Session: got %q, want %q", st.Session, tc.wantSession)
			}
			if st.NextOpen.IsZero() {
				t.Errorf("NextOpen should always be set, got zero")
			}
		})
	}
}

func TestNextOpenAfterHoliday(t *testing.T) {
	// Friday afternoon before Memorial Day weekend (Memorial Day = Mon 2026-05-25).
	// Next open is Tuesday 2026-05-26 09:30 ET.
	now, _ := time.ParseInLocation("2006-01-02 15:04", "2026-05-22 16:30", ETLocation)
	st := CurrentStatus(now)
	if st.NextOpen.Format("2006-01-02 15:04") != "2026-05-26 09:30" {
		t.Errorf("NextOpen after Memorial Day weekend: got %s, want 2026-05-26 09:30", st.NextOpen.Format("2006-01-02 15:04"))
	}
}
