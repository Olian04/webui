package ir

import (
	"context"
)

// Query is what the page's arguments ask of a table: a window and an order.
type Query struct {
	Offset int
	Limit  int
	After  string // a feed's cursor: where the page starts, "" for the first
	Sort   string // a column's Key; empty means the loader's order
	Desc   bool

	// Filters are the column filters in force, by the column's Key. A
	// column with fixed options holds the chosen options; any other holds the
	// one text typed. Empty and unknown ones are already gone.
	Filters map[string][]string

	// Ranges are the bounds on the numeric columns, by Key. A column is in
	// Filters or in Ranges, never both.
	Ranges map[string]Range

	// Search is what the visitor typed in the global search, to find rows of this
	// table by any column, ignoring case. It is empty for the table's own view.
	Search string
}

// Range bounds a number, both ends inclusive. A nil end is unbounded.
type Range struct {
	Min, Max *float64
}

// Window is what a table's Load returns: the rows asked for, how many there are in
// all, and the values of its open-set columns.
type Window struct {
	Rows  []any
	Total int // -1 when unknown

	// Next is, for a feed, the cursor of the page after this one, "" when this is the
	// last.
	Next string

	// Options is, for each OpenSet column by Key, the values the rows hold, sorted.
	// It is over every row, not only this window or what the filters keep, so a
	// filter never offers fewer choices for having been used.
	Options map[string][]string
}

// Table is a leaf listing rows.
type Table struct {
	At Addr

	Title string
	Desc  string

	// Load returns one window of the rows. Total is -1 when the loader does not
	// know it, which disables "of N" and last-page detection.
	Load func(ctx context.Context, q Query) (Window, error)

	// Key identifies a row for selection and row actions; nil when the table
	// declares neither.
	Key func(row any) string

	// ID names this table's view state in the address: "<ID>.offset",
	// "<ID>.sort", "<ID>.desc". Set whenever the table pages or sorts.
	ID       string
	PageSize int // rows per page, always positive: the declaration defaults it

	// Feed says the rows come from a source that hands them out a page at a time by a
	// cursor, with no way to sort or filter them: the table has no sort links and no
	// filters, and its pager is Next and First page.
	Feed bool

	// Search says the table's rows are found by the global search: each is a result
	// that leads where RowClick does. Only a table with a RowClick has it.
	Search bool

	Columns  []Field
	RowClick *Link // nil means rows do not lead anywhere
	// RowAction runs when a row is clicked, in place of RowClick's link: a POST to
	// the page naming the row by Key. At most one of the two is set.
	RowAction *Action
	Actions   []*Action // per row: a button in a trailing cell
	Bulk      []*Action // over the selection: the checkbox column and action bar
}

// Addr reports the node's position in the body tree.
func (t *Table) Addr() Addr { return t.At }
