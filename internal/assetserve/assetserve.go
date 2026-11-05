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

	"github.com/andybalholm/brotli"
)

// Blob is one file, and what it needs to be served well.
type Blob struct {
	Mime  string
	Cache string // the Cache-Control of the response
	ETag  string // quoted; a compressed copy has its own, made from it
	Body  []byte

	// Gzip and Brotli are the compressed Body, or nil for a file that is not offered in that coding. Each is called only when a client takes that coding, so a file that
	// nobody asks for compressed is never compressed. Brotli is preferred where a client takes both, being the smaller; it is a file that was compressed when it was built
	// (the build writes them beside the files that are embedded) or, for one that the app makes when it starts, the first time it is asked for.
	Gzip   func() []byte
	Brotli func() []byte
}

// ServeHTTP answers a request with the file: compressed when the file offers it and the client takes it,
// and with a 304 when the client has it already.
func (b *Blob) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	etag := b.ETag
	h := w.Header()
	h.Set("Content-Type", b.Mime)
	h.Set("Cache-Control", b.Cache)
	var offered []string
	if b.Brotli != nil {
		offered = append(offered, "br")
	}
	if b.Gzip != nil {
		offered = append(offered, "gzip")
	}
	coding := ""
	if len(offered) > 0 {
		h.Add("Vary", "Accept-Encoding")
		if coding = Negotiate(req, offered...); coding != "" {
			h.Set("Content-Encoding", coding)
			etag = strings.TrimSuffix(etag, `"`) + "-" + coding + `"`
		}
	}
	h.Set("ETag", etag)
	if req.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return // a client that has the file is not sent it, and not made to compress it either
	}
	body := b.Body
	switch coding {
	case "br":
		body = b.Brotli()
	case "gzip":
		body = b.Gzip()
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

// Brotli is body compressed at the best level, which is slow (a megabyte takes seconds): for a file that is served many times, and compressed once.
func Brotli(body []byte) []byte {
	var buf bytes.Buffer
	zw := brotli.NewWriterLevel(&buf, brotli.BestCompression)
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

// Negotiate is the coding of those offered, in order of preference, that the client's Accept-Encoding takes the most: the one it gives the highest quality, the first
// offered of those that are equal, and none (an empty string) when it takes none of them. A coding it lists with a quality of zero is refused, and "*" is every coding
// it does not name.
func Negotiate(r *http.Request, offered ...string) string {
	quality := map[string]float64{}
	star, hasStar := 0.0, false
	for part := range strings.SplitSeq(r.Header.Get("Accept-Encoding"), ",") {
		coding, params, _ := strings.Cut(part, ";")
		coding = strings.ToLower(strings.TrimSpace(coding))
		if coding == "" {
			continue
		}
		q := 1.0
		if _, v, ok := strings.Cut(strings.ReplaceAll(params, " ", ""), "q="); ok {
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				q = parsed
			}
		}
		if coding == "*" {
			star, hasStar = q, true
			continue
		}
		quality[coding] = q
	}
	best, bestQ := "", 0.0
	for _, coding := range offered {
		q, named := quality[coding]
		if !named {
			if !hasStar {
				continue
			}
			q = star
		}
		if q > bestQ {
			best, bestQ = coding, q
		}
	}
	return best
}
