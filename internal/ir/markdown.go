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

	// TrustedOrigins are the origins a picture in the text may be drawn from, each as
	// markdown.Origin writes it: "https://host", with a port only when it is not the default.
	TrustedOrigins []string
}

// Addr reports the node's position in the body tree.
func (m *Markdown) Addr() Addr { return m.At }
