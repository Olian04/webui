package ir

import (
	"context"
)

// Action is a mutation. Guard receives the subject: one model, or the
// selection for a bulk action. It gates the control at render time and
// authorises the request before Run.
type Action struct {
	Label string
	Guard func(ctx context.Context, subject any) error
	Run   func(ctx context.Context, subject any) (Effect, error)
	Bulk  bool
}

func (*Action) isRowTarget() {}
