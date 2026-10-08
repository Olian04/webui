package webui

import (
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
// the browser. The returned error is a CompileErrors in that case.
func (a App) Compile(prefix string) (http.Handler, error) {
	errs := a.validate()
	errs = append(errs, validatePrefix(prefix)...)
	if len(errs) > 0 {
		return failure(errs), errs
	}

	app, err := a.lower()
	if err != nil {
		errs = CompileErrors{{Detail: err.Error()}}
		return failure(errs), errs
	}

	program, err := runtime.NewProgram(app, prefix)
	if err != nil {
		errs = CompileErrors{{Detail: err.Error()}}
		return failure(errs), errs
	}
	return program.Handler(), nil
}

func failure(errs CompileErrors) http.Handler {
	problems := make([]render.Problem, len(errs))
	for i, e := range errs {
		problems[i] = render.Problem{Where: e.where(), Detail: e.Detail, Fix: e.Fix}
	}
	return runtime.FailureHandler(problems)
}

// validatePrefix checks the one place the mount prefix is stated.
func validatePrefix(prefix string) CompileErrors {
	if prefix == "" || (prefix[0] == '/' && prefix[len(prefix)-1] != '/' && !strings.ContainsAny(prefix, "{}?#")) {
		return nil
	}
	return CompileErrors{{
		Detail: fmt.Sprintf("the mount prefix %q is not \"\" or a path such as \"/admin\"", prefix),
		Fix:    "Start the prefix with a slash and drop the trailing one; pass \"\" to mount at the root.",
	}}
}
