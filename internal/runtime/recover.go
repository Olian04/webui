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
		// However the request ends, the sidebar's workers and the panels it started have finished when it does: a request that panicked,
		// failed, or was left by the visitor before it waited for them would otherwise leave them running past its end. Their context
		// is the request's, so a Load that heeds it is done soon.
		defer func() {
			req := From(r.Context())
			if req == nil {
				return
			}
			if req.shell != nil {
				req.shell.join()
			}
			if req.panels != nil {
				req.panels.settle()
			}
		}()
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(v)
			}
			stack := debug.Stack()
			if w, ok := v.(*workerPanic); ok {
				v, stack = w.value, w.stack // where it happened, and not where it was raised again
			}
			p.log.Error("webui: panic while serving", "method", r.Method, "path", r.URL.Path, "panic", v, "stack", string(stack))
			if tw.status == 0 {
				p.write(tw, r, http.StatusInternalServerError, render.Doc{Title: "Error", Content: render.ServerError(r.URL.RequestURI())})
			}
		}()
		next.ServeHTTP(tw, r)
	})
}

// workerPanic is a panic on a goroutine that a request started, caught there so that it does not end the
// process, and raised again on the request's own goroutine, where [Program.recovered] answers it.
type workerPanic struct {
	value any
	stack []byte
}

func (w *workerPanic) raise() {
	if err, ok := w.value.(error); ok && errors.Is(err, http.ErrAbortHandler) {
		panic(w.value) // the server's own way to end a response, which it handles
	}
	panic(w)
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
// way of every page, or on every keystroke, and must not be what takes it down. what names the func. The
// error of a panic is a *panicError, so that a caller can tell a func that broke from one that said no.
func guarded[T any](what string, f func() (T, error)) (v T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &panicError{what: what, value: r}
		}
	}()
	return f() //nolint:wrapcheck // the caller logs it with the page
}

// panicError is a panic in an application's func, as [guarded] returns it.
type panicError struct {
	what  string
	value any
}

func (e *panicError) Error() string { return fmt.Sprintf("panic in %s: %v", e.what, e.value) }
