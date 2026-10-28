package ir

import (
	"context"
	"time"
)

// Calendar is a leaf that shows a month as a grid of days, each with a mark when it has items and a link when it leads somewhere. The visible month is in the
// address, and is changed by links. It only reads.
type Calendar struct {
	At Addr

	// ID names this leaf's view state in the address: "<ID>.month" is the month that is shown.
	ID string

	Title string
	Desc  string

	// Load returns the items whose dates are in [from, to): the days that the grid shows, the days of the weeks round the month too.
	Load func(ctx context.Context, from, to time.Time) ([]any, error)

	// Date is the moment of an item, whose UTC day it is on.
	Date func(model any) time.Time

	// Open is where a day with items leads, built from a CalendarDay; nil for none.
	Open *Link

	// SundayFirst begins a week on Sunday and not on Monday, and WeekNumbers adds a column of ISO week numbers, which are of weeks that begin on Monday.
	SundayFirst, WeekNumbers bool

	// Today is the moment that is now, or nil for the clock.
	Today func() time.Time
}

// CalendarDay is what a day's link is built from: the day, at midnight UTC, and how many items are on it.
type CalendarDay struct {
	Date  time.Time
	Items int
}

// Addr reports the node's position in the body tree.
func (c *Calendar) Addr() Addr { return c.At }
