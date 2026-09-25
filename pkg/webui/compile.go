package webui

import (
	"net/http"

	"github.com/Olian04/webui/internal/router"
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
// path under prefix.
func (a App) Compile(prefix string) (http.Handler, error) {
	errors := a.validate()
	if len(errors) > 0 {
		return nil, errors
	}

	program, err := a.lower()
	if err != nil {
		return nil, err
	}

	router := router.New(program)
	return router.Mux, nil
}
