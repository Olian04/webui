// Command demo serves a small admin panel built with webui, over in-memory
// data. It exists to look at the library working; it is not part of its API.
//
// The app is a static config: every page, accessor and action is a top-level
// var, read top to bottom. devices.go is the place to start, and each file shows
// its own corner of the library:
//
//	devices.go   a table (paging, sorting, filters, search, row links), a form
//	             with tabs, a path and a query argument, a rejection
//	sites.go     a nested path, two stateful tables on one page, more search
//	alerts.go    row and bulk actions, a destructive role, links with arguments,
//	             every kind of outcome (success, warning, failure, then)
//	settings.go  forms: rules, Float, Slider, a placeholder, Secret, Email, Phone, Optional*
//	uploads.go   File: a logo and a certificate, kept by the app and served as downloads
//	system.go    a read-only form; a page guarded for editors; an unknown total
//	overview.go  the landing page, which shows the library all at once
//	auth.go      the surrounding authentication: sign in as a viewer or an editor at
//	             /login, and the Guards read the role from the request's context
//
//	go run ./cmd/demo                        # http://localhost:7689/admin/
//	go run ./cmd/demo -accent '#2f9e8f'      # override the theme's accent colour
//	go run ./cmd/demo -broken                # the failed-to-compile page: three pages, broken in three ways
package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"slices"
	"time"

	// The JSON language service for the editor: an optional import, since it is a worker of
	// its own. The program is smaller without it, and the editor still highlights.
	_ "github.com/Olian04/webui/pkg/monaco/json"

	// And TypeScript's, which is large (about 7 MB of worker), so it is the one to leave out when
	// the program should be small: the editor then highlights TypeScript and checks nothing.
	_ "github.com/Olian04/webui/pkg/monaco/typescript"
	"github.com/Olian04/webui/pkg/webui"
)

var app = webui.App{
	// The logo is any image.Image. The theme is left at its default here; the
	// -accent flag overrides the accent colour (see themeFor).
	Brand: webui.Brand{Name: "Collector", Logo: logo()},
	// Where the app says what went wrong that it could not tell the visitor. Left out, it logs nothing.
	Logger: slog.Default(),
	// The app's links to places outside it, behind their own button in the top bar. An
	// ExternalLink is a Nav entry (a Label, an Icon and a Section) with a URL, used as
	// written: a web address opens in a tab of its own, a path such as /login in this one.
	ExternalLinks: []webui.ExternalLink{
		{Label: "Documentation", Icon: "book", Section: "Help", URL: "https://pkg.go.dev/github.com/Olian04/webui/pkg/webui"},
		{Label: "Report a problem", Icon: "bug", Section: "Help", URL: "https://github.com/Olian04/webui/issues"},
		{Label: "Log out", Icon: "right-from-bracket", Section: "Account", URL: "/login"},
	},
	Pages: webui.Pages{Overview, Devices, NewDevicePage, Details, Compare, Sites, SiteDetail, Alerts, AlertDetails, Ingest, Retention, Configuration, PagingRule, System, Audit, FirmwarePage, FilesRoot, FilesAt, HandbookRunbook, HandbookIncidents, HandbookSinks, HandbookGlossary, LoadingOrder, LoadingStaircase, LoadingTabs, LoadingNested, LoadingFailures, LoadingWhole, LoadingGuarded, LoadingFast, LayoutSplit, LayoutRail, LayoutColumns, LayoutTabs, LayoutManyTabs, LayoutDeep, LayoutReview, LayoutRunbook},
}

var (
	broken = flag.Bool("broken", false, "declare three invalid pages, each broken in its own way, to see the compile error page")
	accent = flag.String("accent", "", "override the theme's accent colour, such as '#2f9e8f'")
)

// Arguments must be string, bool, int, int64 or float64, or a slice of one of them for a
// query parameter that repeats, so a map is a mistake that Compile reports, with the page
// and a fix, rather than a panic. The one field is also the one in the path, so the page
// has the one problem.
type brokenArgs struct{ When map[string]int }

var Broken = webui.Page[brokenArgs]{Path: "/event/{when}", Body: webui.Stack{}}

// A Table is given one source of its rows, and this one is given two: Compile says so, with what each is for.
var BrokenTable = webui.Page[webui.NoArgs]{
	Path: "/broken/source",
	Body: webui.Table[Device]{
		Title: "Events",
		Rows:  func(context.Context) ([]Device, webui.Loaded) { return nil, webui.Ready() },
		Load: func(context.Context, webui.Query) (webui.Window[Device], webui.Loaded) {
			return webui.Window[Device]{}, webui.Ready()
		},
		Columns: []webui.Accessor[Device]{DeviceID},
	},
}

// A pattern is RE2, which has no lookahead, and this one has: Compile says so, and what to write instead.
var BrokenPattern = webui.Page[webui.NoArgs]{
	Path: "/broken/pattern",
	Body: webui.Form[Note]{
		Title: "A code",
		Load:  func(context.Context) (Note, webui.Loaded) { return Note{}, webui.Ready() },
		Fields: []webui.Accessor[Note]{webui.String[Note]{
			Label: "Code",
			Load:  func(n Note) string { return n.Text },
			Store: func(n *Note, v string) { n.Text = v },
			Rules: webui.StringRules{Pattern: &webui.PatternRule{Expr: `(?=.*\d)[a-z0-9]+`, Message: "letters, with a digit somewhere"}},
		}},
		Submit: webui.Action[Note]{Run: func(context.Context, Note) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
	},
}

// brokenPages are the pages that -broken adds, each broken in a way of its own.
var brokenPages = webui.Pages{Broken, BrokenTable, BrokenPattern}

// themeFor overrides the accent. A Theme is four colours (Accent, OK, Warning,
// Critical); the library derives the rest and tunes it for light and dark, which
// are the viewer's choice and not the app's. A value that is not a hex colour is
// a Compile error, so a colour cannot break out of its declaration.
func themeFor(accent string) webui.Theme {
	if accent == "" {
		return webui.Theme{}
	}
	return webui.Theme{Accent: webui.Color(accent)}
}

func main() {
	addr := flag.String("addr", ":7689", "listen address")
	flag.Parse()

	a := app
	a.Theme = themeFor(*accent)
	a.Metrics = registry // what the app counts, to be served below with the program's own
	if *broken {
		a.Pages = append(slices.Clone(a.Pages), brokenPages...)
	}

	// Compile never returns a nil handler: a broken app serves its own errors.
	// MustCompile is the fail-fast form, for a process that should refuse to start.
	handler, err := a.Compile("/admin")
	if err != nil {
		log.Println("webui:", err)
	}

	log.Printf("listening on http://localhost%s/admin/", *addr)
	// A program that takes uploads bounds how long it waits for a request, body included, as ReadHeaderTimeout alone does not.
	srv := &http.Server{Addr: *addr, Handler: routes(handler), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: time.Minute}
	log.Fatal(srv.ListenAndServe())
}

// routes is the whole site: the app behind authentication, and the pages that sign in
// and out. webui does no authentication of its own, so this is where it is.
func routes(app http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/admin/", authenticated(app))
	mux.Handle("GET /uploads/{name}", authenticated(http.HandlerFunc(serveUpload))) // what a File holds, served by the app: see uploads.go
	mux.Handle("GET /uploads/{name}/thumb", authenticated(http.HandlerFunc(serveThumb)))
	mux.Handle("GET /metrics", metricsHandler(registry)) // the program's own, with the app's (App.Metrics) in it
	mux.HandleFunc("GET /login", loginPage)              // the app's menu links here: "Log out"
	mux.Handle("POST /login", http.NewCrossOriginProtection().Handler(http.HandlerFunc(login)))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusFound)
	})
	return mux
}
