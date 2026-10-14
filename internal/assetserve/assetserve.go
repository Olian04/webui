// Package assetserve serves the files an app answers with that do not change under their name: the
// stylesheets and scripts, the editor, the files an editor is given. It is the one place that knows
// how such a file is kept by a browser, offered compressed, and answered with a 304, so that what builds
// the files (internal/render) knows nothing of a request.
package assetserve

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strconv"
	"strings"
)

// Blob is one file, and what it needs to be served well.
type Blob struct {
	Mime  string
	Cache string // the Cache-Control of the response
	ETag  string // quoted; the compressed copy has its own, made from it
	Body  []byte

	// Gzip is the compressed Body, or nil for a file that is not offered compressed. It is called
	// only when a client accepts gzip, so a file nobody asks for compressed is never compressed.
	Gzip func() []byte
}

// ServeHTTP answers a request with the file: compressed when the file offers it and the client takes it,
// and with a 304 when the client has it already.
func (b *Blob) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, etag := b.Body, b.ETag
	h := w.Header()
	h.Set("Content-Type", b.Mime)
	h.Set("Cache-Control", b.Cache)
	if b.Gzip != nil {
		h.Add("Vary", "Accept-Encoding")
		if AcceptsGzip(req) {
			h.Set("Content-Encoding", "gzip")
			body, etag = b.Gzip(), strings.TrimSuffix(etag, `"`)+`-gzip"`
		}
	}
	h.Set("ETag", etag)
	if req.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = w.Write(body) //nolint:gosec // G705: a file the application embedded, never request data.
}

// Gzip is body compressed at the best level.
func Gzip(body []byte) []byte {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = zw.Write(body)
	_ = zw.Close()
	return buf.Bytes()
}

// Compressible reports whether a response of this content type is worth compressing:
// text of any kind, JSON and SVG. Images and fonts are compressed already.
func Compressible(contentType string) bool {
	ct, _, _ := strings.Cut(contentType, ";")
	ct = strings.TrimSpace(strings.ToLower(ct))
	return strings.HasPrefix(ct, "text/") || ct == "application/json" || ct == "image/svg+xml"
}

// AcceptsGzip reports whether the client accepts a gzip-encoded response: it lists
// gzip, and not with a quality of zero.
func AcceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		coding, params, _ := strings.Cut(part, ";")
		if !strings.EqualFold(strings.TrimSpace(coding), "gzip") {
			continue
		}
		q := 1.0
		if _, v, ok := strings.Cut(strings.ReplaceAll(params, " ", ""), "q="); ok {
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				q = parsed
			}
		}
		return q > 0
	}
	return false
}
