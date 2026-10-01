package ir

import (
	"context"
)

// Link navigates. The destination is named, not built: only the runtime knows
// the mount prefix.
type Link struct {
	Dest  string // PathTemplate of the target page
	Args  func(ctx context.Context, row any) map[string]string
	Guard func(ctx context.Context, row any) error
}

func (*Link) isRowTarget() {}
