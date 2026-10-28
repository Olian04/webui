package ir

import "context"

// Stat is a leaf that shows a few figures of one model, each as a tile: a label, a value, and the change in it and its
// trend when it has them. It only reads.
type Stat struct {
	At Addr

	Title string
	Desc  string

	// Load returns the model, as a Form's does.
	Load func(ctx context.Context) (any, error)

	Tiles []StatTile
}

// StatTile is one figure. Value is any field that is read, and Change and Trend are nil when the tile has none.
type StatTile struct {
	Label string
	Help  string
	Value Field

	// Change is a DisplayDelta field, and Trend a DisplaySeries one.
	Change *Field
	Trend  *Field
}

// Addr reports the node's position in the body tree.
func (s *Stat) Addr() Addr { return s.At }
