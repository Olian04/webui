package webui

import (
	"context"
	"fmt"
)

// Target is a resolved href. Err is set when Open cannot produce a URL
// (no runtime, unmounted page, empty path argument). The link renders disabled.
type Target struct {
	URL string
	Err error
}

// Open resolves page+args through the compiled runtime, including the mount
// prefix. Page carries no behaviour; the wrong A fails at compile time.
func Open[A any](ctx context.Context, page Page[A], args A) Target {
	_ = ctx
	_ = args
	return Target{Err: fmt.Errorf("webui: Open %q: no compiled app in this context", page.Path)}
}

// ArgsOf returns the current page's argument struct from ctx.
func ArgsOf[A any](ctx context.Context) (A, error) {
	var zero A
	_ = ctx
	return zero, fmt.Errorf("webui: ArgsOf: no compiled app in this context")
}
