package webui

import (
	"context"
	"fmt"

	"github.com/Olian04/webui/internal/ir"
)

// Role decides an action's button style. Secondary is the zero value: a page
// may have no primary action, but it may not have two.
type Role uint8

// The roles.
const (
	RoleSecondary Role = iota
	RolePrimary
	RoleDestructive
)

// Action is a mutation of M. Guard enables the control and authorises Run.
// Label is the button text; a Form's Submit defaults to "Save".
type Action[M any] struct {
	Label string
	Role  Role
	Guard func(ctx context.Context, m M) error
	Run   func(ctx context.Context, m M) (Effect, error)
}

// Effect is the outcome of an understood action. A nil error on Run plus
// Fields is a validation rejection the user can fix. Redirect navigates.
//
// Effect is a request-time value, not part of the declaration, so Redirect
// holds a resolved Target: build it with Open, which has ctx and therefore the
// runtime and the mount prefix.
type Effect struct {
	Toast    string
	Fields   FieldErrors
	Redirect Target
}

// FieldErrors is Fields[M] with the model erased so Effect stays non-generic.
type FieldErrors interface {
	isFieldErrors()
}

// Fields is a list of per-accessor messages for model M.
type Fields[M any] []FieldError[M]

func (Fields[M]) isFieldErrors() {}

// FieldError is one accessor's rejection message. Use keyed literals.
type FieldError[M any] struct {
	Field   Accessor[M]
	Message string
}

// ASSERT: Fields implements FieldErrors
var _ FieldErrors = Fields[struct{}](nil)

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
	out.Run = func(ctx context.Context, s any) (ir.Effect, error) {
		e, err := a.Run(ctx, s.(M))
		if err != nil {
			return ir.Effect{}, err
		}
		return lowerEffect[M](e)
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
	out := &ir.Action{Label: a.Label, Role: ir.Role(a.Role), Bulk: true}
	if a.Guard != nil {
		out.Guard = func(ctx context.Context, s any) error { return a.Guard(ctx, rows(s)) }
	}
	out.Run = func(ctx context.Context, s any) (ir.Effect, error) {
		e, err := a.Run(ctx, rows(s))
		if err != nil {
			return ir.Effect{}, err
		}
		return lowerEffect[M](e)
	}
	return out
}

// lowerEffect converts the request-time Effect. A Redirect whose Open failed,
// or Fields for a different model, are bugs in the user's Run, reported as an
// error rather than silently dropped.
func lowerEffect[M any](e Effect) (ir.Effect, error) {
	out := ir.Effect{Toast: e.Toast}
	if e.Redirect.Err != nil {
		return ir.Effect{}, fmt.Errorf("webui: Effect.Redirect: %w", e.Redirect.Err)
	}
	out.Redirect = e.Redirect.URL
	switch f := e.Fields.(type) {
	case nil:
	case Fields[M]:
		for _, fe := range f {
			out.Fields = append(out.Fields, ir.FieldError{Label: accessorLabel[M](fe.Field), Message: fe.Message})
		}
	default:
		return ir.Effect{}, fmt.Errorf("webui: Effect.Fields is %T, not the action's model", e.Fields)
	}
	return out, nil
}
