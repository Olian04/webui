package runtime

import (
	"context"
	"strings"
	"sync"

	"github.com/Olian04/webui/internal/ir"
)

// A page's Guard says whether a visitor may open an address, and it is asked for the same address by many parts of one request: by the request for the page itself, by
// the sidebar for the entry of each page, by a breadcrumb, by Cancel and Back, and by every link that leads there, which is drawn only for a visitor who may follow it. A
// table of a hundred rows that lead to three pages asked the Guard of those pages a hundred times, and a Guard that reads a store paid for each; and as they were asked
// at different moments, one that changed its mind in the middle of a request (a session that ended, a role that was taken away) gave a page that disagreed with itself, with a
// link to a page whose entry in the sidebar had been left out, or the other way round.
//
// So a request asks each of them once. The first to ask runs the Guard, and every other part of the request is told what it said.

// verdictCache is what the Guard of each address said in one request. It is shared by the request, the copies of it that the panels are built with, and the in-process
// requests that ask whether a link may be followed.
type verdictCache struct {
	mu sync.Mutex
	m  map[string]*verdict
}

// verdict is one Guard's answer, or the panic that it ended in.
type verdict struct {
	once   sync.Once
	err    error
	caught *workerPanic
}

// verdictKey identifies the Guard call that a request to page makes, which is a call with the arguments that the page's own values decode to. A Guard is not told the address
// beyond them, so what else the address has (a table's sort, where the visitor came from) does not make another call. Decode takes a missing value and an empty one for the same, so
// they are the same here.
func verdictKey(page *ir.Page, raw map[string]string) string {
	var b strings.Builder
	b.WriteString(page.PathTemplate)
	for _, a := range page.Args {
		b.WriteByte(0)
		b.WriteString(raw[a.Name])
	}
	return b.String()
}

// ask runs guard for key if no part of the request has, and says what it said. A Guard that panicked is panicked again for every part that asks, as it was when each ran it, and
// the request that asked first answers it with its own recovery. An answer that came of a visitor who left, or of a deadline that ran out, says nothing of the Guard: it is not
// kept, and the next to ask runs it again.
func (c *verdictCache) ask(ctx context.Context, key string, guard func() error) error {
	c.mu.Lock()
	if c.m == nil {
		c.m = map[string]*verdict{}
	}
	v := c.m[key]
	if v == nil {
		v = &verdict{}
		c.m[key] = v
	}
	c.mu.Unlock()

	v.once.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				v.caught = newWorkerPanic(r)
			}
		}()
		v.err = guard()
	})
	if v.err != nil && ctx.Err() != nil {
		c.mu.Lock()
		if c.m[key] == v {
			delete(c.m, key)
		}
		c.mu.Unlock()
	}
	if v.caught != nil {
		v.caught.raise()
	}
	return v.err
}

// verdictsOf is the cache of the request that ctx belongs to, so that the in-process requests that ask whether a link may be followed share it with the request that
// asked; and a new one for a request that is not inside another.
func verdictsOf(ctx context.Context) *verdictCache {
	if outer := From(ctx); outer != nil && outer.verdicts != nil {
		return outer.verdicts
	}
	return &verdictCache{}
}

// askVerdict is what the Guard of page says of the address whose own values are raw, asked once per request when ctx belongs to one, and every time when it does not (a request
// that is not for a page).
func askVerdict(ctx context.Context, page *ir.Page, raw map[string]string, guard func() error) error {
	if req := From(ctx); req != nil && req.verdicts != nil {
		return req.verdicts.ask(ctx, verdictKey(page, raw), guard)
	}
	return guard()
}

// rawOf is the values of an address, as a request to it would be read: the path's and the query's by the name of the argument.
func rawOf(path, query map[string]string) map[string]string {
	raw := make(map[string]string, len(path)+len(query))
	for k, v := range path {
		raw[k] = v
	}
	for k, v := range query {
		raw[k] = v
	}
	return raw
}
