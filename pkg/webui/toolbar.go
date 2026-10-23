package webui

import (
	"fmt"
	"strconv"

	"github.com/Olian04/webui/internal/ir"
)

// Toolbar is a leaf that is a row of controls for the page it is on: buttons that do something or lead somewhere, and the values of the
// models that its forms load, to read. It has no type parameter, because it has no model of its own: what it shows is in its Forms, each of
// which loads one, and its buttons are about the page, which they read from the request as any Load does.
//
//	webui.Toolbar{
//		Left: []webui.ToolbarItem{
//			webui.Link[webui.NoArgs, NewDeviceArgs]{Label: "Create device", Role: webui.RolePrimary, Page: NewDevice, Args: ...},
//			webui.Action[webui.NoArgs]{Label: "Sync now", Guard: canEdit, Run: syncNow},
//		},
//		Right: []webui.ToolbarItem{
//			webui.Form[Collector]{Title: "System", Load: loadCollector, Fields: []webui.Accessor[Collector]{Health, Version, Uptime}},
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
//   - An [Action] of [NoArgs] is a button that does something. It posts, and its Run says how it ended with an [Outcome], as
//     any action does. It has no subject: what it needs of the page it reads with [ArgsOf]. Its Guard decides whether the visitor may: an
//     error leaves the button out, and [Disabled] draws it disabled, with its message.
//   - A [Link] of [NoArgs] is a button that goes to another page, with a Label and a Role. It is a real anchor, and is left out for a
//     visitor whom the page it leads to refuses. Its Args are built from the request, with [ArgsOf] for the page's.
//   - A [Form] shows the values of the model that its Load returns, read-only, as a read-only form field shows them: each field's Label and
//     its value, in a row, under the Form's Title, which names the group (a [Select] as its badges, a number as a number, with its Format). It
//     is for the state of something, such as the health of a service, and a Load that returns [Loaded.RefreshAfter] has the whole toolbar asked for
//     again, as a panel is. A Form in a toolbar has no Submit, no Desc and no Help, and no field that is multiline, secret or a file: a toolbar
//     has no inputs, and a [Form] panel is for editing. Several Forms of different models may share a toolbar.
//
// A toolbar is not a panel: it has no frame, so it sits on the page, in a [Stack] with what it is for. One whose controls are all
// left out for the visitor is not drawn.
type Toolbar struct {
	// Left are the controls at the left edge, in order.
	Left []ToolbarItem

	// Right are the controls at the right edge, in order. A toolbar with nothing in either is a mistake.
	Right []ToolbarItem
}

// items are the controls, the left ones first, with where the right ones start.
func (t Toolbar) items() []ToolbarItem {
	return append(append([]ToolbarItem(nil), t.Left...), t.Right...)
}

// ToolbarItem is a control of a [Toolbar]: an [Action] or a [Link] of [NoArgs], or a [Form]. Only this package implements it, and an
// Action or a Link that has a subject of its own is not one, which is a compile error: a toolbar has no model to give it.
type ToolbarItem interface {
	toolbarItem(NoArgs)
}

// toolbarButton is what an [Action] and a [Link] are to a toolbar.
type toolbarButton interface {
	validateButton(v *bodyValidator, at string)
	toolbarButton() ir.ToolbarItem
}

// toolbarForm is what a [Form] is to a toolbar: a model to load and show.
type toolbarForm interface {
	validateToolbarForm(v *bodyValidator, at string, seen map[string]bool)
	lowerToolbarForm(at ir.Addr, prefix string) ir.ToolbarItem
}

// ASSERT: Toolbar implements PageBody
var _ PageBody = Toolbar{}

func (t Toolbar) validateBody(v *bodyValidator) {
	if len(t.Left)+len(t.Right) == 0 {
		v.add("a Toolbar has no controls", "Put the controls it is for in Left or in Right: an Action, a Link, or a Form to show.")
	}
	seen := map[string]bool{}
	for i, item := range t.items() {
		at := fmt.Sprintf("Toolbar.Left[%d]", i)
		if i >= len(t.Left) {
			at = fmt.Sprintf("Toolbar.Right[%d]", i-len(t.Left))
		}
		switch it := item.(type) {
		case toolbarButton:
			it.validateButton(v, at)
		case toolbarForm:
			it.validateToolbarForm(v, at, seen)
		case nil:
			v.add(at+" is nil", "Remove it or declare a control.")
		}
	}
}

func (t Toolbar) lowerBody(at ir.Addr) ir.Node {
	out := &ir.Toolbar{At: at, RightFrom: len(t.Left)}
	for i, item := range t.items() {
		switch it := item.(type) {
		case toolbarButton:
			out.Items = append(out.Items, it.toolbarButton())
		case toolbarForm:
			out.Items = append(out.Items, it.lowerToolbarForm(at, "t"+strconv.Itoa(i)+"_"))
		}
	}
	return out
}

func (f Form[M]) toolbarItem(NoArgs) {}

func (f Form[M]) validateToolbarForm(v *bodyValidator, at string, seen map[string]bool) {
	if f.Title == "" {
		v.add(at+" is a Form with no Title", "Set Title: in a toolbar it names the group of values, and is what is said when they cannot be loaded.")
	}
	needLoad(v, at+" is a Form that", f.Load != nil)
	if f.Submit.Run != nil || f.Submit.Guard != nil || f.Submit.Label != "" {
		v.add(at+" is a Form with a Submit", "A toolbar has no inputs: remove Submit, or put the Form on the page as a panel.")
	}
	for i, field := range f.Fields {
		validateAccessor[M](v, field, fmt.Sprintf("%s.Fields[%d]", at, i), accessorSite{where: "Toolbar", allowGroup: true, uniqueLbl: true}, seen)
	}
	if len(f.Fields) == 0 {
		v.add(at+" is a Form with no Fields", "Set Fields to the values to show.")
	}
}

func (f Form[M]) lowerToolbarForm(at ir.Addr, prefix string) ir.ToolbarItem {
	return ir.ToolbarItem{Form: &ir.Form{At: at, Title: f.Title, Fields: lowerAccessors(f.Fields, prefix), Load: eraseLoad(f.Load)}}
}
