package ir

// Nav is what a page knows of its entry in the app's navigation. Shadow is the PathTemplate of the page whose entry stays active while this page is open: lowering infers it, as
// the nearest ancestor path that has an entry in the sidebar.
type Nav struct {
	Label   string // the words the page is called by: its entry's, in the sidebar or else in the menu; empty when it has none
	Shadow  string
	Hidden  bool // no entry of its own in the sidebar
	Dynamic bool // the entries are not declared but loaded, one for each of the page's NavEntries
}

// NavEntry is one entry of a page that lists several: the arguments of the page it opens, and what it says.
type NavEntry struct {
	Args  any
	Label string
}

// NavKind is what an entry of the navigation is.
type NavKind uint8

const (
	NavKindPage  NavKind = iota // opens a page that has no arguments
	NavKindPages                // opens a page, once for each address that the page lists
	NavKindURL                  // opens an address
	NavKindGroup                // holds entries
)

// NavItem is an entry of the sidebar or of the menu. A group holds entries and no other groups.
type NavItem struct {
	Kind  NavKind
	Label string // empty for NavKindPages, whose entries say their own
	Icon  string // a Font Awesome Free solid icon name, or empty
	Path  string // the PathTemplate of the page, for NavKindPage and NavKindPages
	URL   string // for NavKindURL
	Items []NavItem
}
