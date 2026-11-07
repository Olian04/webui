package webui

import (
	"time"

	"github.com/Olian04/webui/internal/ir"
)

// Time projects a time.Time field of M as a date and time. It is shown in UTC as "2026-10-09 11:27" (a date alone as "2026-10-09"), and a table sorts and
// filters it as a moment and not as the text: the filter takes a start and an end, in UTC. The zero time is "not set": it shows as nothing, sorts
// first and is in no range, and clearing the picker stores it. A moment that may be absent is an [OptionalTime].
//
// Without Store it is read-only. With Store a form shows a date and time picker, in UTC, and Store is handed the choice in UTC, or the zero time when the
// picker is cleared (which Required refuses). A model that holds a moment in another form, an ISO 8601 string or Unix seconds, reads and writes it in
// Load and Store: time.Parse and time.Unix, and Format and Unix.
type Time[M any] struct {
	// Label is the column header and the field's label. In a table it also names
	// the column in the address and in a [PagedQuery].
	Label string

	// Load reads the moment from the model. The zero time is not set.
	Load func(M) time.Time

	// Store writes the chosen moment to the model, in UTC (a day at midnight, with DateOnly), or the zero time when the picker was cleared. Without it
	// the field is read-only.
	Store func(*M, time.Time)

	// Required refuses an empty picker. The zero time is a legitimate empty value of its own, so a Time may be left empty, and is shown blank, unless it
	// is Required. It needs a Store.
	Required bool

	// Format says how the moment reads where it is only read, as [Int.Format] does for a number: a relative "3 h ago", or another layout than the
	// default. It is handed the moment in UTC, and a date alone as midnight. The column still sorts and filters by the moment, and its <time>
	// element and the export still hold it. The zero time is not handed to Format: it shows as nothing.
	Format func(time.Time) string

	// DateOnly makes the field a date: the input picks a day, with no time of day, and it is shown and filtered as one. A day is midnight UTC,
	// so what is stored is that moment. A moment that Load returns with a time of day is shown as the day it is on.
	DateOnly bool

	// Help is a line or two that tells the visitor what the field is for, or what to put in it, where its label and
	// its rules cannot: it is in the popover beside the label that lists the rules, under a heading of its own and above
	// them, on hover and on keyboard focus, and is part of the icon's accessible name. It is shown where the field is, in a
	// form or a toolbar, read-only too. In a table it is not shown, as a placeholder is not. It is text, not markup.
	Help string

	// NoSort leaves the sort link off the column's header, in a table: it cannot be sorted by,
	// and a sort by it in the address is ignored. It does nothing in a form.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table: it cannot be filtered by,
	// and a filter on it in the address is ignored. It does nothing in a form.
	NoFilter bool
}

func (a Time[M]) label() string { return a.Label }

func (a Time[M]) meta() accessorMeta {
	return accessorMeta{NoSort: a.NoSort, NoFilter: a.NoFilter, Help: a.Help}
}

// OptionalTime projects a *time.Time field of M: a moment that may be absent, as [OptionalInt] is a whole number that may be. It reads, sorts, filters and
// is picked as a [Time] is, with nil for none, and a picker that is cleared stores nil. A nil moment and the zero time are both "not set". Absent is
// not a moment, so it is not Required: an accessor that may be absent is not wanted.
type OptionalTime[M any] struct {
	// Label is the column header and the field's label. In a table it also names
	// the column in the address and in a [PagedQuery].
	Label string

	// Load reads the moment from the model: nil when there is none.
	Load func(M) *time.Time

	// Store writes the chosen moment to the model, in UTC (a day at midnight, with DateOnly): nil when the picker was cleared. Without it the
	// field is read-only.
	Store func(*M, *time.Time)

	// Format says how the moment reads where it is only read, as [Time.Format] does. It is not handed an absent moment.
	Format func(time.Time) string

	// DateOnly makes the field a date, as it does a [Time].
	DateOnly bool

	// Help is a line or two that tells the visitor what the field is for, or what to put in it, where its label and
	// its rules cannot: it is in the popover beside the label that lists the rules, under a heading of its own and above
	// them, on hover and on keyboard focus, and is part of the icon's accessible name. It is shown where the field is, in a
	// form or a toolbar, read-only too. In a table it is not shown, as a placeholder is not. It is text, not markup.
	Help string

	// NoSort leaves the sort link off the column's header, in a table: it cannot be sorted by,
	// and a sort by it in the address is ignored. It does nothing in a form.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table: it cannot be filtered by,
	// and a filter on it in the address is ignored. It does nothing in a form.
	NoFilter bool
}

func (a OptionalTime[M]) label() string { return a.Label }

func (a OptionalTime[M]) meta() accessorMeta {
	return accessorMeta{NoSort: a.NoSort, NoFilter: a.NoFilter, Help: a.Help}
}

// ASSERT: Time and OptionalTime implement Accessor
var (
	_ Accessor[struct{}] = Time[struct{}]{}
	_ Accessor[struct{}] = OptionalTime[struct{}]{}
)

// validateTime checks what a Time needs besides what every accessor does: Required needs a Store, since an input that cannot be filled in cannot be required.
func validateTime[M any](v *viewValidator, at string, a Time[M]) {
	if a.Required && a.Store == nil {
		v.add(at+" is Required but has no Store", "Required is about a submission, and a read-only field has none: remove it, or add a Store.")
	}
}

// pickedTime is what a date and time picker submitted, as the moment to store: in UTC, a day at its midnight with dateOnly, and not set when it was cleared.
func pickedTime(raw string, dateOnly bool) (t time.Time, set bool, err error) {
	t, blank, err := pickedMoment(raw)
	if err != nil || blank {
		return time.Time{}, false, err
	}
	if dateOnly {
		t = t.Truncate(24 * time.Hour) // a day is its midnight, whatever time of day was sent by hand
	}
	return t, true, nil
}

// lowerTimeField is what Time and OptionalTime have in common, as the IR holds it: the moment is read through get, which says nothing for one that is not set.
func lowerTimeField[M any](name, label string, dateOnly bool, format func(time.Time) string, get func(M) (time.Time, bool)) ir.Field {
	f := head(name, label, ir.KindString)
	f.Display, f.DateOnly = ir.DisplayTime, dateOnly
	f.Get = func(m any) string {
		if t, ok := get(m.(M)); ok {
			return showMoment(t, dateOnly)
		}
		return ""
	}
	f.Num = func(m any) float64 {
		t, ok := get(m.(M))
		return momentNum(t, ok)
	}
	f.Show = shown(label, f.Get, format, func(m any) (time.Time, bool) {
		t, ok := get(m.(M))
		return t.UTC(), ok
	})
	return f
}

func lowerTime[M any](a Time[M], name string) ir.Field {
	f := lowerTimeField(name, a.Label, a.DateOnly, a.Format, func(m M) (time.Time, bool) {
		t := a.Load(m)
		return t, !t.IsZero()
	})
	f.Rules = ir.Rules{Required: a.Required}
	f.Set = setter(a.Store, func(raw string) (time.Time, error) {
		t, _, err := pickedTime(raw, a.DateOnly) // cleared: stored as not set
		return t, err
	})
	f.Unset = unsetter(a.Store)
	return f
}

func lowerOptionalTime[M any](a OptionalTime[M], name string) ir.Field {
	f := lowerTimeField(name, a.Label, a.DateOnly, a.Format, func(m M) (time.Time, bool) {
		if t := a.Load(m); t != nil && !t.IsZero() {
			return *t, true
		}
		return time.Time{}, false
	})
	f.Nullable = true
	f.Set = setter(a.Store, func(raw string) (*time.Time, error) {
		t, set, err := pickedTime(raw, a.DateOnly)
		if err != nil || !set {
			return nil, err // cleared: stored as none
		}
		return &t, nil
	})
	f.Unset = unsetter(a.Store)
	return f
}

func (a Time[M]) validateAccessor(v *viewValidator, at string, site accessorSite, seen map[string]bool) {
	validateValue(v, at, site, seen, a.Label, a.Load == nil, "func(M) time.Time")
	validateTime(v, at, a)
}

func (a Time[M]) lowerAccessor(name string) ir.Field { return lowerTime(a, name) }

func (a OptionalTime[M]) validateAccessor(v *viewValidator, at string, site accessorSite, seen map[string]bool) {
	validateValue(v, at, site, seen, a.Label, a.Load == nil, "func(M) *time.Time")
}

func (a OptionalTime[M]) lowerAccessor(name string) ir.Field { return lowerOptionalTime(a, name) }
