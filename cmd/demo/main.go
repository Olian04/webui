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
//	activity.go  what is new: a List (a headline, small facts, a badge, a picture, Show more), Cards in a row that scrolls, a Calendar of releases, a When that shows a notice only while critical alerts are open
//	art.go       the pictures that page draws, made from a name and served from this site
//	auth.go      the surrounding authentication: sign in as a viewer or an editor at
//	             /login, and the Guards read the role from the request's context
//
//	go run ./cmd/demo                        # http://localhost:7689/admin/
//	go run ./cmd/demo -accent '#1f7a6d'      # override the theme's accent colour
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

	// The code editor that the Editor and the Diff are drawn with: a program that has either imports it, and one that has
	// neither does not link it. The language services below import it too, so it is written here for the reader.
	_ "github.com/Olian04/webui/pkg/monaco"

	// The JSON language service for the editor: an optional import, since it is a worker of
	// its own. The program is smaller without it, and the editor still highlights.
	_ "github.com/Olian04/webui/pkg/monaco/json"

	// And TypeScript's, which is large (about 7 MB of worker), so it is the one to leave out when
	// the program should be small: the editor then highlights TypeScript and checks nothing.
	_ "github.com/Olian04/webui/pkg/monaco/typescript"
	"github.com/Olian04/webui/pkg/prom"
	"github.com/Olian04/webui/pkg/webui"
)

var app = webui.App{
	// The logo is any image.Image. The theme is left at its default here; the
	// -accent flag overrides the accent colour (see themeFor).
	Brand: webui.Brand{Name: "Collector", Logo: logo()},
	// What the app is, for the pages' meta description: what a search engine shows under the title.
	Description: "Collector: devices, alerts, rules and settings of a telemetry collector, in a small admin panel built with webui.",
	// Where the app says what went wrong that it could not tell the visitor. Left out, it logs nothing.
	Logger: slog.Default(),
	// The links to places outside the app, behind their own button in the top bar: the same entries as the sidebar's. A NavURL is used as written: a web address opens in a tab of its
	// own, a path such as /login in this one.
	Menu: []webui.NavItem{
		webui.NavGroup{Label: "Help", Items: []webui.NavItem{
			webui.NavURL{Label: "Documentation", Icon: "book", URL: "https://pkg.go.dev/github.com/Olian04/webui/pkg/webui"},
			webui.NavURL{Label: "Report a problem", Icon: "bug", URL: "https://github.com/Olian04/webui/issues"},
		}},
		webui.NavGroup{Label: "Account", Items: []webui.NavItem{
			webui.NavURL{Label: "Log out", Icon: "right-from-bracket", URL: "/login"},
		}},
	},
	// The sidebar: categories that open and close, each with an icon, in this order. The one that holds the page on screen is open. A page that is in no entry (a detail page, a form
	// for a new device) lights the entry of its nearest ancestor.
	Navigation: []webui.NavItem{
		webui.NavGroup{Label: "Platform", Icon: "server", Items: []webui.NavItem{
			webui.NavPage{Page: Devices, Label: "Devices", Icon: "display"},
			webui.NavPage{Page: Compare, Label: "Compare devices", Icon: "code-compare"},
			webui.NavPage{Page: Sites, Label: "Sites", Icon: "location-dot"},
			webui.NavPage{Page: Alerts, Label: "Alerts", Icon: "bell"},
			webui.NavPage{Page: Activity, Label: "Activity", Icon: "bolt"},
		}},
		webui.NavGroup{Label: "Sites", Icon: "location-dot", Items: []webui.NavItem{
			// A page with arguments has one entry for each address it lists: each site, which the page loads when a page is drawn. The page of a site lights its entry, and so does
			// any page below it that borrows its place.
			webui.NavPages[SiteArgs]{Page: SiteDetail, Entries: siteEntries, Icon: "location-dot"},
		}},
		webui.NavGroup{Label: "Configuration", Icon: "sliders", Items: []webui.NavItem{
			webui.NavPage{Page: Ingest, Label: "Ingest", Icon: "database"},
			webui.NavPage{Page: Retention, Label: "Retention and archival policy for every kind of collected event"}, // no Icon: the place of one stays empty, and the label is cut short when it is wider than the sidebar
			webui.NavPage{Page: Configuration, Label: "Config", Icon: "file-code"},
			webui.NavPage{Page: PagingRule, Label: "Paging rule", Icon: "code"},
		}},
		webui.NavGroup{Label: "Operations", Icon: "gears", Items: []webui.NavItem{
			webui.NavPage{Page: System, Label: "System", Icon: "wave-square"},
			webui.NavPage{Page: Audit, Label: "Audit log", Icon: "list"},
			webui.NavPage{Page: FirmwarePage, Label: "Firmware", Icon: "database"},
			webui.NavPage{Page: FilesRoot, Label: "Files", Icon: "folder-open"},
		}},
		webui.NavGroup{Label: "Handbook", Icon: "book", Items: []webui.NavItem{
			webui.NavPage{Page: HandbookRunbook, Label: "Runbook", Icon: "book-open"},
			webui.NavPage{Page: HandbookIncidents, Label: "Incidents", Icon: "triangle-exclamation"},
			webui.NavPage{Page: HandbookSinks, Label: "Sinks", Icon: "plug"},
			webui.NavPage{Page: HandbookGlossary, Label: "Glossary", Icon: "spell-check"},
		}},
		webui.NavGroup{Label: "Layout", Icon: "table-columns", Items: []webui.NavItem{
			webui.NavPage{Page: LayoutSplit, Label: "Split rows", Icon: "table-columns"},
			webui.NavPage{Page: LayoutRail, Label: "Main and rail", Icon: "table-cells-large"},
			webui.NavPage{Page: LayoutColumns, Label: "Columns in columns", Icon: "grip"},
			webui.NavPage{Page: LayoutWeights, Label: "Weighted columns", Icon: "table-columns"},
			webui.NavPage{Page: LayoutTabs, Label: "Tabs side by side", Icon: "window-restore"},
			webui.NavPage{Page: LayoutManyTabs, Label: "Many tabs", Icon: "folder-tree"},
			webui.NavPage{Page: LayoutDeep, Label: "Nested five deep", Icon: "layer-group"},
			webui.NavPage{Page: LayoutReview, Label: "Review a change", Icon: "code-compare"},
			webui.NavPage{Page: LayoutRunbook, Label: "Runbook and state", Icon: "book-open"},
			webui.NavPage{Page: LayoutToolbars, Label: "Toolbars", Icon: "grip"},
			webui.NavPage{Page: LayoutList, Label: "A long list", Icon: "list"},
		}},
		webui.NavGroup{Label: "Loading", Icon: "spinner", Items: []webui.NavItem{
			webui.NavPage{Page: LoadingOrder, Label: "In order", Icon: "list-ol"},
			webui.NavPage{Page: LoadingStaircase, Label: "Staircase (10 s)", Icon: "stairs"},
			webui.NavPage{Page: LoadingTabs, Label: "Tabs read one tab", Icon: "window-restore"},
			webui.NavPage{Page: LoadingNested, Label: "Nested layouts", Icon: "layer-group"},
			webui.NavPage{Page: LoadingFailures, Label: "Panels that fail", Icon: "triangle-exclamation"},
			webui.NavPage{Page: LoadingWhole, Label: "Built whole (editor)", Icon: "file-code"},
			webui.NavPage{Page: LoadingGuarded, Label: "Guard refuses", Icon: "shield-halved"},
			webui.NavPage{Page: LoadingFast, Label: "Answers at once", Icon: "bolt"},
		}},
	},
	Pages: webui.Pages{Overview, Devices, NewDevicePage, Details, Compare, Sites, SiteDetail, Alerts, AlertDetails, Activity, ReleaseDay, Ingest, Retention, Configuration, PagingRule, System, Audit, FirmwarePage, FilesRoot, FilesAt, HandbookRunbook, HandbookIncidents, HandbookSinks, HandbookGlossary, LoadingOrder, LoadingStaircase, LoadingTabs, LoadingNested, LoadingFailures, LoadingWhole, LoadingGuarded, LoadingFast, LayoutSplit, LayoutRail, LayoutColumns, LayoutWeights, LayoutTabs, LayoutManyTabs, LayoutDeep, LayoutReview, LayoutRunbook, LayoutToolbars, LayoutList},
}

var (
	broken = flag.Bool("broken", false, "declare three invalid pages, each broken in its own way, to see the compile error page")
	accent = flag.String("accent", "", "override the theme's accent colour, such as '#1f7a6d'")
)

// Arguments must be string, bool, int, int64 or float64, or a slice of one of them for a
// query parameter that repeats, so a map is a mistake that Compile reports, with the page
// and a fix, rather than a panic. The one field is also the one in the path, so the page
// has the one problem.
type brokenArgs struct{ When map[string]int }

var Broken = webui.Page[brokenArgs]{Path: "/event/{when}", View: webui.Stack{}}

// A Table is given one source of its rows, a Loader, and a Table that is given two cannot be written. This one is given a loader that has nothing to load with: Compile says so, with what to
// set.
var BrokenTable = webui.Page[webui.NoArgs]{
	Path: "/broken/source",
	View: webui.Table[Device]{
		Title:   "Events",
		Loader:  webui.RowsLoader[Device]{},
		Columns: []webui.Accessor[Device]{DeviceID},
	},
}

// A pattern is RE2, which has no lookahead, and this one has: Compile says so, and what to write instead.
var BrokenPattern = webui.Page[webui.NoArgs]{
	Path: "/broken/pattern",
	View: webui.Form[Note]{
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
// Critical) for both modes, and, in Light and Dark, what an app gives one mode alone; the
// library derives the rest and tunes it for light and dark, which are the viewer's choice.
// A value that is not a hex colour is a Compile error, so a colour cannot break out of its
// declaration, and one that cannot be read where it is drawn is too.
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
	observer, err := prom.New(registry) // what the app counts, to be served below with the program's own
	if err != nil {
		log.Fatal(err)
	}
	a.Observer = observer
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
	mux.HandleFunc("GET /art/{name}", serveArt)          // the pictures of the Activity page: made from the name, on this site
	mux.Handle("GET /metrics", metricsHandler(registry)) // the program's own, with the app's (App.Observer) in it
	mux.HandleFunc("GET /login", loginPage)              // the app's menu links here: "Log out"
	mux.Handle("POST /login", http.NewCrossOriginProtection().Handler(http.HandlerFunc(login)))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusFound)
	})
	return mux
}
