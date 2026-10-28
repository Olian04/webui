// Package calendar is the date arithmetic of a month on a page: which weeks show it, which days they hold, the ISO number of a week, and the
// name of a month in an address. Every day is a moment at midnight UTC, since a calendar here is of dates and not of moments, and an
// app's own time zone is its own to apply to what it keeps.
package calendar

import (
	"fmt"
	"time"
)

// The years a month in an address may be in, so that a hand-edited address cannot ask a source for the year 9999.
const (
	minYear = 1900
	maxYear = 2200
)

// Month is a month of a year.
type Month struct {
	Year  int
	Month time.Month
}

// Of is the month that t is in, in UTC.
func Of(t time.Time) Month {
	t = t.UTC()
	return Month{Year: t.Year(), Month: t.Month()}
}

// Parse reads "2026-09", the way a month is written in an address. A month that is not one, or is outside the years that a calendar shows, is not read.
func Parse(s string) (Month, bool) {
	t, err := time.Parse("2006-01", s)
	if err != nil || len(s) != 7 || t.Year() < minYear || t.Year() > maxYear {
		return Month{}, false
	}
	return Of(t), true
}

// String is the month as an address writes it: "2026-09".
func (m Month) String() string { return fmt.Sprintf("%04d-%02d", m.Year, int(m.Month)) }

// Title is the month as a heading says it: "September 2026".
func (m Month) Title() string { return fmt.Sprintf("%s %d", m.Month, m.Year) }

// First is the first day of the month.
func (m Month) First() time.Time { return time.Date(m.Year, m.Month, 1, 0, 0, 0, 0, time.UTC) }

// Prev is the month before, and Next the month after, across a year's end.
func (m Month) Prev() Month { return Of(m.First().AddDate(0, -1, 0)) }
func (m Month) Next() Month { return Of(m.First().AddDate(0, 1, 0)) }

// InRange is whether a calendar shows the month: within the years that one does.
func (m Month) InRange() bool { return m.Year >= minYear && m.Year <= maxYear }

// Weeks are the weeks that show the month: whole weeks, from the one that holds its first day to the one that holds its last, so four to six of
// them. A week begins on Monday, as ISO 8601 has it, or on Sunday.
func Weeks(m Month, sundayFirst bool) [][7]time.Time {
	first := m.First()
	start := first.AddDate(0, 0, -offset(first.Weekday(), sundayFirst))
	last := first.AddDate(0, 1, -1)
	var weeks [][7]time.Time
	for week := start; !week.After(last); week = week.AddDate(0, 0, 7) {
		var days [7]time.Time
		for i := range days {
			days[i] = week.AddDate(0, 0, i)
		}
		weeks = append(weeks, days)
	}
	return weeks
}

// Span is the days that Weeks has, as a half-open range of moments: from the midnight that begins the first, to the midnight that ends the last.
func Span(m Month, sundayFirst bool) (from, to time.Time) {
	weeks := Weeks(m, sundayFirst)
	return weeks[0][0], weeks[len(weeks)-1][6].AddDate(0, 0, 1)
}

// offset is how many days a weekday is after the first day of its week.
func offset(day time.Weekday, sundayFirst bool) int {
	if sundayFirst {
		return int(day)
	}
	return (int(day) + 6) % 7
}

// Weekday is a column's heading: the two letters it is written with, and its whole name, for a reader who is told the abbreviation by it.
type Weekday struct{ Abbr, Name string }

// Weekdays are the headings of the seven columns, from the first day of the week.
func Weekdays(sundayFirst bool) []Weekday {
	days := []Weekday{{"Mo", "Monday"}, {"Tu", "Tuesday"}, {"We", "Wednesday"}, {"Th", "Thursday"}, {"Fr", "Friday"}, {"Sa", "Saturday"}, {"Su", "Sunday"}}
	if sundayFirst {
		return append([]Weekday{days[6]}, days[:6]...)
	}
	return days
}

// WeekNumber is the ISO 8601 number of the week that a day is in, in which the first week is the one with the year's first Thursday: the first days of January may be
// in the last week of the year before, and the last days of December in the first of the next.
func WeekNumber(day time.Time) int {
	_, week := day.ISOWeek()
	return week
}

// Key is a day as a date, "2026-09-06", which is how a day is named in an address and how the items of a day are found.
func Key(day time.Time) string { return day.UTC().Format("2006-01-02") }

// Today is the day that t is in, at its midnight.
func Today(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
