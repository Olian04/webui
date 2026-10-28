package calendar

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestParseReadsAMonthAsAnAddressWritesIt(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]Month{
		"2026-09": {2026, time.September}, "2024-02": {2024, time.February}, "1900-01": {1900, time.January}, "2200-12": {2200, time.December},
	} {
		got, ok := Parse(in)
		if !ok || got != want {
			t.Errorf("Parse(%q) = %v, %v; want %v", in, got, ok, want)
		}
		if got.String() != in {
			t.Errorf("%v writes as %q, want %q", got, got.String(), in)
		}
	}
	// What is not a month, or is outside the years a calendar shows, is not read, so that an address cannot ask for the year 9999.
	for _, bad := range []string{"", "2026", "2026-9", "2026-13", "2026-00", "26-09", "2026-09-01", "abcd-ef", " 2026-09", "1899-12", "2201-01", "9999-01", "2026/09", "2026-09\n"} {
		if got, ok := Parse(bad); ok {
			t.Errorf("Parse(%q) was read as %v", bad, got)
		}
	}
}

func TestMonthsAreHeadedAndStepAcrossAYearsEnd(t *testing.T) {
	t.Parallel()

	m := Month{2026, time.September}
	if m.Title() != "September 2026" {
		t.Errorf("title %q", m.Title())
	}
	if got := (Month{2026, time.January}).Prev(); got != (Month{2025, time.December}) {
		t.Errorf("before January 2026 is %v", got)
	}
	if got := (Month{2025, time.December}).Next(); got != (Month{2026, time.January}) {
		t.Errorf("after December 2025 is %v", got)
	}
	if got := (Month{2024, time.January}).Next(); got != (Month{2024, time.February}) {
		t.Errorf("after January 2024 is %v", got)
	}
	// The thirty-first of a month does not skip a month that has fewer days.
	if got := Of(day(2026, time.January, 31)).Next(); got != (Month{2026, time.February}) {
		t.Errorf("after the month of 31 January is %v", got)
	}
	if got := Of(day(2026, time.March, 31)).Prev(); got != (Month{2026, time.February}) {
		t.Errorf("before the month of 31 March is %v", got)
	}
}

// The weeks of a month are whole weeks, four to six of them, from the week that holds the first day to the week that holds the last.
func TestTheWeeksOfAMonthAreWholeWeeksFromTheFirstDaysWeekToTheLastDaysWeek(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		month       Month
		sundayFirst bool
		rows        int
		first, last time.Time
	}{
		"february 2026 begins on a sunday, so Monday weeks begin the week before": {Month{2026, time.February}, false, 5, day(2026, time.January, 26), day(2026, time.March, 1)},
		"february 2026 is four whole weeks when they begin on Sunday":             {Month{2026, time.February}, true, 4, day(2026, time.February, 1), day(2026, time.February, 28)},
		"february 2024 is a leap year, with 29 days":                              {Month{2024, time.February}, false, 5, day(2024, time.January, 29), day(2024, time.March, 3)},
		"august 2026 begins on a saturday and has 31 days: six weeks":             {Month{2026, time.August}, false, 6, day(2026, time.July, 27), day(2026, time.September, 6)},
		"august 2026 with Sunday weeks is six too":                                {Month{2026, time.August}, true, 6, day(2026, time.July, 26), day(2026, time.September, 5)},
		"september 2026 begins on a tuesday":                                      {Month{2026, time.September}, false, 5, day(2026, time.August, 31), day(2026, time.October, 4)},
		"a month across the year's end":                                           {Month{2025, time.December}, false, 5, day(2025, time.December, 1), day(2026, time.January, 4)},
		"january 2027 begins on a friday":                                         {Month{2027, time.January}, false, 5, day(2026, time.December, 28), day(2027, time.January, 31)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			weeks := Weeks(tc.month, tc.sundayFirst)
			if len(weeks) != tc.rows {
				t.Fatalf("%d weeks, want %d", len(weeks), tc.rows)
			}
			if !weeks[0][0].Equal(tc.first) || !weeks[len(weeks)-1][6].Equal(tc.last) {
				t.Errorf("from %s to %s, want %s to %s", Key(weeks[0][0]), Key(weeks[len(weeks)-1][6]), Key(tc.first), Key(tc.last))
			}
			// Every day follows the one before, and a week begins where it should.
			begins := time.Monday
			if tc.sundayFirst {
				begins = time.Sunday
			}
			var prev time.Time
			for _, week := range weeks {
				if week[0].Weekday() != begins {
					t.Errorf("a week begins on %s, want %s", week[0].Weekday(), begins)
				}
				for _, d := range week {
					if !prev.IsZero() && !d.Equal(prev.AddDate(0, 0, 1)) {
						t.Errorf("%s follows %s", Key(d), Key(prev))
					}
					prev = d
				}
			}
			// The month is all in it, once.
			var in []int
			for _, week := range weeks {
				for _, d := range week {
					if Of(d) == tc.month {
						in = append(in, d.Day())
					}
				}
			}
			if in[0] != 1 || len(in) != tc.month.First().AddDate(0, 1, -1).Day() {
				t.Errorf("the month's days in the grid are %v", in)
			}
		})
	}
}

func TestSpanIsTheGridAsAHalfOpenRangeSoTheLastDaysItemsAreInIt(t *testing.T) {
	t.Parallel()

	from, to := Span(Month{2026, time.September}, false)
	if !from.Equal(day(2026, time.August, 31)) || !to.Equal(day(2026, time.October, 5)) {
		t.Errorf("span %s to %s", Key(from), Key(to))
	}
	// A moment late on the last day shown is inside, and the midnight that ends it is not.
	last := day(2026, time.October, 4).Add(23*time.Hour + 59*time.Minute)
	if last.Before(from) || !last.Before(to) {
		t.Error("the last day's evening is outside the span")
	}
}

func TestTheDaysAreTheSameWhateverTheLocationOfTheMomentThatNamesThem(t *testing.T) {
	t.Parallel()

	// 23:30 on 30 September in Los Angeles is 06:30 on 1 October in UTC: a calendar is of UTC days.
	la := time.FixedZone("PDT", -7*3600)
	late := time.Date(2026, time.September, 30, 23, 30, 0, 0, la)
	if Key(late) != "2026-10-01" || Of(late) != (Month{2026, time.October}) || !Today(late).Equal(day(2026, time.October, 1)) {
		t.Errorf("%s, %v, %s", Key(late), Of(late), Today(late))
	}
}

func TestWeekdaysBeginWhereTheWeekDoes(t *testing.T) {
	t.Parallel()

	monday := Weekdays(false)
	sunday := Weekdays(true)
	if monday[0].Abbr != "Mo" || monday[6].Abbr != "Su" || sunday[0].Abbr != "Su" || sunday[1].Abbr != "Mo" || sunday[6].Abbr != "Sa" {
		t.Errorf("%v / %v", monday, sunday)
	}
	if monday[2].Name != "Wednesday" || sunday[3].Name != "Wednesday" {
		t.Errorf("names %v", monday)
	}
}

// The first week of a year is the one with its first Thursday, so a few days of January may be in the last week of the year before, and a few of
// December in the first of the next.
func TestWeekNumbersAreISOAroundTheNewYear(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		d    time.Time
		week int
	}{
		{day(2026, time.January, 1), 1},   // a Thursday: week 1
		{day(2025, time.December, 29), 1}, // the Monday of that week, in the year before
		{day(2025, time.December, 28), 52},
		{day(2027, time.January, 1), 53},  // a Friday, in the last week of 2026, which has 53
		{day(2027, time.January, 4), 1},   // the Monday after
		{day(2024, time.December, 31), 1}, // a Tuesday, in week 1 of 2025
		{day(2024, time.February, 29), 9}, // the leap day
		{day(2026, time.September, 6), 36},
		{day(2026, time.June, 15), 25},
	} {
		if got := WeekNumber(tc.d); got != tc.week {
			t.Errorf("%s is in week %d, want %d", Key(tc.d), got, tc.week)
		}
	}
}
