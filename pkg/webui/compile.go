package webui

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Olian04/webui/internal/convert"
	"github.com/Olian04/webui/internal/metrics"
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
	h, _, err := a.compile(prefix, nil)
	return h, err
}

// compile is Compile counting into rec when it is given one, which the reloader package does, through CompileCounting, so that a second compile does not register
// the metrics again, and saying what it counts into.
func (a App) compile(prefix string, rec *metrics.Recorder) (http.Handler, *metrics.Recorder, error) {
	errs := a.validate()
	errs = append(errs, validatePrefix(prefix)...)
	if len(errs) > 0 {
		return failure(errs), nil, joined(errs)
	}

	app, err := a.lower()
	if err != nil {
		errs = []CompileError{{Detail: err.Error()}}
		return failure(errs), nil, joined(errs)
	}

	program, err := runtime.NewProgramWith(app, prefix, rec)
	if err != nil {
		errs = []CompileError{{Detail: err.Error()}}
		if errors.As(err, new(runtime.RouteConflict)) {
			errs[0].Fix = "Change one path so that the other is strictly more specific: wherever one has a placeholder the other must have a placeholder or fixed text, and never the reverse. \"/y/{b}\" and \"/{a}/x\" cross; \"/y/{b}\" with \"/{a}/{b}\", or \"/y/x\" with \"/{a}/x\", do not."
		}
		return failure(errs), nil, joined(errs)
	}
	return program.Handler(), program.Recorder(), nil
}

// Counters is what a compiled app counts its metrics into, for an app that replaces it to count into too. It holds nothing
// a program can read: it is handed from one [App.CompileCounting] to the next by the reloader package.
type Counters struct{ rec *metrics.Recorder }

// CompileCounting is [App.Compile] for an app that replaces another in the same process: given the [Counters] of the first it
// counts into them, and the metrics are not registered a second time, which a second Compile of the same [App.Metrics] would
// fail on. Given nil it registers its own, as Compile does. It returns the counters of the app it compiled, nil when that has
// no [App.Metrics]. A program that recompiles uses the reloader package, which does this and swaps the handler.
func (a App) CompileCounting(prefix string, c *Counters) (http.Handler, *Counters, error) {
	var rec *metrics.Recorder
	if c != nil {
		rec = c.rec
	}
	h, got, err := a.compile(prefix, rec)
	if got == nil {
		return h, nil, err
	}
	return h, &Counters{rec: got}, err
}

// joined is the problems as one error that wraps each of them.
func joined(errs []CompileError) error {
	return errors.Join(convert.Slice(errs, func(e CompileError) error { return e })...)
}

func failure(errs []CompileError) http.Handler {
	return runtime.FailureHandler(convert.Slice(errs, func(e CompileError) render.Problem {
		return render.Problem{Where: e.where(), Detail: e.Detail, Fix: e.Fix}
	}))
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
