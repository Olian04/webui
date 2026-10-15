package runtime

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/Olian04/webui/internal/ir"
)

// streamAfter is how long a page waits for all of its panels before it is sent as it is built. A page whose panels
// answer within it is built whole and sent in one piece, as it has always been: the same bytes, a real status if
// something fails, no flush. Only a page with a panel that is slower is sent early, with the part of it that is
// ready, and the panels after it, in order, as they are.
const streamAfter = 50 * time.Millisecond

// streamable is whether a page may be sent before its panels are built: nothing in the head of the page depends on
// them. An editor, a diff and Markdown do (the head links their script and their stylesheet), so a page that has one
// is built whole. A layout that has one on a tab that is not selected is built whole too: the page is not asked.
func streamable(body ir.Node) bool {
	ok := true
	ir.Walk(body, func(n ir.Node) {
		switch n := n.(type) {
		case *ir.Form:
			ok = ok && n.Editor == nil
		case *ir.Diff, *ir.Markdown:
			ok = false
		}
	})
	return ok
}

// writeBuilt answers with a page whose panels have been started: whole when they are all done within streamAfter, and
// as they are built when they are not and the page may be.
func (p *Program) writeBuilt(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, body *panel, mayStream bool) {
	if mayStream && r.Method == http.MethodGet {
		ready := make(chan struct{})
		go func() { req.panels.settle(); close(ready) }()
		timer := time.NewTimer(streamAfter)
		defer timer.Stop()
		select {
		case <-ready:
		case <-r.Context().Done():
			return
		case <-timer.C:
			p.stream(w, r, page, req, body)
			return
		}
	}
	if err := req.panels.wait(&req.code); err != nil {
		p.fail(w, r, page, req, err)
		return
	}
	p.writePage(w, r, http.StatusOK, page, req, body)
}

// stream sends the page as it is built: the headers and everything up to the first panel that is not ready, at once,
// and then each panel as it is done, in the order the page has them. The status is the page's, so it is decided
// already: what the Guard refused or the address did not decode never gets here. After the first byte nothing can
// make it an error page, so a panel that fails says so where it is (see [panel.Render]).
func (p *Program) stream(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, body *panel) {
	ctx := context.WithValue(r.Context(), streamingKey{}, true)
	doc := p.finish(w, r, p.pageDoc(page, req, body), req.shell)
	started := startedAt(r)
	// The headers say how long it took to send the first of it. The bar, which is drawn last, says how long all of it took.
	doc.Timing = func() (time.Time, time.Duration) { return time.Now().UTC(), time.Since(started) }

	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("X-Accel-Buffering", "no") // a reverse proxy that buffers a response would hold the page back
	w.WriteHeader(http.StatusOK)
	if err := p.render.Document(doc).Render(ctx, &flushWriter{w: w, rc: http.NewResponseController(w)}); err != nil && ctx.Err() == nil {
		p.log.Error("webui: render failed", "path", r.URL.Path, "err", err)
	}
}

// flushWriter is what a page that is sent as it is built is drawn into: the response, which flushes when the drawing
// asks it to, through the writers that wrap it (the compression's among them).
type flushWriter struct {
	w  io.Writer
	rc *http.ResponseController
}

func (f *flushWriter) Write(b []byte) (int, error) { return f.w.Write(b) } //nolint:wrapcheck // a pass-through

// Flush is http.Flusher, which is what the buffer of a drawing looks for under it.
func (f *flushWriter) Flush() { _ = f.rc.Flush() }
