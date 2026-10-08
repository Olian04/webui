package webui

import (
	"context"

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

	// Guard decides whether the visitor may do this. It receives the action's
	// subject, and the same check disables the button, with its reason shown, and
	// authorises the request, so there is no control that looks available and then
	// fails. A bulk action's subject is the selection.
	Guard func(ctx context.Context, m M) error

	// Run does it, and says how it ended with an [Outcome]. An error is for
	// something the user cannot fix by editing the form: it is logged, and the user
	// sees that something went wrong.
	Run func(ctx context.Context, m M) (Outcome, error)
}

// validateAction checks what every action needs. needLabel is false only for a
// Form's Submit, which has a default.
func validateAction(v *bodyValidator, at, label string, hasRun, needLabel bool) {
	if !hasRun {
		v.add(at+" has no Run", "Set Run, or remove the action.")
	}
	if needLabel && label == "" {
		v.add(at+" has no Label", "Set Label; it is the button text.")
	}
}

// lowerAction wraps the typed closures with the single assertion each needs.
func lowerAction[M any](a Action[M], label string) *ir.Action {
	out := &ir.Action{Label: label, Role: ir.Role(a.Role)}
	if a.Guard != nil {
		out.Guard = func(ctx context.Context, s any) error { return a.Guard(ctx, s.(M)) }
	}
	out.Run = func(ctx context.Context, s any) (ir.Outcome, error) {
		e, err := a.Run(ctx, s.(M))
		if err != nil {
			return ir.Outcome{}, err
		}
		return lowerOutcome[M](e)
	}
	return out
}

// lowerBulk is lowerAction for a selection. The runtime holds rows as []any,
// so the []M the user's func wants is built here, where M is known.
func lowerBulk[M any](a Action[[]M]) *ir.Action {
	rows := func(s any) []M {
		in := s.([]any)
		out := make([]M, len(in))
		for i, r := range in {
			out[i] = r.(M)
		}
		return out
	}
	out := &ir.Action{Label: a.Label, Role: ir.Role(a.Role)}
	if a.Guard != nil {
		out.Guard = func(ctx context.Context, s any) error { return a.Guard(ctx, rows(s)) }
	}
	out.Run = func(ctx context.Context, s any) (ir.Outcome, error) {
		e, err := a.Run(ctx, rows(s))
		if err != nil {
			return ir.Outcome{}, err
		}
		return lowerOutcome[M](e)
	}
	return out
}
