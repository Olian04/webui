package runtime

import (
	"context"
	"io"
	"log/slog"
	"sync"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// part is a part of a page that is built on a goroutine of its own, and is drawn by waiting for it. It is a
// templ.Component, so a layout draws its markup, then each part in order, and a part that is slow holds back
// only what comes after it. What it records for the page (the editors it has) is its own, and is put together
// with the others when they are done.
type part struct {
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

// Render waits for the part, and draws it. What was drawn before it is sent before it is waited for, and what it drew is
// sent when it is done, so a page that is sent as it is built has each part of it as soon as it can. A panic in it, or an
// error, is raised again here, on the goroutine that draws, where the request's own recovery answers it with a 500; once
// the page has begun to be sent it cannot be, so the part says that it could not be built, and is logged.
func (pt *part) Render(ctx context.Context, w io.Writer) error {
	select {
	case <-pt.done:
	default:
		flush(ctx, w)
		select {
		case <-pt.done:
		case <-ctx.Done():
			return ctx.Err() //nolint:wrapcheck // the visitor went away: nothing to say
		}
	}
	if pt.caught != nil || pt.err != nil {
		if streaming(ctx) {
			return pt.failed(ctx, w)
		}
		if pt.caught != nil {
			pt.caught.raise()
		}
		return pt.err
	}
	if err := pt.comp.Render(ctx, w); err != nil {
		return err //nolint:wrapcheck // a component's own error
	}
	flush(ctx, w)
	return nil
}

// failed draws a part that could not be built, in the page that is being sent, and logs the cause.
func (pt *part) failed(ctx context.Context, w io.Writer) error {
	if pt.caught != nil {
		pt.log.Error("webui: a part panicked after the page began to be sent", "page", pt.page, "panic", pt.caught.value, "stack", string(pt.caught.stack))
	} else {
		pt.log.Error("webui: a part could not be built after the page began to be sent", "page", pt.page, "err", pt.err)
	}
	return render.PanelFailed(pt.title).Render(ctx, w) //nolint:wrapcheck // a component's own error
}

// partSet is every part of a page that is being drawn, the nested ones too, so that one wait is for all of them.
type partSet struct {
	mu  sync.Mutex
	all []*part
}

func (s *partSet) add(pt *part) {
	s.mu.Lock()
	s.all = append(s.all, pt)
	s.mu.Unlock()
}

// settle is for every part to be built, those that were started while it waited too.
func (s *partSet) settle() {
	for i := 0; ; i++ {
		s.mu.Lock()
		if i >= len(s.all) {
			s.mu.Unlock()
			break
		}
		pt := s.all[i]
		s.mu.Unlock()
		<-pt.done
	}
}

// wait is settle, and then raises again the panic of the first part that had one, returns the error of the first that
// failed, and records in code what they all need of the page, so that the page has what it would have had in any order.
func (s *partSet) wait(code *render.CodeNeeds) error {
	s.settle()
	s.mu.Lock()
	all := append([]*part(nil), s.all...)
	s.mu.Unlock()
	for _, pt := range all {
		if pt.caught != nil {
			pt.caught.raise()
		}
	}
	var first error
	for _, pt := range all {
		if pt.err != nil && first == nil {
			first = pt.err
		}
		code.Merge(pt.code)
	}
	return first
}

// part starts building n, at once, and returns before it is built. A panic in it is caught where it happens, so
// that it does not take the process down, and raised again where the part is drawn, where the request's own recovery
// answers it.
func (p *Program) part(ctx context.Context, req *Request, page *ir.Page, n ir.Node) *part {
	pt := &part{done: make(chan struct{}), log: p.log, page: page.PathTemplate, title: nodeTitle(n)}
	if req.parts != nil {
		req.parts.add(pt)
	}
	go func() {
		defer close(pt.done)
		defer func() {
			if v := recover(); v != nil {
				pt.caught = newWorkerPanic(v)
			}
		}()
		own := *req // its own copy for what it records
		own.code = render.CodeNeeds{}
		pt.comp, pt.err = p.body(ctx, &own, page, n)
		pt.code = own.code
	}()
	return pt
}

// nodeTitle is what a part is called, for saying that it could not be built; a layout has none.
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
	case *ir.Items:
		return n.Title
	case *ir.Calendar:
		return n.Title
	case *ir.FileUpload:
		return n.Title
	}
	return ""
}
