package runtime

import (
	"compress/gzip"
	"net/http"
	"slices"
	"sync"

	"github.com/Olian04/webui/internal/render"
)

var gzipPool = sync.Pool{New: func() any { return gzip.NewWriter(nil) }}

// compressed gzips a response when the client accepts it and the content is text,
// JSON or SVG: the pages and the search, which are built per request. The assets are
// compressed once, when they are built, and never reach this. The decision is made
// when the status is written, from the content type the handler set, so a response
// that is already encoded, has no body, or is an image or font is left alone.
func compressed(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead || !render.AcceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzipWriter{ResponseWriter: w}
		defer gw.finish()
		next.ServeHTTP(gw, r)
	})
}

type gzipWriter struct {
	http.ResponseWriter
	zw      *gzip.Writer
	decided bool
}

func (g *gzipWriter) decide(status int) {
	g.decided = true
	h := g.Header()
	if !slices.Contains(h.Values("Vary"), "Accept-Encoding") {
		h.Add("Vary", "Accept-Encoding")
	}
	if status < 200 || status == http.StatusNoContent || status == http.StatusNotModified || (status >= 300 && status < 400) {
		return
	}
	if h.Get("Content-Encoding") != "" || !render.Compressible(h.Get("Content-Type")) {
		return
	}
	h.Set("Content-Encoding", "gzip")
	h.Del("Content-Length") // the length of what is sent is not the length of what was written
	g.zw = gzipPool.Get().(*gzip.Writer)
	g.zw.Reset(g.ResponseWriter)
}

func (g *gzipWriter) WriteHeader(status int) {
	if !g.decided {
		g.decide(status)
	}
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	if !g.decided {
		g.decide(http.StatusOK)
	}
	if g.zw != nil {
		return g.zw.Write(b) //nolint:wrapcheck // a pass-through
	}
	return g.ResponseWriter.Write(b) //nolint:wrapcheck // a pass-through
}

// finish ends the compressed stream and gives the writer back.
func (g *gzipWriter) finish() {
	if g.zw == nil {
		return
	}
	_ = g.zw.Close()
	g.zw.Reset(nil)
	gzipPool.Put(g.zw)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (g *gzipWriter) Unwrap() http.ResponseWriter { return g.ResponseWriter }
