package ir

import (
	"context"
)

// Link navigates. The destination is named, not built: only the runtime knows
// the mount prefix.
type Link struct {
	Dest string // PathTemplate of the target page

	// Args returns the destination page's argument struct for a row. The
	// runtime encodes it through the destination's Page.Encode.
	Args func(ctx context.Context, row any) any
}

func (*Link) isRowTarget() {}
