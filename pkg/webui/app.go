package webui

import (
	"image"
	"log/slog"
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
	// Compiling again, or compiling another app with the same registry, is not an error: the
	// metrics that the registry already has are counted into, so an app that replaces the one that was
	// serving keeps its counts. Two different apps then add their counts together where a page has
	// the same path template: give each its own wrapper from [prometheus.WrapRegistererWith], with a
	// label of its own, to tell them apart. A metric of another kind with the name of a webui one
	// is an error.
	Metrics prometheus.Registerer

	// Logger is where the app says what went wrong that it could not tell the visitor: a Load that
	// failed, a panic in the application's code, a table export that was cut. It is a
	// [log/slog.Logger], so the application decides where the lines go and at what level. Left nil,
	// the app logs nothing at all: it does not fall back to the default logger, so a library that
	// is mounted in a program is silent until that program asks otherwise.
	//
	//	handler := webui.App{Logger: slog.Default(), Pages: pages}.MustCompile("/admin")
	//
	// Every message begins with "webui:", and names the page by its path template where it is about
	// one. An error's cause is in the log and never on the page, since it may name what the visitor
	// should not see.
	Logger *slog.Logger

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

// Mode is the theme an app is drawn in: left empty (ModeAuto) it is the viewer's choice
// between light and dark, which they make with the button in the top bar and which
// follows their system until they do. An app may pin itself to one.
type Mode string

const (
	// ModeAuto leaves light or dark to the viewer. It is the default.
	ModeAuto Mode = ""

	// ModeDark draws the app in the dark theme only: the page has no theme button, and a
	// choice that the viewer made in another app on the same site is not followed.
	ModeDark Mode = "dark"

	// ModeLight draws the app in the light theme only, in the same way.
	ModeLight Mode = "light"
)

// Theme is what an app may restyle. Left at its zero value, the app has the design as it is.
//
// Four colours carry meaning: Accent, OK, Warning and Critical. Each is the colour as the
// design draws it; the library derives the rest (hover, the readable text tint, the faint
// background and border of a badge) and tunes them for light and dark, which are the
// viewer's choice unless the app pins a Mode. They work in either.
//
// Canvas is the page's background, and from it the library derives every other surface, the
// borders and the text, so that a theme is a whole look and not four spots of colour on the
// design's own. It is one colour, which cannot be right in both light and dark, so it needs a
// Mode: Canvas without one is a compile error. The presets (ThemeSlate, ThemeMidnight and
// the others) are Themes of this kind, and an app may start from one and change a field.
//
// Compile checks that what it derives can be read: the text on the surfaces it is drawn on
// at 4.5:1, and what is only decoration at 3:1, in each mode the theme is drawn in. A colour
// that fails is an error that names the field, the pair and what to change, and not a button
// whose label is out of sight.
type Theme struct {
	// Accent is interactive: primary buttons, links, the active entry in the
	// sidebar, the focus ring. White is drawn on it, so it must be deep enough for that.
	Accent Color

	// OK is healthy, saved: a badge with ToneOK, and the toast of a Success.
	OK Color

	// Warning is degraded, needs a look: a badge with ToneWarning.
	Warning Color

	// Critical is failed, destructive: a badge with ToneCritical, error text, the
	// destructive button, which has a white label.
	Critical Color

	// Canvas is the page's background in Mode, an opaque colour. The panels, what is raised
	// above them, inputs, borders, hover and text are derived from it. It needs a Mode.
	Canvas Color

	// Mode pins the app to the dark or the light theme. Empty, the viewer chooses.
	Mode Mode
}

// pageLike is Page[A] with the argument type erased so Pages can mix A.
type pageLike interface {
	pagePath() string
	hasNavEntry() bool
	pageArgType() reflect.Type
	validatePage(f *facts) []CompileError
	lowerPage(entries map[string]bool) *ir.Page
}

// Pages is the list of pages in an [App]. A [Page] differs by its argument type,
// so the elements are an interface that only [Page] implements.
type Pages []pageLike
