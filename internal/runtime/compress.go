package runtime

import (
	"compress/gzip"
	"io"
	"net/http"
	"slices"
	"sync"

	"github.com/Olian04/webui/internal/assetserve"
)

// A response that is built for one request is compressed with gzip, even for a client that takes Brotli. Measured on a 64 KB page, gzip's default takes 0.9 ms and makes 11.5 KB,
// Brotli at level 4 takes 1.6 ms for 11.1 KB and at level 5 2.5 ms for 10.3 KB; the page itself takes about 1 ms to build, so the better compression was more CPU than the page, for a
// tenth of its bytes. The files that are compressed once (the stylesheets, the scripts, the editor) are Brotli at the best level, which is where the bytes are: see assetserve.

// encoder is what a response is compressed with.
type encoder interface {
	io.WriteCloser
	Flush() error
	Reset(io.Writer)
}

var gzipPool = sync.Pool{New: func() any { return gzip.NewWriter(nil) }}

// compressed gzips a response when the client takes it and the content is text, JSON or SVG: the pages and the search, which are built per request. The assets are compressed when they are built, and never reach this. The decision is made when the status is written, from
// the content type the handler set, so a response that is already encoded, has no body, or is an image or font is left alone.
func compressed(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		coding := assetserve.Negotiate(r, "gzip")
		if r.Method == http.MethodHead || coding == "" {
			next.ServeHTTP(w, r)
			return
		}
		cw := &compressWriter{ResponseWriter: w, coding: coding}
		defer cw.finish()
		next.ServeHTTP(cw, r)
	})
}

type compressWriter struct {
	http.ResponseWriter
	coding  string // "gzip": the one coding that a built response is compressed with
	zw      encoder
	pool    *sync.Pool
	decided bool
}

func (c *compressWriter) decide(status int) {
	c.decided = true
	h := c.Header()
	if !slices.Contains(h.Values("Vary"), "Accept-Encoding") {
		h.Add("Vary", "Accept-Encoding")
	}
	if status < 200 || status == http.StatusNoContent || status == http.StatusNotModified || (status >= 300 && status < 400) {
		return
	}
	if h.Get("Content-Encoding") != "" || !assetserve.Compressible(h.Get("Content-Type")) {
		return
	}
	h.Set("Content-Encoding", c.coding)
	h.Del("Content-Length") // the length of what is sent is not the length of what was written
	c.pool = &gzipPool
	c.zw = c.pool.Get().(encoder)
	c.zw.Reset(c.ResponseWriter)
}

func (c *compressWriter) WriteHeader(status int) {
	if !c.decided {
		c.decide(status)
	}
	c.ResponseWriter.WriteHeader(status)
}

func (c *compressWriter) Write(b []byte) (int, error) {
	if !c.decided {
		c.decide(http.StatusOK)
	}
	if c.zw != nil {
		return c.zw.Write(b) //nolint:wrapcheck // a pass-through
	}
	return c.ResponseWriter.Write(b) //nolint:wrapcheck // a pass-through
}

// FlushError sends what has been written so far: the compressed stream is flushed, so that the client can decode
// it up to here, and then the writer below, so that a response that is sent as it is built reaches the client as
// it is built.
func (c *compressWriter) FlushError() error {
	if c.zw != nil {
		if err := c.zw.Flush(); err != nil {
			return err //nolint:wrapcheck // the client went away
		}
	}
	return http.NewResponseController(c.ResponseWriter).Flush() //nolint:wrapcheck // as above
}

// finish ends the compressed stream and gives the writer back.
func (c *compressWriter) finish() {
	if c.zw == nil {
		return
	}
	_ = c.zw.Close()
	c.zw.Reset(nil)
	c.pool.Put(c.zw)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (c *compressWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }
