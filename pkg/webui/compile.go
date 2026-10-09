package webui

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Olian04/webui/internal/render"
	"github.com/Olian04/webui/internal/runtime"
)

// MustCompile is Compile panicking on error, like regexp.MustCompile.
func (a App) MustCompile(prefix string) http.Handler {
	h, err := a.Compile(prefix)
	if err != nil {
		panic(err)
	}
	return h
}

// Compile validates the declaration, lowers it to the runtime IR, and returns
// a handler. The handler is never nil: a failed compile still serves, at every
// path under prefix, rendering the problems, so a broken app is diagnosable in
// the browser. The returned error wraps a [CompileError] for each problem found.
func (a App) Compile(prefix string) (http.Handler, error) {
	errs := a.validate()
	errs = append(errs, validatePrefix(prefix)...)
	if len(errs) > 0 {
		return failure(errs), joined(errs)
	}

	app, err := a.lower()
	if err != nil {
		errs = []CompileError{{Detail: err.Error()}}
		return failure(errs), joined(errs)
	}

	program, err := runtime.NewProgram(app, prefix)
	if err != nil {
		errs = []CompileError{{Detail: err.Error()}}
		return failure(errs), joined(errs)
	}
	return program.Handler(), nil
}

// joined is the problems as one error that wraps each of them.
func joined(errs []CompileError) error {
	all := make([]error, len(errs))
	for i, e := range errs {
		all[i] = e
	}
	return errors.Join(all...)
}

func failure(errs []CompileError) http.Handler {
	problems := make([]render.Problem, len(errs))
	for i, e := range errs {
		problems[i] = render.Problem{Where: e.where(), Detail: e.Detail, Fix: e.Fix}
	}
	return runtime.FailureHandler(problems)
}

// validatePrefix checks the one place the mount prefix is stated.
func validatePrefix(prefix string) []CompileError {
	if prefix == "" || (prefix[0] == '/' && prefix[len(prefix)-1] != '/' && !strings.ContainsAny(prefix, "{}?#")) {
		return nil
	}
	return []CompileError{{
		Detail: fmt.Sprintf("the mount prefix %q is not \"\" or a path such as \"/admin\"", prefix),
		Fix:    "Start the prefix with a slash and drop the trailing one; pass \"\" to mount at the root.",
	}}
}
