package mock

import (
	"bytes"
	"net/http"
	"net/url"
)

type testResponseWriter struct {
	body       *bytes.Buffer
	header     http.Header
	statusCode int
}

func (w *testResponseWriter) Header() http.Header {
	return w.header
}

func (w *testResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *testResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

var _ http.ResponseWriter = &testResponseWriter{}

// Response is the response from the mock HTTP server.
type Response struct {
	Header     http.Header
	Method     string
	URL        *url.URL
	StatusCode int
	Body       *bytes.Buffer
}

// Mux is a mock HTTP server.
type Mux struct {
	mux *http.ServeMux
}

// Handle registers the handler for the given pattern.
func (m *Mux) Handle(pattern string, handler http.Handler) {
	m.mux.Handle(pattern, handler)
}

// Serve the request and return the response.
func (m *Mux) Serve(r *http.Request) Response {
	writer := &testResponseWriter{
		body:       bytes.NewBufferString(""),
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
	m.mux.ServeHTTP(writer, r)
	return Response{
		Header:     writer.header,
		Method:     r.Method,
		URL:        r.URL,
		StatusCode: writer.statusCode,
		Body:       writer.body,
	}
}

// NewMux creates a new mock HTTP server.
func NewMux() *Mux {
	return &Mux{mux: http.NewServeMux()}
}
