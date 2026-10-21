package runtime

import (
	"context"
	"runtime/debug"
	"sync"

	"github.com/Olian04/webui/internal/render"
)

// shellInputs are what the page's sidebar and brand need from the app, and that nothing in the page's panels
// has a say in: which navigation entries the visitor may not open (the Guard of each page, asked bare), where
// the brand leads (more of the same), and the entries that pages list (each page's NavEntries, with a Guard
// for each). They are slow when the app's Guards and entries are, and they were asked one after the other once
// the panels were all loaded; asked here, beside the panels, a page waits for the slowest of them.
type shellInputs struct {
	wg     sync.WaitGroup
	hide   map[string]bool
	home   string
	links  map[string][]render.NavLink
	caught [2]*workerPanic
}

// startShell asks for them, at once, and returns before any is answered. The entries that pages list do not
// depend on which entries are hidden, so they are asked beside the hidden ones, and where the brand leads, which does.
func (p *Program) startShell(ctx context.Context, here render.Here, active string) *shellInputs {
	s := &shellInputs{}
	run := func(i int, f func()) {
		s.wg.Go(func() {
			defer func() {
				if v := recover(); v != nil {
					s.caught[i] = &workerPanic{value: v, stack: debug.Stack()}
				}
			}()
			f()
		})
	}
	run(0, func() {
		s.hide = p.hiddenNav(ctx)
		s.home = p.start(ctx, s.hide)
	})
	run(1, func() { s.links = p.navLinks(ctx, here, active) })
	return s
}

// join is for them to have finished, and nothing else: a panic in one is not raised. It is how a request that ends before it asked for
// their answers (it panicked, or failed) leaves none of them running past its end, writing to a logger the program may have closed.
func (s *shellInputs) join() { s.wg.Wait() }

// wait is for them to be answered. A panic in one is raised again here, on the request's own goroutine, where
// the recovery of the request answers it, as when they were asked from there.
func (s *shellInputs) wait() {
	s.wg.Wait()
	for _, c := range s.caught {
		if c != nil {
			c.raise()
		}
	}
}
