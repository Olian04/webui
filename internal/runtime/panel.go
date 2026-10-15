package runtime

import (
	"context"
	"io"
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
}

// Render waits for the panel, and draws it. A panic in it is raised again here, on the goroutine that draws.
func (pn *panel) Render(ctx context.Context, w io.Writer) error {
	select {
	case <-pn.done:
	case <-ctx.Done():
		return ctx.Err() //nolint:wrapcheck // the visitor went away: nothing to say
	}
	if pn.caught != nil {
		pn.caught.raise()
	}
	if pn.err != nil {
		return pn.err
	}
	return pn.comp.Render(ctx, w) //nolint:wrapcheck // a component's own error
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

// wait is for every panel to be built, those that were started while it waited too. It raises again the panic of
// the first that had one, returns the error of the first that failed, and records in code what they all need of the
// page, so that the page has what it would have had in any order.
func (s *panelSet) wait(code *render.CodeNeeds) error {
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
	pn := &panel{done: make(chan struct{})}
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
