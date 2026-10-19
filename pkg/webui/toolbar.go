package webui

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/rules"
)

// Toolbar is a leaf that is a row of controls for the page it is on: buttons that do something or lead
// somewhere, and inputs that change the page's arguments. A is the page's argument type, and everything in a
// toolbar is about the page's arguments as they are for the request: that is what a button's Guard and Run are
// handed, what a link builds its destination from, and what an input shows and changes.
//
//	webui.Toolbar[DevicesArgs]{Items: []webui.ToolbarItem[DevicesArgs]{
//		webui.Link[DevicesArgs, NewDeviceArgs]{Label: "Create device", Role: webui.RolePrimary, Page: NewDevice, Args: ...},
//		webui.Action[DevicesArgs]{Label: "Sync now", Guard: canEdit, Run: syncNow},
//		webui.Select[DevicesArgs]{Label: "Window", Load: ..., Store: ..., Options: ...},
//	}}
//
// The items are, in the order they are drawn:
//
//   - An [Action] is a button that does something. It posts, and its Run says how it ended with an [Outcome], as
//     any action does. Its Guard decides whether the visitor may, and disables the button, with the reason, when
//     they may not.
//   - A [Link] is a button that goes to another page, with a Label and a Role. It is a real anchor.
//   - An accessor with a Store, such as a [Select], an [Int] or a [String], is an input for the page's arguments.
//     Its Load reads the value from the arguments and its Store writes the visitor's to them, and then the visitor
//     is taken to the page with the arguments that came of it, which is a link that can be copied. The rules of the
//     accessor are checked, in the browser and again on the server, and a [Select] may read its options with
//     LoadOptions. An input is applied when it changes with script, and with an Apply button without. An input that
//     is emptied is set to nothing: its Store is handed the zero value. Every table of the page returns to its first
//     page, and keeps its sort and filters.
//
// A toolbar is not a panel: it has no frame, so it sits on the page, in a [Stack] with what it is for.
type Toolbar[A any] struct {
	// Items are the controls, in the order they are drawn: a toolbar with nothing in it is a mistake.
	Items []ToolbarItem[A]
}

// ToolbarItem is a control of a [Toolbar] of arguments A: an [Action] or a [Link] of A, or an accessor of A with a
// Store. Only this package implements it.
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
	if len(t.Items) == 0 {
		v.add("a Toolbar has no Items", "List the controls it is for: an Action, a Link, or an accessor with a Store.")
	}
	if v.argType != nil && v.argType != reflect.TypeFor[A]() {
		v.add(fmt.Sprintf("a Toolbar of %s is on a page that takes %s", typeName(reflect.TypeFor[A]()), v.args),
			"A Toolbar is about the arguments of the page it is on: declare it as a Toolbar of that type.")
	}
	seen := map[string]bool{}
	for i, item := range t.Items {
		at := fmt.Sprintf("Toolbar.Items[%d]", i)
		switch it := any(item).(type) {
		case toolbarButton:
			it.validateButton(v, at)
		case Accessor[A]:
			validateAccessor[A](v, it, at, accessorSite{where: "Toolbar.Items", uniqueLbl: true}, seen)
			if lowerAccessor[A](it, "t").Set == nil {
				v.add(at+" has no Store", "An input in a toolbar changes the page's arguments: set Store to write the value to them.")
			}
		case nil:
			v.add(at+" is nil", "Remove it or declare a control.")
		}
	}
}

func (t Toolbar[A]) lowerBody(at ir.Addr) ir.Node {
	out := &ir.Toolbar{At: at, Bind: bindToolbar[A]}
	for i, item := range t.Items {
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

// bindToolbar applies what the inputs were given to the page's arguments, checking each as a form does, and
// collecting every failure. An input that is emptied is set to nothing.
func bindToolbar[A any](fields []ir.Field, base any, values map[string]string) (any, []ir.FieldError) {
	m := base.(A)
	var errs []ir.FieldError
	for _, f := range fields {
		if f.Set == nil {
			continue
		}
		raw := values[f.Name]
		if f.Display == ir.DisplaySelect {
			if msg := rules.CheckSelect(f, raw); msg != "" {
				errs = append(errs, ir.FieldError{Label: f.Label, Message: msg})
				continue
			}
		}
		if msg := rules.Check(f.Rules, f.Kind, raw); msg != "" {
			errs = append(errs, ir.FieldError{Label: f.Label, Message: msg})
			continue
		}
		if raw == "" {
			f.Unset(&m)
			continue
		}
		if err := f.Set(&m, raw); err != nil {
			errs = append(errs, ir.FieldError{Label: f.Label, Message: "Could not read this value."})
		}
	}
	return m, errs
}
