package assetserve

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func counted(body []byte) (*Blob, *atomic.Int32) {
	var calls atomic.Int32
	return &Blob{
		Mime: "text/plain", Cache: "public, immutable", ETag: `"abc"`, Body: body,
		Gzip: func() []byte { calls.Add(1); return Gzip(body) },
	}, &calls
}

func serve(b *Blob, mod func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/f", nil)
	mod(req)
	rec := httptest.NewRecorder()
	b.ServeHTTP(rec, req)
	return rec
}

func TestAClientThatHasTheFileIsNotSentItAndNotMadeToCompressIt(t *testing.T) {
	t.Parallel()

	b, calls := counted([]byte("the file"))
	rec := serve(b, func(r *http.Request) {
		r.Header.Set("Accept-Encoding", "gzip")
		r.Header.Set("If-None-Match", `"abc-gzip"`)
	})
	assert.Equal(t, rec.Code, http.StatusNotModified)
	assert.Equal(t, calls.Load(), int32(0))
	assert.Equal(t, rec.Body.Len(), 0)
}

func TestACompressedFileIsMadeOnlyForAClientThatTakesIt(t *testing.T) {
	t.Parallel()

	b, calls := counted([]byte("the file"))
	plain := serve(b, func(*http.Request) {})
	assert.Equal(t, plain.Body.String(), "the file")
	assert.Equal(t, plain.Header().Get("Content-Encoding"), "")
	assert.Equal(t, calls.Load(), int32(0))

	zipped := serve(b, func(r *http.Request) { r.Header.Set("Accept-Encoding", "gzip") })
	assert.Equal(t, zipped.Header().Get("Content-Encoding"), "gzip")
	assert.Equal(t, zipped.Header().Get("ETag"), `"abc-gzip"`)
	assert.Equal(t, calls.Load(), int32(1))
	assert.Contains(t, zipped.Header().Get("Vary"), "Accept-Encoding")
}

func TestNegotiateTakesTheCodingTheClientLikesBestAndTheFirstOfferedOfEqualOnes(t *testing.T) {
	t.Parallel()

	for header, want := range map[string]string{
		"":                       "",
		"identity":               "",
		"gzip":                   "gzip",
		"br":                     "br",
		"gzip, br":               "br", // equal: the one offered first
		"br, gzip":               "br",
		"gzip, deflate, br":      "br",
		"br;q=0, gzip":           "gzip", // refused
		"gzip;q=0":               "",
		"gzip;q=0.5, br;q=0.4":   "gzip", // the client likes gzip better
		"gzip; q=0.2, br; q=0.9": "br",
		"*":                      "br",
		"*;q=0":                  "",
		"br;q=0, *":              "gzip", // * is what it does not name
		"GZIP":                   "gzip",
		"br;q=oops":              "br", // a quality that is not a number is the default
	} {
		req := httptest.NewRequest(http.MethodGet, "/f", nil)
		if header != "" {
			req.Header.Set("Accept-Encoding", header)
		}
		assert.Equal(t, Negotiate(req, "br", "gzip"), want)
	}
	req := httptest.NewRequest(http.MethodGet, "/f", nil)
	req.Header.Set("Accept-Encoding", "br")
	assert.Equal(t, Negotiate(req, "gzip"), "") // a coding that is not offered is not taken
}

func bothCodings(body []byte) (*Blob, *atomic.Int32) {
	var calls atomic.Int32
	return &Blob{
		Mime: "text/plain", Cache: "public, immutable", ETag: `"abc"`, Body: body,
		Gzip:   func() []byte { calls.Add(1); return Gzip(body) },
		Brotli: func() []byte { calls.Add(100); return Brotli(body) },
	}, &calls
}

func TestAFileThatOffersBothIsServedInBrotliToAClientThatTakesBoth(t *testing.T) {
	t.Parallel()

	b, calls := bothCodings([]byte("the file, the file, the file"))
	rec := serve(b, func(r *http.Request) { r.Header.Set("Accept-Encoding", "gzip, br") })
	assert.Equal(t, rec.Header().Get("Content-Encoding"), "br")
	assert.Equal(t, rec.Header().Get("ETag"), `"abc-br"`)
	assert.Contains(t, rec.Header().Get("Vary"), "Accept-Encoding")
	assert.Equal(t, calls.Load(), int32(100)) // the Brotli copy was made, and not the gzip one

	zipped := serve(b, func(r *http.Request) { r.Header.Set("Accept-Encoding", "gzip") })
	assert.Equal(t, zipped.Header().Get("Content-Encoding"), "gzip")
	assert.Equal(t, zipped.Header().Get("ETag"), `"abc-gzip"`)

	plain := serve(b, func(*http.Request) {})
	assert.Equal(t, plain.Header().Get("Content-Encoding"), "")
	assert.Equal(t, plain.Header().Get("ETag"), `"abc"`)
	assert.Equal(t, plain.Body.String(), "the file, the file, the file")
}

func TestAClientThatHasTheBrotliCopyIsNotSentItAndNotMadeToMakeIt(t *testing.T) {
	t.Parallel()

	b, calls := bothCodings([]byte("the file"))
	rec := serve(b, func(r *http.Request) {
		r.Header.Set("Accept-Encoding", "br")
		r.Header.Set("If-None-Match", `"abc-br"`)
	})
	assert.Equal(t, rec.Code, http.StatusNotModified)
	assert.Equal(t, calls.Load(), int32(0))

	// The tag of the other coding is not the tag of this one: it is sent the file.
	other := serve(b, func(r *http.Request) {
		r.Header.Set("Accept-Encoding", "br")
		r.Header.Set("If-None-Match", `"abc-gzip"`)
	})
	assert.Equal(t, other.Code, http.StatusOK)
}

func TestAFileThatOffersOnlyGzipIsNotSentInBrotli(t *testing.T) {
	t.Parallel()

	b, _ := counted([]byte("the file"))
	rec := serve(b, func(r *http.Request) { r.Header.Set("Accept-Encoding", "br") })
	assert.Equal(t, rec.Header().Get("Content-Encoding"), "")
	assert.Equal(t, rec.Body.String(), "the file")
}
