package webui

import (
	"context"
	"fmt"
	"reflect"

	"github.com/Olian04/webui/internal/ir"
)

// Control is a button, a link or an address that belongs to a row of a table: what a click on the row does ([Table.RowClick]), or one of the controls in its trailing
// cell ([Table.Actions]). It is an [Action], which posts: its Run is handed the row, and may look at it to decide where to go, with [Outcome.Then], or change something,
// and it needs the table's Key; a [Link], which leads to a page of the app; or an [Href], which leads to an address. They are interchangeable where one is taken: the
// difference is how each is drawn and what it does, a link being a real anchor, so a middle-click, a new tab and the keyboard work, where an action is a POST that
// cannot be opened in a tab. The interface exists to take any of them, and each type says how it is checked and how it is lowered, so nothing that takes a
// Control looks at which one it was given.
type Control[M any] interface {
	// validateControl checks the control where it is, which says what it needs: a button needs a Label, a click on a row has none.
	validateControl(v *bodyValidator, at string, slot controlSlot)

	// lowerControl is the control as the IR holds it.
	lowerControl() *ir.Control

	// posts is whether the control is a POST, which needs the rows of its table to have an identity.
	posts() bool
}

// controlSlot is where a control is, which decides what it needs and what it refuses.
type controlSlot uint8

const (
	slotRowClick  controlSlot = iota // the click on a whole row: no button, so no Label, and no Confirm to ask from
	slotRowAction                    // a button in the row's trailing cell: a Label, and a Confirm if it asks first
	slotItem                         // an item, a headline or a day, which is read: only what leads somewhere
)

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

// Destination is where an item, a day or a row leads, and it is not something that is done: a [Link] to a page of the app, or an [Href] to an address. It is a
// [Control] that cannot be an [Action], which is a POST, and is what [List], [Cards] and [Calendar] take, since what they show is read. Every Destination is a Control,
// so a slot that takes a Control takes it.
type Destination[M any] interface {
	Control[M]
	destination()
}

// Href is an address as a control: a page on another site, or a path on this one that is not a page of the app, built from M. It opens in a tab of its
// own, with the mark that says so and rel="noopener noreferrer". An address that is not a path on this site or an http, https or mailto address, such as a
// script, is not a link, and what would have led there is plain text, so data in the model cannot make one. An item whose URL says "" has no link.
type Href[M any] struct {
	// URL is the address, built from the item, the row or the day.
	URL func(M) string

	// Label and Role are the button's text and style, in [Table.Actions], where an address is a button. Where it is the whole row or an item's headline there is no
	// button, and they are ignored.
	Label string
	Role  Role
}

// ASSERT: Link, Href and Action implement Control, and Link and Href implement Destination
var (
	_ Control[struct{}]     = Link[struct{}, struct{}]{}
	_ Control[struct{}]     = Href[struct{}]{}
	_ Control[struct{}]     = Action[struct{}]{}
	_ Destination[struct{}] = Link[struct{}, struct{}]{}
	_ Destination[struct{}] = Href[struct{}]{}
)

func (l Link[M, A]) validateLink(v *bodyValidator) {
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

func (Link[M, A]) destination() {}
func (Href[M]) destination()    {}

func (Link[M, A]) posts() bool { return false }
func (Href[M]) posts() bool    { return false }

func (l Link[M, A]) validateControl(v *bodyValidator, at string, slot controlSlot) {
	if slot == slotRowAction && l.Label == "" {
		v.add(at+" has no Label", "Set Label; it is the button text.")
	}
	l.validateLink(v)
}

func (l Link[M, A]) lowerControl() *ir.Control {
	return &ir.Control{Label: l.Label, Role: ir.Role(l.Role), Link: l.lowerLink()}
}

func (h Href[M]) validateControl(v *bodyValidator, at string, slot controlSlot) {
	if slot == slotRowAction && h.Label == "" {
		v.add(at+" has no Label", "Set Label; it is the button text.")
	}
	if h.URL == nil {
		v.add(at+" is an Href with no URL", "Set URL to func(M) string, returning the address.")
	}
}

func (h Href[M]) lowerControl() *ir.Control {
	return &ir.Control{Label: h.Label, Role: ir.Role(h.Role), External: func(m any) string {
		if u := h.URL(m.(M)); validURL(u) {
			return u
		}
		return ""
	}}
}

func (Link[M, A]) toolbarItem(M) {}

func (l Link[M, A]) validateButton(v *bodyValidator, at string) {
	if l.Label == "" {
		v.add(at+" has no Label", "Set Label; it is the button text.")
	}
	l.validateLink(v)
}

// toolbarButton is a toolbar's link: its Args are built with no subject, so what the runtime hands over is ignored, and the zero M, which is NoArgs,
// is what they are given.
func (l Link[M, A]) toolbarButton() ir.ToolbarItem {
	link := l.lowerLink()
	link.Args = func(ctx context.Context, _ any) any { var none M; return l.Args(ctx, none) }
	return ir.ToolbarItem{Label: l.Label, Role: ir.Role(l.Role), Link: link}
}

func (l Link[M, A]) lowerLink() *ir.Link {
	return &ir.Link{
		Dest: l.Page.pagePath(),
		Args: func(ctx context.Context, row any) any { return l.Args(ctx, row.(M)) },
	}
}
