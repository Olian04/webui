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
