package ir

import (
	"context"
)

// Query is what the page's arguments ask of a table: a window and an order.
type Query struct {
	Offset int
	Limit  int
	Sort   string // a column's Key; empty means the loader's order
	Desc   bool

	// Filters are the column filters in force, by the column's Key. A
	// column with fixed options holds the chosen options; any other holds the
	// one text typed. Empty and unknown ones are already gone.
	Filters map[string][]string

	// Ranges are the bounds on the numeric columns, by Key. A column is in
	// Filters or in Ranges, never both.
	Ranges map[string]Range
}

// Range bounds a number, both ends inclusive. A nil end is unbounded.
type Range struct {
	Min, Max *float64
}

// Table is a leaf listing rows.
type Table struct {
	At Addr

	Title string
	Desc  string

	// Load returns the rows and the total matching count. Total is -1 when the
	// loader does not know it, which disables "of N" and last-page detection.
	Load func(ctx context.Context, q Query) (rows []any, total int, err error)

	// Key identifies a row for selection and row actions; nil when the table
	// declares neither.
	Key func(row any) string

	// ID names this table's view state in the address: "<ID>.offset",
	// "<ID>.sort", "<ID>.desc". Set whenever the table pages or sorts.
	ID       string
	PageSize int // rows per page; 0 means the table does not page

	Columns  []Field
	RowClick RowTarget // nil means rows are not clickable
	Actions  []*Action // per row: a button in a trailing cell
	Bulk     []*Action // over the selection: the checkbox column and action bar
}

// Kind reports the node kind.
func (t *Table) Kind() NodeKind { return NodeTable }

// Addr reports the node's position in the body tree.
func (t *Table) Addr() Addr { return t.At }

// RowTarget is what activating a row does.
type RowTarget interface {
	isRowTarget()
}
