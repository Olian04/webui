package webui

import (
	"image"
	"reflect"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Olian04/webui/internal/ir"
)

// App is the declaration of an admin UI. Compile produces the runtime handler.
type App struct {
	// Brand is the name and logo shown in the chrome.
	Brand Brand

	// Theme restyles the app's colours.
	Theme Theme

	// Pages are the pages to serve. Each page that another links to must be here.
	Pages Pages

	// ExternalLinks are the app's links to places outside it, behind a button in the top
	// bar that is not part of the sidebar: signing out, an account page on another
	// service, the documentation. With none, there is no button.
	ExternalLinks []ExternalLink

	// Metrics is the registry the app puts its own metrics in, or nil to count nothing.
	// They are Prometheus metrics, all named webui_..., about what the app does: the
	// requests each page answers and how long they take, how long the application's Load
	// functions take and how often they fail, what actions come to, searches, exports, and
	// the sidebar entries of pages that list several. The labels are low in number by
	// design: the path template of a page ("/device/{id}") and never an address that was
	// asked for, a method, a status, and a few fixed names. It has no Go runtime or process
	// metrics: the application has its own for those.
	//
	// The app serves none of it. The application registers a registry of its own, as it
	// does for its other metrics, and serves that where it serves them, so what is
	// exposed, where, and to whom stay its decisions:
	//
	//	reg := prometheus.NewRegistry()
	//	handler := webui.App{Metrics: reg, Pages: pages}.MustCompile("/admin")
	//	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	//
	// Compile fails if the registry already has a webui metric, which is two apps given one
	// registry: give each its own wrapper from [prometheus.WrapRegistererWith], with a
	// label of its own, to tell them apart.
	Metrics prometheus.Registerer

	// Secret keys the addresses of an [Editor]'s Context files. They are served from addresses
	// that carry a key, which a page gives only to a visitor who may edit the editor, so the
	// files are as private as the page is. Left empty, the app makes a random secret when it
	// starts, which is right for one process. Behind more than one instance, set the same
	// value, of at least 16 bytes, on every one: a page drawn by one instance hands out
	// addresses that another must accept. Changing it, like restarting, changes the addresses,
	// so a browser fetches the files again once.
	Secret []byte
}

// ExternalLink is a link in [App.ExternalLinks]. It is a [Nav] entry, with the Label
// that is its text, the Icon beside it, and the Section that captions it, and with an
// address outside the app.
//
// An address on the web opens in a tab of its own, so the app stays where it was; a path
// on the same site, such as "/logout", and a mail address, open in the same tab.
//
// Like the sidebar, the list is grouped by Section and not by position: the
// entries with no Section come first, and each Section follows, in the order it is
// first named, with its entries in the order they were given. An entry belongs to
// a section only by saying so.
type ExternalLink struct {
	Nav

	// URL is where the link goes, used as written: an http, https or mailto
	// address, or a path elsewhere on the same site such as "/logout". The mount
	// prefix is not added, so it is no way to link to one of the app's own pages;
	// give the page a [Nav] entry for that.
	URL string
}

// Brand is the product identity shown in the chrome.
//
// The browser tab's icon is made from Logo: scaled to the sizes browsers ask
// for and served by the app, so there is no file to produce and host. An app with
// no Logo gets the library's own mark. NoFavicon turns that off.
type Brand struct {
	// Name is the product's name, shown in the sidebar and as the first
	// breadcrumb, and in the tab title.
	Name string

	// Logo is shown in the sidebar and scaled into the browser's favicon. Any
	// [image.Image] will do.
	Logo image.Image

	// NoFavicon stops the favicon being generated and served, for an app that
	// brings its own.
	NoFavicon bool
}

// Color is a colour written as a hex string: "#3d71d9", or "#3d7" short, or
// "#3d71d9cc" with an alpha. Compile refuses anything else, so a colour cannot
// close the declaration it is written into.
type Color string

// Theme is what an app may restyle: the four colours that carry meaning. Each is
// the colour as the design draws it; the library derives the rest (hover, the
// readable text tint, the faint background and border of a badge) and tunes
// them for light and dark, which are the viewer's choice and not the app's. A
// field left empty keeps the design's own colour, so a zero Theme is the default.
//
// Surfaces and text are not themeable: they are what makes light and dark differ,
// and a single colour cannot be right in both.
type Theme struct {
	// Accent is interactive: primary buttons, links, the active entry in the
	// sidebar, the focus ring and the selected tab.
	Accent Color

	// OK is healthy, saved: a badge with ToneOK, and the toast of a Success.
	OK Color

	// Warning is degraded, needs a look: a badge with ToneWarning.
	Warning Color

	// Critical is failed, destructive: a badge with ToneCritical, error text, the
	// destructive button.
	Critical Color
}

// pageLike is Page[A] with the argument type erased so Pages can mix A.
type pageLike interface {
	isPage()
	pagePath() string
	pageNav() Nav
	hasNavEntries() bool
	pageArgType() reflect.Type
	validatePage(f *facts) []CompileError
	lowerPage(l *appLowerer) *ir.Page
}

// Pages is the list of pages in an [App]. A [Page] differs by its argument type,
// so the elements are an interface that only [Page] implements.
type Pages []pageLike
