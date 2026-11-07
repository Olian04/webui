package ir

// Columns arranges children side by side in a row, each as wide as its weight says of the row. Where there is not room for
// every column at its least width, the last of them move to a row of their own.
type Columns struct {
	At    Pos
	Nodes []Node

	// Weights are the columns' shares, one for each child, each at least 1.
	Weights []int
}

// Pos reports the node's position in the view tree.
func (c *Columns) Pos() Pos { return c.At }

func (c *Columns) Children() []Node { return c.Nodes }
