package ir

import (
	"context"
	"strings"
)

// Embed is a panel that shows a page of this site in a frame. The address is what Load returns for the request.
type Embed struct {
	At Pos
	Headed

	Title string
	Desc  string

	// Load returns the path of the page: the model it returns is a string.
	Load func(ctx context.Context) (any, error)
}

// Pos reports the node's position in the view tree.
func (e *Embed) Pos() Pos { return e.At }

func (e *Embed) Children() []Node { return nil }

// SamePath is whether an address is a path on this site: it begins with one slash, and is not an address on another site or a path that a browser reads as one.
func SamePath(s string) bool {
	return strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") && !strings.HasPrefix(s, `/\`) && !strings.ContainsAny(s, "\n\r")
}
