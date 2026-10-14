package runtime

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/Olian04/webui/internal/render"
)

// recovered turns a panic in a user's closure, or in the library's own use of
// one, into the server-error page and a log line carrying the cause and the
// stack, rather than a dropped connection. net/http would recover it too, but
// only to close the connection and log: the browser would see nothing.
//
// A response already started cannot be replaced; it is left as it is, and the
// panic is still logged. http.ErrAbortHandler is the one deliberate panic, and
// is passed on.
func (p *Program) recovered(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tw := &statusWriter{ResponseWriter: w}
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(v)
			}
			p.log.Error("webui: panic while serving", "method", r.Method, "path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
			if tw.status == 0 {
				p.write(tw, r, http.StatusInternalServerError, render.Doc{Title: "Error", Content: render.ServerError()})
			}
		}()
		next.ServeHTTP(tw, r)
	})
}

// statusWriter remembers the status written, which is whether the response has begun, and what the
// metrics call it.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b) //nolint:wrapcheck // a plain pass-through
}

// Unwrap lets http.ResponseController reach the writer underneath.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// guarded is what an application's func returns, with a panic in it answered as an error: it runs on the
// way of every page, or on every keystroke, and must not be what takes it down. what names the func.
func guarded[T any](what string, f func() (T, error)) (v T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in %s: %v", what, r)
		}
	}()
	return f() //nolint:wrapcheck // the caller logs it with the page
}
