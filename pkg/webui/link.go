package webui

import (
	"context"
	"fmt"
	"reflect"

	"github.com/Olian04/webui/internal/ir"
)

// RowClick is what a click on a row does: a [Link] or an [Action].
//
// A Link is for the simple case, a row that leads to a page. It is one real anchor,
// so middle-click, open-in-new-tab and the keyboard work. An Action is for the rest: its Run is handed the row and may look at it
// to decide where to go, with Outcome.Then, or change something. It is a POST, so
// the row cannot be opened in a new tab, and it needs the table's Key. The
// interface exists to take either.
type RowClick[M any] interface {
	validateRow(v *bodyValidator)
	lowerRow() (*ir.Link, *ir.Action)
}

// Link names a destination page and how to build its arguments from M. Page is
// the page itself or its PageID; naming a PageID is how a page links back to one
// that links to it. The page and the argument type must agree, which the compiler
// checks, and [App.Compile] checks that the page is mounted. When the destination
// has a form, the library remembers the page the user came from, so the form's
// Cancel returns there, and so does a save, unless the Outcome says otherwise with
// [Outcome.Stay] or [Outcome.Then].
//
// A link to a page that refuses the visitor is not drawn: when it is, the page's Guard is asked with the arguments the link builds,
// and a toolbar's button, or a table's row, is left out or plain. A panic in that Guard answers no, and is logged.
type Link[M, A any] struct {
	// Page is the destination: a [Page] with arguments A, or its [PageID].
	Page PageRef[A]

	// Args builds the destination's arguments from a row, or, in a [Toolbar], from the request, where the page's are [ArgsOf]: a toolbar's link has no subject, and is a Link of [NoArgs].
	Args func(ctx context.Context, m M) A

	// Label and Role are the button's text and style, in a [Toolbar], where a link is a button. A table's
	// RowClick has no button, and ignores them.
	Label string
	Role  Role
}

// Destination is where an item, a row or a day leads, and it is not something that is done: a [Link] to a page of the app, or an [Href] to an address.
// It is a real anchor, so a middle-click, a new tab and the keyboard work, which is why it cannot be an [Action], which is a POST. The interface
// exists to take either.
type Destination[M any] interface {
	validateDestination(v *bodyValidator, what string)
	lowerDestination() (*ir.Link, func(model any) string)
}

// Href is a destination that is an address: a page on another site, or a path on this one that is not a page of the app, built from M. It opens in a tab of its
// own, with the mark that says so and rel="noopener noreferrer". An address that is not a path on this site or an http, https or mailto address, such as a
// script, is not a link, and what would have led there is plain text, so data in the model cannot make one. An item whose URL says "" has no link.
type Href[M any] struct {
	// URL is the address, built from the item, the row or the day.
	URL func(M) string
}

// ASSERT: Link and Action implement RowClick, and Link and Href implement Destination
var (
	_ RowClick[struct{}]    = Link[struct{}, struct{}]{}
	_ RowClick[struct{}]    = Action[struct{}]{}
	_ Destination[struct{}] = Link[struct{}, struct{}]{}
	_ Destination[struct{}] = Href[struct{}]{}
)

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

func (l Link[M, A]) validateDestination(v *bodyValidator, _ string) { l.validateRow(v) }

func (l Link[M, A]) lowerDestination() (*ir.Link, func(model any) string) {
	link, _ := l.lowerRow()
	return link, nil
}

func (h Href[M]) validateDestination(v *bodyValidator, what string) {
	if h.URL == nil {
		v.add(what+" is an Href with no URL", "Set URL to func(M) string, returning the address.")
	}
}

func (h Href[M]) lowerDestination() (*ir.Link, func(model any) string) {
	return nil, func(m any) string {
		if u := h.URL(m.(M)); validURL(u) {
			return u
		}
		return ""
	}
}

func (Link[M, A]) toolbarItem(M) {}

func (l Link[M, A]) validateButton(v *bodyValidator, at string) {
	if l.Label == "" {
		v.add(at+" has no Label", "Set Label; it is the button text.")
	}
	l.validateRow(v)
}

// toolbarButton is a toolbar's link: its Args are built with no subject, so what the runtime hands over is ignored, and the zero M, which is NoArgs,
// is what they are given.
func (l Link[M, A]) toolbarButton() ir.ToolbarItem {
	link, _ := l.lowerRow()
	link.Args = func(ctx context.Context, _ any) any { var none M; return l.Args(ctx, none) }
	return ir.ToolbarItem{Label: l.Label, Role: ir.Role(l.Role), Link: link}
}

func (l Link[M, A]) lowerRow() (*ir.Link, *ir.Action) {
	return &ir.Link{
		Dest: l.Page.pagePath(),
		Args: func(ctx context.Context, row any) any { return l.Args(ctx, row.(M)) },
	}, nil
}
