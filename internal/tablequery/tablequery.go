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
			// A moment that is not set, or is not a moment, is before every start and
			// after no end by its number, so a filter that sets only an end would keep it.
			// It has no time to be within, so any range drops it.
			if c, ok := byName[name]; ok && c.Display == ir.DisplayTime && math.IsInf(c.Num(row), 0) {
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
	for _, c := range cols {
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

// matches is whether a row passes one column's filter: a column with a fixed set
// of options keeps the rows holding any of the options chosen, and any other the
// rows whose text contains what was typed, ignoring case.
func matches(c ir.Field, row any, values []string) bool {
	if len(values) == 0 {
		return true
	}
	if c.Options != nil || c.OpenSet {
		// A select may hold several: the row passes if it holds any of the options chosen.
		return slices.ContainsFunc(held(c, row), func(v string) bool { return slices.Contains(values, v) })
	}
	return strings.Contains(strings.ToLower(c.Get(row)), strings.ToLower(values[0]))
}

// held are the values a row has in a column with a fixed or open set: the one it shows, or for a select,
// each of those it holds. A row with none holds none.
func held(c ir.Field, row any) []string {
	if c.Display == ir.DisplaySelect {
		return c.Values(row)
	}
	if v := c.Get(row); v != "" {
		return []string{v}
	}
	return nil
}

// shown is what a column shows in a row, for finding it by what was typed: for a select, each value and
// what it says, since the values are not joined for reading.
func shown(c ir.Field, row any) string {
	if c.Display != ir.DisplaySelect {
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
