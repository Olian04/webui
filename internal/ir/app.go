package ir

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
)

// App is one compiled program. It carries no mount prefix: where the app is
// mounted is a transport concern, passed to the runtime separately.
type App struct {
	Brand  Brand
	Theme  Theme
	Pages  []*Page
	ByPath map[string]*Page // destination resolution, not a route table

	// ExternalLinks are the app's own links to places outside it, behind a button in the
	// top bar.
	ExternalLinks []ExternalLink

	// Metrics is where the runtime registers what it counts, or nil for nothing counted.
	Metrics prometheus.Registerer

	// Logger is where the runtime says what went wrong that it could not tell the visitor, or nil to say nothing.
	Logger *slog.Logger

	// Secret keys the addresses of the files the application hands only to the visitors it
	// has shown an editor to. Empty: the runtime makes a random one.
	Secret []byte
}

// ExternalLink is one of those links, used as written: it is outside the app.
type ExternalLink struct {
	Label   string
	Icon    string
	Section string // the group the entry is listed under, captioned; empty is ungrouped, listed first
	URL     string
}

// Brand is the product identity in the chrome. Logo is encoded bytes; how to
// serve them is the runtime's choice.
type Brand struct {
	Name string
	Logo []byte
	Mime string

	// Favicons are the logo scaled to the sizes a browser asks for, or none when
	// there is no logo or the declaration turned them off. DefaultFavicon asks for
	// the library's own mark instead, for an app that has no logo.
	Favicons       []Favicon
	DefaultFavicon bool
}

// Favicon is the logo scaled to a Size x Size square, as PNG.
type Favicon struct {
	Size int
	PNG  []byte
}

// Theme is resolved visual settings: the colours the app chose, as hex, or empty
// for the design's own, and the mode it is pinned to ("dark" or "light") or "" for
// the viewer's choice. The renderer turns them into CSS.
type Theme struct {
	Accent, OK, Warning, Critical string
	Canvas, Mode                  string
}
