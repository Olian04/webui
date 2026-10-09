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
//	settings.go  forms: rules, Float, Slider, a placeholder
//	system.go    a read-only form; a page guarded for editors; an unknown total
//	auth.go      the surrounding authentication: sign in as a viewer or an editor at
//	             /login, and the Guards read the role from the request's context
//
//	go run ./cmd/demo                        # http://localhost:8080/admin/
//	go run ./cmd/demo -accent '#2f9e8f'      # override the theme's accent colour
//	go run ./cmd/demo -broken                # the failed-to-compile page
package main

import (
	"flag"
	"log"
	"net/http"
	"slices"
	"time"

	// The JSON language service for the editor: an optional import, since it is a worker of
	// its own. The program is smaller without it, and the editor still highlights.
	_ "github.com/Olian04/webui/pkg/monaco/json"
	"github.com/Olian04/webui/pkg/webui"
)

var app = webui.App{
	// The logo is any image.Image. The theme is left at its default here; the
	// -accent flag overrides the accent colour (see themeFor).
	Brand: webui.Brand{Name: "Collector", Logo: logo()},
	// The app's own links, behind the button in the top bar: places outside it. A
	// MenuItem is a Nav entry (a Label, an Icon and a Section) with an ExternalURL,
	// used as written.
	Menu: []webui.MenuItem{
		{Label: "Documentation", Icon: "book", Section: "Help", ExternalURL: "https://pkg.go.dev/github.com/Olian04/webui/pkg/webui"},
		{Label: "Report a problem", Icon: "bug", Section: "Help", ExternalURL: "https://github.com/Olian04/webui/issues"},
		{Label: "Log out", Icon: "right-from-bracket", Section: "Account", ExternalURL: "/login"},
	},
	Pages: webui.Pages{Overview, Devices, Details, Sites, SiteDetail, Alerts, AlertDetails, Ingest, Retention, Configuration, System, Handbooks, Audit},
}

var (
	broken = flag.Bool("broken", false, "declare an invalid page, to see the compile error page")
	accent = flag.String("accent", "", "override the theme's accent colour, such as '#2f9e8f'")
)

// Arguments must be string, bool, int, int64 or float64, so a slice is a mistake
// Compile reports, with the page and a fix, rather than a panic.
type brokenArgs struct{ When []string }

var Broken = webui.Page[brokenArgs]{Path: "/event/{id}", Body: webui.Stack{}}

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
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	a := app
	a.Theme = themeFor(*accent)
	if *broken {
		a.Pages = append(slices.Clone(a.Pages), Broken)
	}

	// Compile never returns a nil handler: a broken app serves its own errors.
	// MustCompile is the fail-fast form, for a process that should refuse to start.
	handler, err := a.Compile("/admin")
	if err != nil {
		log.Println("webui:", err)
	}

	log.Printf("listening on http://localhost%s/admin/", *addr)
	srv := &http.Server{Addr: *addr, Handler: routes(handler), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

// routes is the whole site: the app behind authentication, and the pages that sign in
// and out. webui does no authentication of its own, so this is where it is.
func routes(app http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/admin/", authenticated(app))
	mux.HandleFunc("GET /login", loginPage) // the app's menu links here: "Log out"
	mux.Handle("POST /login", http.NewCrossOriginProtection().Handler(http.HandlerFunc(login)))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusFound)
	})
	return mux
}
