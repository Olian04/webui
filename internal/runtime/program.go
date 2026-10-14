// Package runtime is the served form of a compiled app: the route table, the
// request pipeline, and the context that lets Open and ArgsOf find the program
// from inside a user's closure.
//
// It decides what data a request shows — guards, loaders, actions — and hands
// plain values to internal/render, which decides how they look.
package runtime

import (
	"cmp"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Olian04/webui/internal/assetserve"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/metrics"
	"github.com/Olian04/webui/internal/render"
)

// Program is the served form of an ir.App: routes derived from page path
// templates, plus the prefix the app is mounted under. Routes are a transport
// concern, which is why they are built here and not carried by the IR.
type Program struct {
	Prefix string
	App    *ir.App

	// mux is every route of the app, each wrapped in the headers and the panic
	// recovery. ServeMux keeps a tree of patterns and picks the most specific one for
	// a request, so registration order does not decide the winner.
	mux *http.ServeMux

	// fw is the framework's own routes, the ones under the asset directory: assets, the
	// search, the editor and its files. They are a mux of their own, because the pages'
	// patterns may begin with a placeholder, and "/{key}/info" and "/_webui/{file}" both match
	// "/_webui/info" with neither the more specific, which a single ServeMux refuses to
	// have. The asset directory is reserved, so no page is ever served from it, and
	// [Program.Handler] sends what is under it here and the rest to mux.
	fw *http.ServeMux

	render *render.Renderer
	log    *slog.Logger

	// m counts and times what the app does. It is nil when the app was given no registry,
	// and a nil Recorder records nothing.
	m *metrics.Recorder

	// gate answers one question about an address: may this visitor open it? Each page's
	// handler on it runs the page's argument decoding and Guard, as a real request would,
	// and reports the verdict as a status. It matches a path against the page patterns
	// regardless of method, which is also how the catch-all tells "no such page" (404) from
	// "wrong method" (405).
	gate *http.ServeMux

	allow map[string]string // gate pattern → Allow header

	// cross refuses cross-origin POSTs, using Fetch metadata and Origin. A
	// request from a client that sends neither is not a browser CSRF vector.
	cross *http.CrossOriginProtection
}

// NewProgram derives the routes from a compiled app.
func NewProgram(app *ir.App, prefix string) (*Program, error) {
	secret := app.Secret
	if len(secret) == 0 {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("webui: cannot make a secret: %w", err)
		}
	}
	r, err := render.New(app, prefix, secret)
	if err != nil {
		return nil, err
	}
	p := &Program{
		Prefix: prefix, App: app, render: r, log: slog.Default(),
		mux: http.NewServeMux(), fw: http.NewServeMux(), gate: http.NewServeMux(), allow: map[string]string{}, cross: http.NewCrossOriginProtection(),
	}

	if app.Metrics != nil {
		m, err := metrics.New(app.Metrics)
		if err != nil {
			return nil, fmt.Errorf("webui: App.Metrics: %w", err)
		}
		p.m = m
	}

	own := func(method, path string, h http.Handler) {
		p.handle(p.fw, method, prefix+render.AssetDir+path, framework, csp, h)
	}
	own(http.MethodGet, "/{file}", serveFile(func(req *http.Request) (*assetserve.Blob, bool) { return p.render.Asset(req.PathValue("file")) }))
	own(http.MethodGet, "/search", http.HandlerFunc(p.search))
	own(http.MethodGet, "/monaco/{path...}", serveFile(func(req *http.Request) (*assetserve.Blob, bool) { return p.render.MonacoFile(req.PathValue("path")) }))
	own(http.MethodGet, "/code/{name}", serveFile(func(req *http.Request) (*assetserve.Blob, bool) {
		return p.render.ContextFile(req.PathValue("name"), req.URL.Query().Get("k"))
	}))
	// Anything else under the directory is not a page, whatever a page's pattern could
	// say of it: it is a 404 in the shell.
	own("", "/", http.HandlerFunc(p.missing))
	own("", "", http.HandlerFunc(p.missing))

	if err := p.addPages(); err != nil {
		return nil, err
	}
	if app.ByPath["/"] == nil {
		p.handle(p.mux, http.MethodGet, prefix+"/{$}", "/", csp, http.HandlerFunc(p.root))
		if prefix != "" {
			p.handle(p.mux, http.MethodGet, prefix, "/", csp, http.HandlerFunc(p.root)) // mounted at "/admin" exactly
		}
	}

	// The catch-all is last in intent, not in registration: ServeMux picks the
	// most specific pattern, so it only sees what nothing else claimed.
	p.handle(p.mux, "", prefix+"/", unmatched, csp, http.HandlerFunc(p.notFound))
	return p, nil
}

// Handler is the whole app, for the mount prefix it was built with: what is under the
// asset directory is the framework's, and the rest is the pages'.
func (p *Program) Handler() http.Handler {
	dir := p.Prefix + render.AssetDir
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == dir || strings.HasPrefix(r.URL.Path, dir+"/") {
			p.fw.ServeHTTP(w, r)
			return
		}
		p.mux.ServeHTTP(w, r)
	})
}

// The page label of what no page answered, and of the framework's own routes. Neither is
// the path template of a page, which always begins with a slash and is not reserved.
const (
	unmatched = "unmatched"
	framework = render.AssetDir
)

// handle registers a route on mux, wrapped in what every response of the app has: the metrics, the
// headers and content security policy, compression and the recovery of a panic. An empty method
// matches any, which only the catch-all uses. page is what the route is called in the metrics: the path
// template of the page it answers.
func (p *Program) handle(mux *http.ServeMux, method, path, page, policy string, h http.Handler) {
	pattern := path
	if method != "" {
		pattern = method + " " + path
	}
	mux.Handle(pattern, p.metered(page, securedFor(policy, compressed(p.recovered(h)))))
}

// metered counts a request and its time, under the page that answers it. The status is what
// the handler wrote, and a handler that wrote none answered 200.
func (p *Program) metered(page string, next http.Handler) http.Handler {
	if p.m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		done := p.m.Begin()
		sw := &statusWriter{ResponseWriter: w}
		defer func() { done(page, r.Method, cmp.Or(sw.status, http.StatusOK), started) }() // none written is a 200
		next.ServeHTTP(sw, r)
	})
}

// RouteConflict is two pages whose paths match one address, with neither more specific than
// the other: "/{a}/x" and "/y/{b}" both match "/y/x". The router cannot choose between them.
type RouteConflict struct{ Detail string }

func (e RouteConflict) Error() string { return e.Detail }

// addPages registers every page. ServeMux panics when two patterns conflict, which is the
// application's declaration being wrong, so it is reported as an error and not a crash.
func (p *Program) addPages() (err error) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprint(r)
			lines := strings.Split(msg, "\n")
			detail := msg
			if len(lines) > 1 {
				detail = strings.TrimSpace(lines[1])
			}
			for _, method := range []string{"GET ", "POST "} {
				detail = strings.ReplaceAll(detail, method, "")
			}
			if p.Prefix != "" {
				detail = strings.ReplaceAll(detail, p.Prefix+"/", "/") // the paths as the pages declare them
			}
			err = RouteConflict{Detail: "two page paths match the same address and neither is more specific: " + detail}
		}
	}()
	for _, page := range p.App.Pages {
		p.addPage(page)
	}
	return nil
}

// serveFile is a route that answers with a file looked up in the request. A file that is not there is a 404.
func serveFile(find func(*http.Request) (*assetserve.Blob, bool)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if b, ok := find(req); ok {
			b.ServeHTTP(w, req)
			return
		}
		http.NotFound(w, req)
	})
}
