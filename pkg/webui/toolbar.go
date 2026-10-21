package webui

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/Olian04/webui/internal/ir"
)

// Toolbar is a leaf that is a row of controls for the page it is on: buttons that do something or lead somewhere, and
// the values of the page's arguments, to read. A is the page's argument type, and everything in a toolbar is about the
// page's arguments as they are for the request: that is what a button's Guard and Run are handed, what a link builds its
// destination from, and what a value shows.
//
//	webui.Toolbar[DevicesArgs]{
//		Left: []webui.ToolbarItem[DevicesArgs]{
//			webui.Link[DevicesArgs, NewDeviceArgs]{Label: "Create device", Role: webui.RolePrimary, Page: NewDevice, Args: ...},
//			webui.Action[DevicesArgs]{Label: "Sync now", Guard: canEdit, Run: syncNow},
//		},
//		Right: []webui.ToolbarItem[DevicesArgs]{
//			webui.Select[DevicesArgs]{Label: "Window", Load: ..., Options: ...},
//		},
//	}
//
// The controls are in two groups, Left and Right, in the order they are given. With room, the left ones are at the left
// edge and the right ones at the right, and the space between them is what the page has to spare. When there is no longer room for
// both, there is no longer a left and a right: all the controls, the left ones first, flow from the left in rows, as a
// grid does, and the space between them is the same. A toolbar with controls in one of them only is fine.
//
// The items are:
//
//   - An [Action] is a button that does something. It posts, and its Run says how it ended with an [Outcome], as
//     any action does. Its Guard decides whether the visitor may: an error leaves the button out, and [Disabled] draws it
//     disabled, with its message.
//   - A [Link] is a button that goes to another page, with a Label and a Role. It is a real anchor, and is left out for a
//     visitor whom the page it leads to refuses.
//   - An accessor, such as a [Select], an [Int] or a [Bool], is a value of the page's arguments, shown read-only: its Label and
//     what its Load reads from the arguments, as a table cell or a read-only form field shows it (a [Select] as its badges, a
//     number as a number, with its Format). A Store on it is ignored, as it is in a table, so one accessor serves a form, a
//     table and a toolbar (a [File] is the exception: it is a form's input and a table's column, and not a toolbar's). A toolbar has no inputs: nothing in it changes the page's arguments, and it has no form of its own. A
//     page whose arguments the visitor sets uses a [Form], and a [Link] for a page that takes other arguments.
//
// A toolbar is not a panel: it has no frame, so it sits on the page, in a [Stack] with what it is for. One whose controls are all
// left out for the visitor is not drawn.
type Toolbar[A any] struct {
	// Left are the controls at the left edge, in order.
	Left []ToolbarItem[A]

	// Right are the controls at the right edge, in order. A toolbar with nothing in either is a mistake.
	Right []ToolbarItem[A]
}

// items are the controls, the left ones first, with where the right ones start.
func (t Toolbar[A]) items() []ToolbarItem[A] {
	return append(append([]ToolbarItem[A](nil), t.Left...), t.Right...)
}

// ToolbarItem is a control of a [Toolbar] of arguments A: an [Action] or a [Link] of A, or an accessor of A, which is shown
// as its value. Only this package implements it.
type ToolbarItem[A any] interface {
	toolbarItem(A)
}

// toolbarButton is what an [Action] and a [Link] are to a toolbar.
type toolbarButton interface {
	validateButton(v *bodyValidator, at string)
	toolbarButton() ir.ToolbarItem
}

// ASSERT: Toolbar implements PageBody
var _ PageBody = Toolbar[struct{}]{}

func (t Toolbar[A]) validateBody(v *bodyValidator) {
	if len(t.Left)+len(t.Right) == 0 {
		v.add("a Toolbar has no controls", "Put the controls it is for in Left or in Right: an Action, a Link, or an accessor to show.")
	}
	if v.argType != nil && v.argType != reflect.TypeFor[A]() {
		v.add(fmt.Sprintf("a Toolbar of %s is on a page that takes %s", typeName(reflect.TypeFor[A]()), v.args),
			"A Toolbar is about the arguments of the page it is on: declare it as a Toolbar of that type.")
	}
	seen := map[string]bool{}
	for i, item := range t.items() {
		at := fmt.Sprintf("Toolbar.Left[%d]", i)
		if i >= len(t.Left) {
			at = fmt.Sprintf("Toolbar.Right[%d]", i-len(t.Left))
		}
		switch it := any(item).(type) {
		case toolbarButton:
			it.validateButton(v, at)
		case Accessor[A]:
			validateAccessor[A](v, it, at, accessorSite{where: "Toolbar", uniqueLbl: true}, seen)
		case nil:
			v.add(at+" is nil", "Remove it or declare a control.")
		}
	}
}

func (t Toolbar[A]) lowerBody(at ir.Addr) ir.Node {
	out := &ir.Toolbar{At: at, RightFrom: len(t.Left)}
	for i, item := range t.items() {
		switch it := any(item).(type) {
		case toolbarButton:
			out.Items = append(out.Items, it.toolbarButton())
		case Accessor[A]:
			f := lowerAccessor[A](it, "t"+strconv.Itoa(i))
			out.Items = append(out.Items, ir.ToolbarItem{Field: &f})
		}
	}
	return out
}
