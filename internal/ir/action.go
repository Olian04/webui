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
// selection (a []any of models) for a bulk action. It gates the control at
// render time and authorises the request before Run.
type Action struct {
	Label string
	Role  Role
	Guard func(ctx context.Context, subject any) error
	Run   func(ctx context.Context, subject any) (Outcome, error)
	Bulk  bool
}
