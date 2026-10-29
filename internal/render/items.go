package render

import (
	"maps"
	"strconv"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/markdown"
)

// ItemsView is a list of items with its items loaded. The runtime fills it.
type ItemsView struct {
	Node *ir.Items
	Page *ir.Page

	// Path and Query are the address the page is at, so that Show more is the same page with a larger limit.
	Path  map[string]string
	Query map[string]string

	// Rows are the items that are shown, and Limit is how many the address asked for. More says the source had more.
	Rows  []any
	Limit int
	More  bool

	// Hrefs is where each item leads in the app, "" for one that leads nowhere or that the visitor may not follow.
	Hrefs []string

	Failed  bool // Load failed: the panel says so, the page stands
	Refusal *ir.Refusal
}

// moreHref is this list's address with a larger limit: the same page, with everything else in the address as it was.
func (r *Renderer) moreHref(v ItemsView) string {
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

// external is where an item is on another site, "" for none: the address of its link, which has been checked.
func (v ItemsView) external(row any) string {
	if v.Node.External == nil {
		return ""
	}
	return v.Node.External(row)
}

// href is where item i leads in the app, "" for none.
func (v ItemsView) href(i int) string {
	if i < len(v.Hrefs) {
		return v.Hrefs[i]
	}
	return ""
}

// label is what the list is called, for a reader who is told that a row of cards scrolls.
func (v ItemsView) label() string {
	if v.Node.Title != "" {
		return v.Node.Title
	}
	if v.Node.Cards {
		return "Cards"
	}
	return "Items"
}
