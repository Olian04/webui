// Package runtime is the served form of a compiled app: the route table, the
// request pipeline, and the context that lets Open and ArgsOf find the program
// from inside a user's closure.
//
// It decides what data a request shows — guards, loaders, actions — and hands
// plain values to internal/render, which decides how they look.
package runtime

import (
	"log/slog"
	"net/http"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// Program is the served form of an ir.App: routes derived from page path
// templates, plus the prefix the app is mounted under. Routes are a transport
// concern, which is why they are built here and not carried by the IR.
type Program struct {
	Prefix string
	App    *ir.App
	Routes []Route

	render *render.Renderer
	log    *slog.Logger

	// probe matches a path against the page patterns regardless of method, so
	// the catch-all can tell "no such page" (404) from "wrong method" (405).
	probe *http.ServeMux

	// gate is probe's twin that answers one question about an address: may this
	// visitor open it? Each page's handler on it runs the page's argument decoding
	// and Guard, as a real request would, and reports the verdict as a status.
	gate *http.ServeMux
	allow map[string]string // probe pattern → Allow header

	// cross refuses cross-origin POSTs, using Fetch metadata and Origin. A
	// request from a client that sends neither is not a browser CSRF vector.
	cross *http.CrossOriginProtection
}

// Route is one pattern registered on the mux.
type Route struct {
	Path       string
	Method     string // Page = GET, Action = POST; empty matches any method
	Middleware []func(http.Handler) http.Handler
	Handler    http.Handler
}

// NewProgram derives the route table from a compiled app.
func NewProgram(app *ir.App, prefix string) (*Program, error) {
	p := &Program{
		Prefix: prefix, App: app, render: render.New(app, prefix), log: slog.Default(),
		probe: http.NewServeMux(), gate: http.NewServeMux(), allow: map[string]string{}, cross: http.NewCrossOriginProtection(),
	}

	assets, err := p.render.Assets()
	if err != nil {
		return nil, err
	}
	p.add(http.MethodGet, prefix+render.AssetDir+"/{file}", assets)
	p.add(http.MethodGet, prefix+render.AssetDir+"/search", http.HandlerFunc(p.search))

	for _, page := range app.Pages {
		p.addPage(page)
	}
	if app.ByPath["/"] == nil {
		p.add(http.MethodGet, prefix+"/{$}", http.HandlerFunc(p.root))
		if prefix != "" {
			p.add(http.MethodGet, prefix, http.HandlerFunc(p.root)) // mounted at "/admin" exactly
		}
	}

	// The catch-all is last in intent, not in registration: ServeMux picks the
	// most specific pattern, so it only sees what nothing else claimed.
	p.add("", prefix+"/", http.HandlerFunc(p.notFound))
	return p, nil
}

func (p *Program) add(method, path string, h http.Handler) {
	p.Routes = append(p.Routes, Route{
		Method: method, Path: path, Handler: h,
		Middleware: []func(http.Handler) http.Handler{secure, p.recovered},
	})
}
