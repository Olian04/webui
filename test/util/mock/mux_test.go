package mock

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestMux(t *testing.T) {
	t.Parallel()

	mux := NewMux()
	mux.Handle("/admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))

	response := mux.Serve(&http.Request{
		Method: http.MethodGet,
		URL:    &url.URL{Path: "/admin"},
	})

	assert.Equal(t, response.StatusCode, http.StatusOK)
	assert.Equal(t, response.Body.String(), "Hello, World!")
	assert.DeepEqual(t, response.Header, http.Header{})
	assert.Equal(t, response.Method, http.MethodGet)
	assert.Equal(t, response.URL.Path, "/admin")
}
