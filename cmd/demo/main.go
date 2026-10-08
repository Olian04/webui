// Command demo serves a small admin panel built with webui, over in-memory
// data. It exists to look at the library working; it is not part of its API.
//
// The app is a static config: every page, accessor and action is a top-level
// var, read top to bottom. devices.go is the place to start, and each file shows
// its own corner of the library:
//
//	devices.go   a table (paging, sorting, filters, search, row links), a form
//	             with tabs, a path and a query argument, a rejection, a redirect
//	sites.go     a nested path, two stateful tables on one page, more search
//	alerts.go    row and bulk actions, a destructive role, links with arguments
//	settings.go  forms: rules, Float, Slider, Placeholder
//	system.go    a read-only form; a page guarded for editors; an unknown total
//
//	go run ./cmd/demo                        # http://localhost:8080/admin/
//	go run ./cmd/demo -viewer                # guarded controls and pages are refused
//	go run ./cmd/demo -accent '#2f9e8f'      # override the theme's accent colour
//	go run ./cmd/demo -broken                # the failed-to-compile page
package main

import (
	"flag"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/Olian04/webui/pkg/webui"
)

var app = webui.App{
	// The logo is any image.Image. The theme is left at its default here; the
	// -accent flag overrides one token (see themeFor).
	Brand: webui.Brand{Name: "Collector", Logo: logo()},
	Pages: webui.Pages{Devices, Details, Sites, SiteDetail, Alerts, Ingest, Retention, System, Audit},
}

var (
	broken = flag.Bool("broken", false, "declare an invalid page, to see the compile error page")
	accent = flag.String("accent", "", "override the theme's accent colour, such as '#2f9e8f'")
)

// Arguments must be string, bool, int, int64 or float64, so a slice is a mistake
// Compile reports, with the page and a fix, rather than a panic.
type brokenArgs struct{ When []string }

var Broken = webui.Page[brokenArgs]{Path: "/event/{id}", Body: webui.Stack{}}

// themeFor overrides the accent. A Theme is a set of design tokens by name, without
// the leading dashes; light or dark is the viewer's choice and not the app's. A
// value that is not a colour or a length is a Compile error, so a token cannot
// break out of its declaration.
func themeFor(accent string) webui.Theme {
	if accent == "" {
		return webui.Theme{}
	}
	return webui.Theme{Tokens: map[string]string{"blue": accent, "blue-hover": accent, "blue-text": accent}}
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

	mux := http.NewServeMux()
	mux.Handle("/admin/", handler)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusFound)
	})
	log.Printf("listening on http://localhost%s/admin/", *addr)
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
