package ir

// Nav is a navbar entry. Shadow is the PathTemplate of the page whose entry
// stays active while this page is open: lowering infers it, as the nearest
// ancestor path that has an entry.
type Nav struct {
	Label   string
	Icon    string // a Font Awesome Free solid icon name; empty draws the label's initial
	Section string // the group the entry is listed under, captioned; empty is ungrouped, listed first
	Shadow  string
	Hidden  bool // no entry of its own: the page has no Label
}
