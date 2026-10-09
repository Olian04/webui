package webui

import (
	"image"
	"reflect"

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

	// Menu is the app's own links, in a menu behind a button in the top bar: the
	// places outside the app that it wants a visitor to reach, such as signing out,
	// an account page on another service, or the documentation. With none, there is no
	// button.
	Menu []MenuItem
}

// MenuItem is a link in [App.Menu]. It is a [Nav] entry, with the Label that is its
// text, the Icon beside it, and the Section that captions it and the entries after
// it, and with an address outside the app.
type MenuItem struct {
	Nav

	// ExternalURL is where the entry goes, used as written: an http, https or mailto
	// address, or a path elsewhere on the same site such as "/logout". The mount
	// prefix is not added, so it is no way to link to one of the app's own pages;
	// give the page a [Nav] entry for that.
	ExternalURL string
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
	pageArgType() reflect.Type
	validatePage(f *facts) []CompileError
	lowerPage(l *appLowerer) *ir.Page
}

// Pages is the list of pages in an [App]. A [Page] differs by its argument type,
// so the elements are an interface that only [Page] implements.
type Pages []pageLike
