package webui

import "context"

// RowClick is a row activation: a Link (GET) or an Action (POST).
type RowClick[M any] interface {
	isRowClick()
}

// Link names a destination page and how to build its arguments from M.
// The page and argument type must agree; the compiler checks that.
type Link[M, A any] struct {
	Page Page[A]
	Args func(ctx context.Context, m M) A
}

func (Link[M, A]) isRowClick() {}

// ASSERT: Link implements RowClick
var _ RowClick[struct{}] = Link[struct{}, struct{}]{}
