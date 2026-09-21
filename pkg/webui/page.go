package webui

import "context"

// NoArgs is the argument type for a page with no path or query parameters.
type NoArgs struct{}

// Page is a route. A is the argument struct: path fields match {placeholders},
// the rest are query parameters.
type Page[A any] struct {
	Path  string
	Nav   Nav
	Guard func(ctx context.Context, a A) error
	Body  PageBody
}

func (Page[A]) isPage() {}

// Nav is a navbar entry. Shadow points at another Nav's identity (the pointer)
// so this page highlights that entry without appearing in the bar.
type Nav struct {
	Label  string
	Shadow *Nav
}

// PageBody is a leaf (Table, Form) or a layout (Stack, Split, Tabs).
type PageBody interface {
	isPageBody()
}

// ASSERT: Page implements PageDecl
var _ pageLike = Page[struct{}]{}
