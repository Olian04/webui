package webui

import (
	"math"
	"time"
)

// layouts are the forms of an ISO 8601 moment a Datetime is read in. A moment with
// no zone is taken as UTC, and a date alone has no time of day to show.
var datetimeLayouts = []struct {
	layout   string
	dateOnly bool
}{
	{time.RFC3339Nano, false},
	{"2006-01-02T15:04:05", false},
	{"2006-01-02T15:04", false},
	{"2006-01-02 15:04:05", false},
	{"2006-01-02", true},
}

// momentLayout and dayLayout are how a moment and a date alone are shown, in UTC.
const (
	momentLayout = "2006-01-02 15:04"
	dayLayout    = "2006-01-02"
)

// readDatetime parses an ISO 8601 string. ok is false when it is not one, and the
// caller then shows the text as it is.
func readDatetime(s string) (t time.Time, dateOnly, ok bool) {
	for _, l := range datetimeLayouts {
		if t, err := time.Parse(l.layout, s); err == nil {
			return t.UTC(), l.dateOnly, true
		}
	}
	return time.Time{}, false, false
}

// showMoment is a moment as a table and a form show it.
func showMoment(t time.Time, dateOnly bool) string {
	if dateOnly {
		return t.UTC().Format(dayLayout)
	}
	return t.UTC().Format(momentLayout)
}

// datetimeText and datetimeNum are what a Datetime is to the table: the text it
// shows, and the number it sorts and filters by. Text that is not a moment is shown
// as written and sorts first.
func datetimeText(s string) string {
	if t, dateOnly, ok := readDatetime(s); ok {
		return showMoment(t, dateOnly)
	}
	return s
}

func datetimeNum(s string) float64 {
	if t, _, ok := readDatetime(s); ok {
		return float64(t.Unix())
	}
	return math.Inf(-1)
}

// timestampText is a Unix time in seconds as shown. Zero is "not set", and shows as
// nothing, as an empty Datetime does.
func timestampText(seconds int) string {
	if seconds == 0 {
		return ""
	}
	return showMoment(time.Unix(int64(seconds), 0), false)
}
