package ir

// App is one compiled program. It carries no mount prefix: where the app is
// mounted is a transport concern, passed to the runtime separately.
type App struct {
	Brand  Brand
	Theme  Theme
	Pages  []*Page
	ByPath map[string]*Page // destination resolution, not a route table
}

// Brand is the product identity in the chrome. Logo is encoded bytes; how to
// serve them is the runtime's choice.
type Brand struct {
	Name string
	Logo []byte
	Mime string
}

// Theme is resolved visual settings. The runtime turns tokens into CSS.
type Theme struct {
	Tokens map[string]string
}
