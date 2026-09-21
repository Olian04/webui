// Package webui is the public library surface for this module.
//
// Declaration types live here. Compile lowers them to the internal runtime.
package webui

import "image"

// App is the declaration of an admin UI. Compile produces the runtime handler.
type App struct {
	Brand Brand
	Theme Theme
	Pages Pages
}

// Brand is the product identity shown in the chrome.
type Brand struct {
	Name string
	Logo image.Image
}

// Theme holds visual settings. A zero Theme is the default.
type Theme struct{}

// PageLike is Page[A] with the argument type erased so Pages can mix A.
type pageLike interface {
	isPage()
}

// Pages is the mount list. Page[A] differs per A, so this is an interface slice.
type Pages []pageLike
