package ir

import "context"

// Markdown is a leaf that shows one text of a model, set as markdown. It only reads.
type Markdown struct {
	At Addr

	Title string
	Desc  string

	// Load returns the model, as a Form's does.
	Load func(ctx context.Context) (any, error)

	// Content is the markdown, as a field.
	Content Field
}

// Addr reports the node's position in the body tree.
func (m *Markdown) Addr() Addr { return m.At }
