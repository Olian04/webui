package runtime

import (
	"context"
	"io"
	"log/slog"
	"runtime/debug"
	"sync"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// panel is a part of a page that is built on a goroutine of its own, and is drawn by waiting for it. It is a
// templ.Component, so a layout draws its markup, then each panel in order, and a panel that is slow holds back
// only what comes after it. What it records for the page (the editors it has) is its own, and is put together
// with the others when they are done.
type panel struct {
	done   chan struct{}
	comp   templ.Component
	err    error
	code   render.CodeNeeds
	caught *workerPanic

	// what it takes to say, once the page has begun to be sent, that it could not be built
	log   *slog.Logger
	page  string
	title string
}

// streamingKey is in the context of a page that is being sent as it is built.
type streamingKey struct{}

func streaming(ctx context.Context) bool { return ctx.Value(streamingKey{}) != nil }

// flush sends what has been drawn so far, to a client that is being sent the page as it is built. A writer that
// cannot flush, which is the buffer of a page that is sent whole, has nothing to do.
func flush(ctx context.Context, w io.Writer) {
	if !streaming(ctx) {
		return
	}
	if f, ok := w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
}

// Render waits for the panel, and draws it. What was drawn before it is sent before it is waited for, and what it drew is
// sent when it is done, so a page that is sent as it is built has each part of it as soon as it can. A panic in it, or an
// error, is raised again here, on the goroutine that draws, where the request's own recovery answers it with a 500; once
// the page has begun to be sent it cannot be, so the panel says that it could not be built, and is logged.
func (pn *panel) Render(ctx context.Context, w io.Writer) error {
	select {
	case <-pn.done:
	default:
		flush(ctx, w)
		select {
		case <-pn.done:
		case <-ctx.Done():
			return ctx.Err() //nolint:wrapcheck // the visitor went away: nothing to say
		}
	}
	if pn.caught != nil || pn.err != nil {
		if streaming(ctx) {
			return pn.failed(ctx, w)
		}
		if pn.caught != nil {
			pn.caught.raise()
		}
		return pn.err
	}
	if err := pn.comp.Render(ctx, w); err != nil {
		return err //nolint:wrapcheck // a component's own error
	}
	flush(ctx, w)
	return nil
}

// failed draws a panel that could not be built, in the page that is being sent, and logs the cause.
func (pn *panel) failed(ctx context.Context, w io.Writer) error {
	if pn.caught != nil {
		pn.log.Error("webui: a panel panicked after the page began to be sent", "page", pn.page, "panic", pn.caught.value, "stack", string(pn.caught.stack))
	} else {
		pn.log.Error("webui: a panel could not be built after the page began to be sent", "page", pn.page, "err", pn.err)
	}
	return render.PanelFailed(pn.title).Render(ctx, w) //nolint:wrapcheck // a component's own error
}

// panelSet is every panel of a page that is being drawn, the nested ones too, so that one wait is for all of them.
type panelSet struct {
	mu  sync.Mutex
	all []*panel
}

func (s *panelSet) add(pn *panel) {
	s.mu.Lock()
	s.all = append(s.all, pn)
	s.mu.Unlock()
}

// settle is for every panel to be built, those that were started while it waited too.
func (s *panelSet) settle() {
	for i := 0; ; i++ {
		s.mu.Lock()
		if i >= len(s.all) {
			s.mu.Unlock()
			break
		}
		pn := s.all[i]
		s.mu.Unlock()
		<-pn.done
	}
}

// wait is settle, and then raises again the panic of the first panel that had one, returns the error of the first that
// failed, and records in code what they all need of the page, so that the page has what it would have had in any order.
func (s *panelSet) wait(code *render.CodeNeeds) error {
	s.settle()
	s.mu.Lock()
	all := append([]*panel(nil), s.all...)
	s.mu.Unlock()
	for _, pn := range all {
		if pn.caught != nil {
			pn.caught.raise()
		}
	}
	var first error
	for _, pn := range all {
		if pn.err != nil && first == nil {
			first = pn.err
		}
		code.Merge(pn.code)
	}
	return first
}

// panel starts building n, at once, and returns before it is built. A panic in it is caught where it happens, so
// that it does not take the process down, and raised again where the panel is drawn, where the request's own recovery
// answers it.
func (p *Program) panel(ctx context.Context, req *Request, page *ir.Page, n ir.Node) *panel {
	pn := &panel{done: make(chan struct{}), log: p.log, page: page.PathTemplate, title: nodeTitle(n)}
	if req.panels != nil {
		req.panels.add(pn)
	}
	go func() {
		defer close(pn.done)
		defer func() {
			if v := recover(); v != nil {
				pn.caught = &workerPanic{value: v, stack: debug.Stack()}
			}
		}()
		own := *req // its own copy for what it records
		own.code = render.CodeNeeds{}
		pn.comp, pn.err = p.body(ctx, &own, page, n)
		pn.code = own.code
	}()
	return pn
}

// nodeTitle is what a panel is called, for saying that it could not be built; a layout has none.
func nodeTitle(n ir.Node) string {
	switch n := n.(type) {
	case *ir.Table:
		return n.Title
	case *ir.Form:
		return n.Title
	case *ir.Diff:
		return n.Title
	case *ir.Markdown:
		return n.Title
	}
	return ""
}
