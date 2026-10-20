package webui

import (
	"context"

	"github.com/Olian04/webui/internal/convert"
	"github.com/Olian04/webui/internal/ir"
)

// Role decides an action's button style. Secondary is the zero value: a page
// may have no primary action, but it may not have two.
type Role uint8

// The roles.
const (
	// RoleSecondary is an ordinary button.
	RoleSecondary Role = iota

	// RolePrimary is the one action the page wants done, in the accent colour. A
	// form's Submit is primary by default.
	RolePrimary

	// RoleDestructive is an action that cannot be taken back, in the critical colour.
	RoleDestructive
)

// Action is a mutation of M. Guard enables the control and authorises Run.
// Label is the button text; a Form's Submit defaults to "Save". Run says how it
// ended with an Outcome (Success, Warning, Failure or Reject), or an error for
// something the user cannot fix.
type Action[M any] struct {
	// Label is the button's text. A form's Submit may leave it empty.
	Label string

	// Role is the button's style.
	Role Role

	// Confirm is a question that the visitor must answer before the action is done, such as
	// "Delete this device? This cannot be undone." The button opens a dialog that says it, with the action's
	// Label to go on and Cancel to leave; nothing is sent until they go on. It is a native <dialog> opened
	// by the button, which needs no script in a current browser (Chrome and Edge 135, Firefox 144, Safari 26.2); script
	// supplies the same for an older one, and an older one with no script cannot open it, and so cannot
	// do the action. It asks the visitor and no one else: a request made by hand is not asked, and an action
	// that must not happen is refused by its Guard. A form that has one is not submitted by pressing Enter in a
	// field, which would not ask. A button that is refused by its Guard is disabled with its reason, and has no
	// dialog. It cannot be set on a Table's RowClick, which has no button.
	Confirm string

	// Guard decides whether the visitor may do this. It receives the action's
	// subject, and the same check disables the button, with its reason shown, and
	// authorises the request, so there is no control that looks available and then
	// fails. A bulk action's subject is the selection. A form's Submit that the
	// visitor is refused is not drawn at all: the form is shown read-only, with a
	// Back button in place of Save and Cancel.
	Guard func(ctx context.Context, m M) error

	// Run does it, and says how it ended with an [Outcome]. An error is for
	// something the user cannot fix by editing the form: it is logged, and the user
	// sees that something went wrong.
	Run func(ctx context.Context, m M) (Outcome, error)
}

// validateAction checks what every action needs. needLabel is false only for a
// Form's Submit and a row's click, which have a default or no button.
func validateAction[S any](v *bodyValidator, at string, a Action[S], needLabel bool) {
	if a.Run == nil {
		v.add(at+" has no Run", "Set Run, or remove the action.")
	}
	if needLabel && a.Label == "" {
		v.add(at+" has no Label", "Set Label; it is the button text.")
	}
}

// lowerActionWith wraps the typed closures with the single assertion each needs. arg turns the
// runtime's subject into what the user's funcs take, and M is the model a Reject is about.
func lowerActionWith[S, M any](a Action[S], arg func(any) S) *ir.Action {
	out := &ir.Action{Label: a.Label, Role: ir.Role(a.Role), Confirm: a.Confirm}
	if a.Guard != nil {
		out.Guard = func(ctx context.Context, s any) error { return a.Guard(ctx, arg(s)) }
	}
	out.Run = func(ctx context.Context, s any) (ir.Outcome, error) {
		e, err := a.Run(ctx, arg(s))
		if err != nil {
			return ir.Outcome{}, err
		}
		return lowerOutcome[M](e)
	}
	return out
}

func (Action[M]) toolbarItem(M) {}

func (a Action[M]) validateButton(v *bodyValidator, at string) { validateAction(v, at, a, true) }

func (a Action[M]) toolbarButton() ir.ToolbarItem {
	return ir.ToolbarItem{Label: a.Label, Role: ir.Role(a.Role), Action: lowerAction(a)}
}

func lowerAction[M any](a Action[M]) *ir.Action {
	return lowerActionWith[M, M](a, func(s any) M { return s.(M) })
}

// lowerBulk is lowerAction for a selection. The runtime holds rows as []any,
// so the []M the user's func wants is built here, where M is known.
func lowerBulk[M any](a Action[[]M]) *ir.Action {
	return lowerActionWith[[]M, M](a, func(s any) []M { return convert.Slice(s.([]any), func(r any) M { return r.(M) }) })
}

// validateRow checks an Action used as a table's RowClick: it has a Run, and no
// Label is needed, since the row is what is clicked.
func (a Action[M]) validateRow(v *bodyValidator) {
	validateAction(v, "Table.RowClick", a, false)
	if a.Confirm != "" {
		v.add("Table.RowClick has a Confirm, but a click on a row has no button to ask from",
			"Remove Confirm, or make the action one of Table.Actions, which has a button for each row.")
	}
}

func (a Action[M]) lowerRow() (*ir.Link, *ir.Action) { return nil, lowerAction(a) }
