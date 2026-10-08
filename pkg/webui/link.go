package webui

import (
	"context"
	"fmt"
	"reflect"

	"github.com/Olian04/webui/internal/ir"
)

// RowClick is a row activation: a Link (GET) or an Action (POST).
type RowClick[M any] interface {
	isRowClick()
	validateRow(v *bodyValidator)
	lowerRow(l *bodyLowerer) ir.RowTarget
}

// Link names a destination page and how to build its arguments from M. Page is
// the page itself or its PageID; naming a PageID is how a page links back to one
// that links to it. The page and argument type must agree; the compiler checks
// that.
type Link[M, A any] struct {
	Page PageRef[A]
	Args func(ctx context.Context, m M) A
}

func (Link[M, A]) isRowClick() {}

// ASSERT: Link implements RowClick
var _ RowClick[struct{}] = Link[struct{}, struct{}]{}

func (l Link[M, A]) validateRow(v *bodyValidator) {
	if l.Args == nil {
		v.add("a Link has no Args", "Set Args to build the destination's argument struct from the row.")
	}
	if l.Page == nil {
		v.add("a Link has no Page", "Set Page to the destination page, or its PageID.")
		return
	}
	path := l.Page.pagePath()
	want, mounted := v.facts.argTypes[path]
	if !mounted {
		v.add(fmt.Sprintf("a link targets %q, which is not mounted in this app", path),
			"Add that page to App.Pages, or point the link at a page that is already there.")
		return
	}
	if got := reflect.TypeFor[A](); got != want {
		v.add(fmt.Sprintf("a link targets %q with %s arguments, but that page takes %s",
			path, typeName(got), typeName(want)),
			"Point the link at the page the mount list contains, not a copy with other arguments.")
	}
}

func (l Link[M, A]) lowerRow(_ *bodyLowerer) ir.RowTarget {
	return &ir.Link{
		Dest: l.Page.pagePath(),
		Args: func(ctx context.Context, row any) any { return l.Args(ctx, row.(M)) },
	}
}
