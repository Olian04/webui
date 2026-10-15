package runtime

import (
	"context"
	"time"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// Request is what the compiled app knows about the request in flight. It rides
// in ctx — the gateway between the declaration's closures and the runtime — so
// anything that touches the runtime takes ctx, and anything that is a pure
// function of the model does not.
type Request struct {
	program *Program

	// Args is the current page's decoded argument struct.
	Args any

	// Address is the request's path and query as the browser sent them, or empty
	// when the request is not for a page (the search endpoint).
	Address string

	// From is the address of the page that sent the user here, when a link the
	// library built said so and it is an address in this app. Empty otherwise.
	From string

	// Raw is the same values by argument name, path and query merged, plus the
	// view state the library keeps for tables and tabs ("devices.offset").
	// Argument names cannot contain the separator, so the two never collide.
	Raw map[string]string

	// sub is a rejected submission being rendered back, and subLeaf the leaf
	// it belongs to. Both are set only while answering a POST.
	sub     *submission
	subLeaf string

	// code is what the editors and diffs built for this response need the page to
	// load, set as the body is built.
	code render.CodeNeeds

	// shell is what the page's sidebar needs, asked for beside the panels, for a page that is
	// being drawn.
	shell *shellInputs
}

// reqCtx is ctx with a request for the page's arguments in it, for the application's funcs the runtime
// calls on its own: a Guard, a search, the entries of the sidebar.
func (p *Program) reqCtx(ctx context.Context, args any, raw map[string]string) context.Context {
	return With(ctx, &Request{program: p, Args: args, Raw: raw})
}

type ctxKey struct{}

// With returns ctx carrying req. The key is an unexported type: a string key
// would collide with any other package using the same string, and go vet does
// not catch it.
func With(ctx context.Context, req *Request) context.Context {
	return context.WithValue(ctx, ctxKey{}, req)
}

// From returns the request carried by ctx, or nil when ctx did not come from a
// compiled app.
func From(ctx context.Context) *Request {
	req, _ := ctx.Value(ctxKey{}).(*Request)
	return req
}

// encode is the page's current address as path and query values: the argument
// struct encoded, plus the view state in the address. Every link, form action
// and redirect back to the page is built from it, so none drops a table's sort
// or the selected tab.
func (r *Request) encode(page *ir.Page) (path, query map[string]string) {
	path, query, err := page.Encode(r.Args)
	if err != nil {
		path, query = map[string]string{}, map[string]string{}
	}
	for k, v := range r.Raw {
		if args.IsViewKey(k) && v != "" {
			query[k] = v
		}
	}
	return path, query
}

// timed runs an application's Load, and counts and times it in the metrics, under the page and the kind
// of panel it is for.
func timed[T any](p *Program, page *ir.Page, kind string, load func() (T, error)) (T, error) {
	started := time.Now()
	v, err := load()
	p.m.Load(page.PathTemplate, kind, started, err)
	return v, err
}

// loaded is timed for a panel that is being drawn: a failure is logged, unless the visitor had gone, and
// ok is false, so that the panel says it could not load and the page stays. what names the panel in the log.
func loaded[T any](p *Program, ctx context.Context, page *ir.Page, kind, what string, at ir.Addr, load func() (T, error)) (v T, ok bool) {
	v, err := timed(p, page, kind, load)
	if err != nil {
		if ctx.Err() == nil {
			p.log.Error("webui: "+what+" load failed", "page", page.PathTemplate, "leaf", render.LeafID(at), "err", err)
		}
		return v, false
	}
	return v, true
}
