// Package tablequery answers a table's Query from rows held in memory: the
// filters, the sort and the window, so a page whose data is a slice does none of
// it itself.
//
// It works from the columns' own accessors, so what a column sorts and filters by
// is what it shows: text compared as text, a number as a number. A loader that
// pages a large source does the same work in its own way, which is why Table has
// Load as well.
package tablequery

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/Olian04/webui/internal/ir"
)

// Apply returns the window q asks of rows, and how many rows pass the filters.
// Rows are not changed; a column q names that cols does not have is ignored.
//
// What the query asks is resolved once, before the rows: the column each filter and range is on, and
// the text a filter looks for in lower case. The sort reads each row's key once, not at every
// comparison.
func Apply(rows []any, cols []ir.Field, q ir.Query) (window []any, total int) {
	var filters []filter
	for name, values := range q.Filters {
		if c := column(cols, name); c != nil && len(values) > 0 {
			f := filter{col: c, values: values}
			if c.Options == nil && !c.OpenSet {
				f.needle = strings.ToLower(values[0])
			}
			filters = append(filters, f)
		}
	}
	var ranges []bounds
	for name, r := range q.Ranges {
		if c := column(cols, name); c != nil {
			ranges = append(ranges, bounds{col: c, r: r})
		}
	}

	kept := make([]any, 0, len(rows))
rows:
	for _, row := range rows {
		for i := range filters {
			if !filters[i].passes(row) {
				continue rows
			}
		}
		for i := range ranges {
			c := ranges[i].col
			if c.Num != nil && !within(c.Num(row), ranges[i].r) {
				continue rows
			}
			// A moment that is not set, or is not a moment, is before every start and
			// after no end by its number, so a filter that sets only an end would keep it.
			// It has no time to be within, so any range drops it.
			if c.Display == ir.DisplayTime && math.IsInf(c.Num(row), 0) {
				continue rows
			}
		}
		kept = append(kept, row)
	}

	if c := column(cols, q.Sort); c != nil && q.Sort != "" {
		sortRows(kept, c, q.Desc)
	}

	total = len(kept)
	lo := min(max(q.Offset, 0), total)
	hi := total
	if q.Limit > 0 {
		hi = min(lo+q.Limit, total)
	}
	return kept[lo:hi], total
}

// column is the column of cols with the key, or nil. Where several have it, the last is it.
func column(cols []ir.Field, key string) *ir.Field {
	for i := len(cols) - 1; i >= 0; i-- {
		if cols[i].Key == key {
			return &cols[i]
		}
	}
	return nil
}

// filter is one column's filter as the query asks it: needle is what a column without a set of
// options looks for, in lower case.
type filter struct {
	col    *ir.Field
	values []string
	needle string
}

// passes is whether a row passes the filter: a column with a fixed set of options keeps the rows
// holding any of the options chosen, and any other the rows whose text contains what was typed,
// ignoring case.
func (f *filter) passes(row any) bool {
	if f.col.Options != nil || f.col.OpenSet {
		// A select may hold several: the row passes if it holds any of the options chosen.
		return slices.ContainsFunc(held(f.col, row), func(v string) bool { return slices.Contains(f.values, v) })
	}
	return strings.Contains(strings.ToLower(f.col.Get(row)), f.needle)
}

// bounds is the range a query puts on a numeric column.
type bounds struct {
	col *ir.Field
	r   ir.Range
}

// keyed is a row with what it is ordered by, read once.
type keyed struct {
	row   any
	num   float64
	text  string
	lower string
}

// sortRows orders rows by a column: a number as a number, anything else as text without regard to
// case, and by the exact text when that ties, so the order is the same however the rows arrived.
// Rows that are equal stay as they were.
func sortRows(rows []any, c *ir.Field, desc bool) {
	keys := make([]keyed, len(rows))
	for i, row := range rows {
		keys[i].row = row
		if c.Num != nil {
			keys[i].num = c.Num(row)
		} else {
			keys[i].text = c.Get(row)
			keys[i].lower = strings.ToLower(keys[i].text)
		}
	}
	slices.SortStableFunc(keys, func(a, b keyed) int {
		if desc {
			a, b = b, a
		}
		if c.Num != nil {
			return cmp.Compare(a.num, b.num)
		}
		if n := cmp.Compare(a.lower, b.lower); n != 0 {
			return n
		}
		return cmp.Compare(a.text, b.text)
	})
	for i := range keys {
		rows[i] = keys[i].row
	}
}

// Find returns the rows in which any column contains text, ignoring case, in their own order, and
// at most limit of them (all of them when limit is not positive). It looks in what a column shows,
// which is what a person typing in a search box has in front of them.
func Find(rows []any, cols []ir.Field, text string, limit int) []any {
	var out []any
	for _, row := range rows {
		if !found(cols, row, text) {
			continue
		}
		out = append(out, row)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}

// Options are the values the rows hold in each of the open-set columns, by Key,
// sorted without regard to case. Every row counts, whatever the filters.
func Options(rows []any, cols []ir.Field) map[string][]string {
	var out map[string][]string
	for i := range cols {
		c := &cols[i]
		if !c.OpenSet || c.Get == nil {
			continue
		}
		seen := map[string]bool{}
		values := []string{}
		for _, row := range rows {
			for _, v := range held(c, row) {
				if !seen[v] {
					seen[v] = true
					values = append(values, v)
				}
			}
		}
		slices.SortFunc(values, func(a, b string) int {
			if n := cmp.Compare(strings.ToLower(a), strings.ToLower(b)); n != 0 {
				return n
			}
			return cmp.Compare(a, b)
		})
		if out == nil {
			out = map[string][]string{}
		}
		out[c.Key] = values
	}
	return out
}

// held are the values a row has in a column with a fixed or open set: the one it shows, or for a select,
// each of those it holds. A row with none holds none.
func held(c *ir.Field, row any) []string {
	if c.Display == ir.DisplaySelect {
		return c.Values(row)
	}
	if v := c.Get(row); v != "" {
		return []string{v}
	}
	return nil
}

// shown is what a column shows in a row, for finding it by what was typed: for a select, each value and
// what it says, since the values are not joined for reading; for a column with a Format, the text it
// makes and the value's own, so "1.5 GiB" and "1572864" both find the row.
func shown(c ir.Field, row any) string {
	if c.Display != ir.DisplaySelect {
		if c.Show != nil {
			return c.Show(row) + " " + c.Get(row)
		}
		return c.Get(row)
	}
	var parts []string
	for _, v := range c.Values(row) {
		parts = append(parts, v, c.LabelOf(v))
	}
	return strings.Join(parts, " ")
}

// found is whether any column of a row contains what was searched for, ignoring
// case: the global search looks in every column the row shows.
func found(cols []ir.Field, row any, text string) bool {
	needle := strings.ToLower(text)
	for _, c := range cols {
		if c.Get != nil && strings.Contains(strings.ToLower(shown(c, row)), needle) {
			return true
		}
	}
	return false
}

// within is whether x is inside the bounds, both ends inclusive; an end that is
// nil is unbounded.
func within(x float64, r ir.Range) bool {
	return (r.Min == nil || x >= *r.Min) && (r.Max == nil || x <= *r.Max)
}
