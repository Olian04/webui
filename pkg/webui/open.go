package webui

import (
	"context"
	"fmt"

	"github.com/Olian04/webui/internal/runtime"
)

// Target is where [Open] says a page is, to hand to [Outcome.Then]. It has no
// parts to read: when Open cannot resolve the page (no compiled app, a page that is
// not mounted, an empty path argument) Then reports why and the outcome does what
// it would have done without it.
type Target struct {
	url string // the address, including the mount prefix
	err error  // why there is no address
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
		return Target{err: fmt.Errorf("webui: Open %q: no compiled app in this context", path)}
	}
	url, err := req.Open(path, args)
	if err != nil {
		return Target{err: fmt.Errorf("webui: Open %q: %w", path, err)}
	}
	return Target{url: url}
}

// ArgsOf returns the current page's argument struct from ctx.
//
// It panics when ctx did not come from a page of a compiled app, or when A is not
// the page's argument type. Both are mistakes in the declaration and neither is
// something a caller can handle, so there is no error to check: the library
// recovers the panic, logs its cause, and answers that request with a 500.
func ArgsOf[A any](ctx context.Context) A {
	req := runtime.From(ctx)
	if req == nil {
		panic("webui: ArgsOf: no compiled app in this context")
	}
	a, ok := req.Args.(A)
	if !ok {
		panic(fmt.Sprintf("webui: ArgsOf: the current page's arguments are %T, not %T", req.Args, a))
	}
	return a
}
