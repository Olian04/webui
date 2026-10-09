package ir

// App is one compiled program. It carries no mount prefix: where the app is
// mounted is a transport concern, passed to the runtime separately.
type App struct {
	Brand  Brand
	Theme  Theme
	Pages  []*Page
	ByPath map[string]*Page // destination resolution, not a route table

	// Menu is the app's own links, in the menu in the top bar.
	Menu []MenuItem
}

// MenuItem is one link of the menu, used as written: it is outside the app.
type MenuItem struct {
	Label   string
	Icon    string
	Section string // caption above a run of entries; empty continues the run
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
// for the design's own. The renderer turns them into CSS.
type Theme struct {
	Accent, OK, Warning, Critical string
}
