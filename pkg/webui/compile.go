package webui

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// CompileError is one declaration problem. Page is the path; Args is the
// argument type name. Both always appear in Error: they are the only
// coordinates available at run time.
type CompileError struct {
	Page   string
	Args   string
	Detail string
	Fix    string
}

func (e CompileError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "page %q", e.Page)
	if e.Args != "" {
		fmt.Fprintf(&b, " (%s)", e.Args)
	}
	fmt.Fprintf(&b, ": %s", e.Detail)
	if e.Fix != "" {
		fmt.Fprintf(&b, "\n  Fix: %s", e.Fix)
	}
	return b.String()
}

// CompileErrors is every problem found in one Compile. Error joins them.
type CompileErrors []CompileError

func (e CompileErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	msgs := make([]string, len(e))
	for i, err := range e {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "\n")
}

var errCompileUnimplemented = errors.New("webui: compile not implemented")

// Compile validates the declaration, lowers it to the runtime IR, and returns
// a handler. The handler is never nil: a failed compile still serves, at every
// path under prefix.
func (App) Compile(prefix string) (http.Handler, error) {
	return unimplementedHandler{prefix: prefix}, errCompileUnimplemented
}

// MustCompile is Compile panicking on error, like regexp.MustCompile.
func (a App) MustCompile(prefix string) http.Handler {
	h, err := a.Compile(prefix)
	if err != nil {
		panic(err)
	}
	return h
}

type unimplementedHandler struct {
	prefix string
}

// ASSERT: unimplementedHandler implements http.Handler
var _ http.Handler = unimplementedHandler{}

func (h unimplementedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = h.prefix
	_ = r
	http.Error(w, errCompileUnimplemented.Error(), http.StatusNotImplemented)
}
