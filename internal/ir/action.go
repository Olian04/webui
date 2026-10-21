package ir

import (
	"context"
)

// Role decides an action's button style. Secondary is the zero value: a page
// may have no primary action, but it may not have two.
type Role uint8

// The roles. Destructive takes the critical colour.
const (
	RoleSecondary Role = iota
	RolePrimary
	RoleDestructive
)

// Action is a mutation. Guard receives the subject: one model, or the
// selection (a []any of models) for a bulk action. It decides whether the control is drawn
// at render time, and authorises the request before Run.
type Action struct {
	Label string
	Role  Role

	// Confirm is a question to be answered yes to before the action is done, "" for none.
	Confirm string

	Guard func(ctx context.Context, subject any) error
	Run   func(ctx context.Context, subject any) (Outcome, error)
}

// Disabled is the error a Guard returns to say that its control is to be drawn, disabled, with Message to say why, in place of
// not being drawn, which is what any other error from a Guard says. Either way the Guard refuses the request.
type Disabled struct{ Message string }

func (d *Disabled) Error() string { return d.Message }
