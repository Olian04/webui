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

// Theme holds visual settings. A zero Theme is the default. Light or dark is
// the viewer's choice, not the app's: it follows their system preference, and
// the sidebar switch overrides it for that browser. Tokens override individual design tokens by name, without the leading
// dashes: {"blue": "#ff6600"}. Values are restricted to colour-like
// characters so a token cannot break out of its declaration.
type Theme struct {
	Tokens map[string]string
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
