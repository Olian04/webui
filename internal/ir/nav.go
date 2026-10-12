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
	Dynamic bool // the entries are not declared but loaded, one for each of the page's NavEntries
}

// NavEntry is one entry of a page that lists several: the arguments of the page it opens, and what it says.
type NavEntry struct {
	Args  any
	Label string
}
