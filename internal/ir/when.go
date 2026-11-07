package ir

import "context"

// When arranges one child, which it shows only while a condition holds of what Load reads. It is the one layout that loads, and it loads for
// nothing but the condition: the child loads its own data. A panel under a When that does not hold is not there, to a request that names it.
type When struct {
	At Pos

	// Load reads the model that the condition is decided from.
	Load func(ctx context.Context) (any, error)

	// Show is the condition, as a Bool: its Get is "true" for a model that shows the child.
	Show Field

	// Child is what is shown, at the position its own address says, whether it is shown or not.
	Child Node
}

// Pos reports the node's position in the view tree.
func (w *When) Pos() Pos { return w.At }

func (w *When) Children() []Node {
	return []Node{w.Child}
}
