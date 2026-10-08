package webui

import (
	"context"
	"fmt"

	"github.com/Olian04/webui/internal/runtime"
)

// Target is a resolved href. Err is set when Open cannot produce a URL
// (no runtime, unmounted page, empty path argument). The link renders disabled.
type Target struct {
	URL string
	Err error
}

// Open resolves page+args through the compiled runtime, including the mount
// prefix. page is a Page or its PageID; the wrong A fails at compile time.
//
// It resolves through the runtime rather than the declaration, so a page that
// was never mounted — or whose declaration was mutated after Compile — is
// reported instead of silently producing a dead link.
func Open[A any](ctx context.Context, page PageRef[A], args A) Target {
	path := page.pagePath()
	req := runtime.From(ctx)
	if req == nil {
		return Target{Err: fmt.Errorf("webui: Open %q: no compiled app in this context", path)}
	}
	url, err := req.Open(path, args)
	if err != nil {
		return Target{Err: fmt.Errorf("webui: Open %q: %w", path, err)}
	}
	return Target{URL: url}
}

// ArgsOf returns the current page's argument struct from ctx.
func ArgsOf[A any](ctx context.Context) (A, error) {
	var zero A
	req := runtime.From(ctx)
	if req == nil {
		return zero, fmt.Errorf("webui: ArgsOf: no compiled app in this context")
	}
	a, ok := req.Args.(A)
	if !ok {
		return zero, fmt.Errorf("webui: ArgsOf: the current page's arguments are %T, not %T", req.Args, zero)
	}
	return a, nil
}
