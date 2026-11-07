package render

import (
	"maps"
	"strconv"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/markdown"
)

// LoadedItems is a list of items with its items loaded. The runtime fills it.
type LoadedItems struct {
	Node *ir.Items
	Page *ir.Page

	// Path and Query are the address the page is at, so that Show more is the same page with a larger limit.
	Path  map[string]string
	Query map[string]string

	// Rows are the items that are shown, and Limit is how many the address asked for. More says the source had more.
	Rows  []any
	Limit int
	More  bool

	// Hrefs is where each item leads, in the app or on another site, "" for one that leads nowhere or that the visitor may not follow.
	Hrefs []string

	// Keys is each item's identity, and ClickRefused whether this viewer may not click it, for a list whose click is an Action; both are empty for any other.
	Keys         []string
	ClickRefused []bool

	Failed  bool // Load failed: the panel says so, the page stands
	Refusal *ir.Refusal
}

// moreHref is this list's address with a larger limit: the same page, with everything else in the address as it was.
func (r *Renderer) moreHref(v LoadedItems) string {
	query := maps.Clone(v.Query)
	if query == nil {
		query = map[string]string{}
	}
	query[args.ViewKey(v.Node.ID, "limit")] = strconv.Itoa(v.Limit + v.Node.PageSize)
	return r.PageHref(v.Page, v.Path, query)
}

// picture is the address an item's picture is drawn from, "" for none: the one the source gave, if the page may fetch it. A picture from anywhere else, or one that a
// browser would read differently from a parser, is not drawn, and the item has none, as an item with none has not.
func picture(n *ir.Items, row any) string {
	if n.Image == nil || n.Image.Get == nil {
		return ""
	}
	src, ok := markdown.Picture(n.Image.Get(row), n.TrustedOrigins)
	if !ok {
		return ""
	}
	return src
}

// isExternal is whether an item leads to another site, which opens in a tab of its own.
func (v LoadedItems) isExternal() bool { return v.Node.Click != nil && v.Node.Click.External != nil }

// posts is whether a click on an item is an action, and so the list is a form.
func (v LoadedItems) posts() bool { return v.Node.Click.Posts() && len(v.Keys) == len(v.Rows) }

// clicks is whether item i runs the list's action when its headline is clicked.
func (v LoadedItems) clicks(i int) bool {
	return v.posts() && (i >= len(v.ClickRefused) || !v.ClickRefused[i])
}

// href is where item i leads, "" for none.
func (v LoadedItems) href(i int) string {
	if i < len(v.Hrefs) {
		return v.Hrefs[i]
	}
	return ""
}

// label is what the list is called, for a reader who is told that a row of cards scrolls.
func (v LoadedItems) label() string {
	if v.Node.Title != "" {
		return v.Node.Title
	}
	if v.Node.Cards {
		return "Cards"
	}
	return "Items"
}
