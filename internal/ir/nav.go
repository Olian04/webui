package ir

// Nav is a navbar entry. Shadow is the PathTemplate of the page whose entry
// stays active while this page is open; the declaration's pointer identity is
// resolved to that template here.
type Nav struct {
	Label  string
	Shadow string
	Hidden bool
}
