package ir

import "context"

// Markdown is a panel that shows one text of a model, set as markdown. It only reads.
type Markdown struct {
	At Pos

	Title string
	Desc  string

	// Load returns the model, as a Form's does.
	Load func(ctx context.Context) (any, error)

	// Content is the markdown, as a field.
	Content Field

	// TrustedOrigins are the origins a picture in the text may be drawn from, each as
	// markdown.Origin writes it: "https://host", with a port only when it is not the default.
	TrustedOrigins []string
}

// Pos reports the node's position in the view tree.
func (m *Markdown) Pos() Pos { return m.At }

func (m *Markdown) Children() []Node { return nil }
