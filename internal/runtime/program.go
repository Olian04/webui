package runtime

import (
	"net/http"

	"github.com/Olian04/webui/internal/ir"
)

// Program is the served form of an ir.App: routes derived from page path
// templates, plus the prefix the app is mounted under. Routes are a transport
// concern, which is why they are built here and not carried by the IR.
type Program struct {
	Prefix string
	App    *ir.App
	Routes []Route
}

// Route is one pattern registered on the mux.
type Route struct {
	Path       string
	Method     string // Page = GET, Action = POST, PUT, DELETE, etc.
	Middleware []func(http.Handler) http.Handler
	Handler    http.Handler
}

// NewProgram derives the route table from a compiled app.
func NewProgram(app *ir.App, prefix string) (*Program, error) {
	return &Program{Prefix: prefix, App: app, Routes: nil}, nil // TODO: implement
}
