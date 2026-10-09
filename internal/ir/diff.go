package ir

import "context"

// Diff is a leaf that shows two texts of one model side by side, and what changed
// between them. It only reads.
type Diff struct {
	At Addr

	Title string
	Desc  string

	// Load returns the model, as a Form's does.
	Load func(ctx context.Context) (any, error)

	// Original and Modified are the two texts, as fields: their labels name the sides.
	Original Field
	Modified Field

	Language string // Monaco's id for it
}

// Addr reports the node's position in the body tree.
func (d *Diff) Addr() Addr { return d.At }
