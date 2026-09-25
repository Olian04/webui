package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Olian04/webui/internal/runtime"
	"github.com/Olian04/webui/test/util/assert"
)

func TestNewRoutesByMethodAndPath(t *testing.T) {
	t.Parallel()

	r := New(&runtime.Program{
		Routes: []runtime.Route{
			{Method: http.MethodGet, Path: "/device", Handler: text("list")},
			{Method: http.MethodPost, Path: "/device", Handler: text("create")},
			{Method: http.MethodGet, Path: "/device/{id}", Handler: text("show")},
			{Method: http.MethodGet, Path: "/device/me", Handler: text("me")},
		},
	})

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
		wantID     string
	}{
		{name: "get list", method: http.MethodGet, path: "/device", wantStatus: http.StatusOK, wantBody: "list"},
		{name: "post create", method: http.MethodPost, path: "/device", wantStatus: http.StatusOK, wantBody: "create"},
		{name: "get show", method: http.MethodGet, path: "/device/42", wantStatus: http.StatusOK, wantBody: "show", wantID: "42"},
		{name: "static beats wildcard", method: http.MethodGet, path: "/device/me", wantStatus: http.StatusOK, wantBody: "me"},
		{name: "head matches get", method: http.MethodHead, path: "/device", wantStatus: http.StatusOK, wantBody: "list"},
		{name: "wrong method", method: http.MethodPut, path: "/device", wantStatus: http.StatusMethodNotAllowed, wantBody: "Method Not Allowed\n"},
		{name: "unknown path", method: http.MethodGet, path: "/missing", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			r.Mux.ServeHTTP(rec, req)

			assert.Equal(t, rec.Code, tt.wantStatus)
			assert.Equal(t, rec.Body.String(), tt.wantBody)
			assert.Equal(t, req.PathValue("id"), tt.wantID)
		})
	}
}

func TestNewSpecificityIgnoresRegistrationOrder(t *testing.T) {
	t.Parallel()

	r := New(&runtime.Program{
		Routes: []runtime.Route{
			{Method: http.MethodGet, Path: "/device/{id}", Handler: text("show")},
			{Method: http.MethodGet, Path: "/device/me", Handler: text("me")},
		},
	})

	rec := httptest.NewRecorder()
	r.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/device/me", nil))
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, rec.Body.String(), "me")
}

func TestNewMiddlewareRunsOuterLast(t *testing.T) {
	t.Parallel()

	r := New(&runtime.Program{
		Routes: []runtime.Route{
			{
				Method: http.MethodGet,
				Path:   "/device",
				Middleware: []func(http.Handler) http.Handler{
					stamp("a"),
					stamp("b"),
				},
				Handler: text("ok"),
			},
		},
	})

	rec := httptest.NewRecorder()
	r.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/device", nil))
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, rec.Body.String(), "ok")
	assert.DeepEqual(t, rec.Header().Values("X-Stamp"), []string{"b", "a"})
}

func TestNewEmptyProgram(t *testing.T) {
	t.Parallel()

	r := New(&runtime.Program{})
	rec := httptest.NewRecorder()
	r.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, rec.Code, http.StatusNotFound)
	assert.Equal(t, rec.Body.String(), "404 page not found\n")
}

func TestNewPanicsOnConflictingPattern(t *testing.T) {
	t.Parallel()

	defer func() {
		assert.NotNil(t, recover())
	}()

	New(&runtime.Program{
		Routes: []runtime.Route{
			{Method: http.MethodGet, Path: "/device", Handler: text("a")},
			{Method: http.MethodGet, Path: "/device", Handler: text("b")},
		},
	})
	t.Fatal("New returned")
}

func text(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := io.WriteString(w, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func stamp(mark string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Stamp", mark)
			next.ServeHTTP(w, r)
		})
	}
}
