package main

import (
	"context"
	"errors"
	"flag"
)

// viewer makes everyone a viewer, so the guarded controls can be seen disabled,
// each with its reason. A real app would read the caller from ctx.
var viewer = flag.Bool("viewer", false, "serve as a viewer: guarded controls are disabled")

// canEdit is the Guard of every action. One function serves them all: it takes
// the action's subject and ignores it.
func canEdit[T any](context.Context, T) error {
	if *viewer {
		return errors.New("requires the editor role")
	}
	return nil
}
