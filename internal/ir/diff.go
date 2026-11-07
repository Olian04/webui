package ir

import "context"

// Diff is a panel that shows two texts of one model side by side, and what changed
// between them. It only reads.
type Diff struct {
	At Pos

	Title string
	Desc  string

	// Load returns the model, as a Form's does.
	Load func(ctx context.Context) (any, error)

	// Original and Modified are the two texts, as fields: their labels name the sides.
	Original Field
	Modified Field

	Language string // Monaco's id for it
}

// Pos reports the node's position in the view tree.
func (d *Diff) Pos() Pos { return d.At }

func (d *Diff) Children() []Node { return nil }
