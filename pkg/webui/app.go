// Package webui is the public library surface for this module.
//
// Declaration types live here. Compile lowers them to the internal runtime.
package webui

import (
	"image"
	"reflect"

	"github.com/Olian04/webui/internal/ir"
)

// App is the declaration of an admin UI. Compile produces the runtime handler.
type App struct {
	Brand Brand
	Theme Theme
	Pages Pages
}

// Brand is the product identity shown in the chrome.
//
// The browser tab's icon is made from Logo: scaled to the sizes browsers ask
// for and served by the app, so there is no file to produce and host. An app with
// no Logo gets the library's own mark. NoFavicon turns that off.
type Brand struct {
	Name string
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

	// OK is healthy, saved, loaded: a badge with ToneOK, the loaded mark.
	OK Color

	// Warning is degraded, needs a look: a badge with ToneWarning.
	Warning Color

	// Critical is failed, destructive: a badge with ToneCritical, error text, the
	// destructive button.
	Critical Color
}

// PageLike is Page[A] with the argument type erased so Pages can mix A.
type pageLike interface {
	isPage()
	pagePath() string
	pageNav() Nav
	pageArgType() reflect.Type
	validatePage(f *facts) []CompileError
	lowerPage(l *appLowerer) *ir.Page
}

// Pages is the mount list. Page[A] differs per A, so this is an interface slice.
type Pages []pageLike
