package ir

import (
	"context"
)

// Table is a leaf listing rows.
type Table struct {
	At Addr

	// Load returns the rows and the total matching count. Total is -1 when the
	// loader does not know it, which disables "of N" and last-page detection.
	Load func(ctx context.Context) (rows []any, total int, err error)

	Columns  []Field
	RowClick RowTarget // nil means rows are not clickable
	Actions  []*Action
	Bulk     []*Action
}

// Kind reports the node kind.
func (t *Table) Kind() NodeKind { return NodeTable }

// Addr reports the node's position in the body tree.
func (t *Table) Addr() Addr { return t.At }

// RowTarget is what activating a row does.
type RowTarget interface {
	isRowTarget()
}
