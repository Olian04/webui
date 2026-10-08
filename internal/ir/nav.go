package ir

// Nav is a navbar entry. Shadow is the PathTemplate of the page whose entry
// stays active while this page is open; the declaration resolves its Nav value
// to that template here.
type Nav struct {
	Label   string
	Icon    string // the name of one of the design's icons; empty draws the label's initial
	Section string // caption above a run of entries; empty continues the run
	Shadow  string
	Hidden  bool // no entry of its own: the page has no Label
}
