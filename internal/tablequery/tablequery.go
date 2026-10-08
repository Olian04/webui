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
	"slices"
	"strings"

	"github.com/Olian04/webui/internal/ir"
)

// Apply returns the window q asks of rows, and how many rows pass the filters.
// Rows are not changed; a column q names that cols does not have is ignored.
func Apply(rows []any, cols []ir.Field, q ir.Query) (window []any, total int) {
	byName := make(map[string]ir.Field, len(cols))
	for _, c := range cols {
		byName[c.Key] = c
	}

	kept := make([]any, 0, len(rows))
rows:
	for _, row := range rows {
		for name, values := range q.Filters {
			if c, ok := byName[name]; ok && !matches(c, row, values) {
				continue rows
			}
		}
		for name, r := range q.Ranges {
			if c, ok := byName[name]; ok && c.Num != nil && !within(c.Num(row), r) {
				continue rows
			}
		}
		kept = append(kept, row)
	}

	if c, ok := byName[q.Sort]; ok && q.Sort != "" {
		slices.SortStableFunc(kept, func(a, b any) int {
			if q.Desc {
				a, b = b, a
			}
			return compare(c, a, b)
		})
	}

	total = len(kept)
	lo := min(max(q.Offset, 0), total)
	hi := total
	if q.Limit > 0 {
		hi = min(lo+q.Limit, total)
	}
	return kept[lo:hi], total
}

// matches is whether a row passes one column's filter: a column with a fixed set
// of options keeps the rows holding any of the options chosen, and any other the
// rows whose text contains what was typed, ignoring case.
func matches(c ir.Field, row any, values []string) bool {
	if len(values) == 0 {
		return true
	}
	have := c.Get(row)
	if c.Options != nil {
		return slices.Contains(values, have)
	}
	return strings.Contains(strings.ToLower(have), strings.ToLower(values[0]))
}

// within is whether x is inside the bounds, both ends inclusive; an end that is
// nil is unbounded.
func within(x float64, r ir.Range) bool {
	return (r.Min == nil || x >= *r.Min) && (r.Max == nil || x <= *r.Max)
}

// compare orders two rows by a column: a number as a number, anything else as text
// without regard to case, and by the exact text when that ties, so the order is
// the same however the rows arrived.
func compare(c ir.Field, a, b any) int {
	if c.Num != nil {
		return cmp.Compare(c.Num(a), c.Num(b))
	}
	x, y := c.Get(a), c.Get(b)
	if n := cmp.Compare(strings.ToLower(x), strings.ToLower(y)); n != 0 {
		return n
	}
	return cmp.Compare(x, y)
}
