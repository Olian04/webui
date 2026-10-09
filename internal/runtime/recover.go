package runtime

import (
	"errors"
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
		tw := &trackedWriter{ResponseWriter: w}
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(v)
			}
			p.log.Error("webui: panic while serving", "method", r.Method, "path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
			if !tw.started {
				p.write(tw, r, http.StatusInternalServerError, render.Doc{Title: "Error", Content: render.ServerError()})
			}
		}()
		next.ServeHTTP(tw, r)
	})
}

// trackedWriter remembers whether the response has begun.
type trackedWriter struct {
	http.ResponseWriter
	started bool
}

func (w *trackedWriter) WriteHeader(code int) {
	w.started = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *trackedWriter) Write(b []byte) (int, error) {
	w.started = true
	return w.ResponseWriter.Write(b) //nolint:wrapcheck // a pass-through
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *trackedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
