package webui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/convert"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render/assets"
)

// NavItem is an entry of [App.Navigation] (the sidebar) or [App.Menu] (the links behind the button in the top bar): a [NavPage], a [NavPages], a [NavURL] or a [NavGroup].
// Only this package implements it.
type NavItem interface {
	// validateNav reports what is wrong with the entry. at is where it is declared, such as "App.Navigation[2]".
	validateNav(c *navCheck, at string)

	// lowerNav is the entry in the IR, and tells l what a page that lists several is to list.
	lowerNav(l *navLower) ir.NavItem
}

// NavTarget is a page an entry opens whatever its arguments: a [Page] or its [PageID]. Only this package implements it.
type NavTarget interface {
	pagePath() string
}

// NavPage is an entry that opens a page whose path has no arguments (its query arguments are as they are when nothing is asked for). The page is the page itself or its [PageID]. A page that is in no entry of [App.Navigation] is not in the
// sidebar, and lights the entry of its nearest ancestor: the page whose path is the longest proper prefix of its own that has an entry, which is where a detail page of a list is.
// The page's own Guard decides whether the visitor is offered the entry. The entry's words are also the page's name in a breadcrumb, in the results of a search, and in the
// title of a tab, unless the page has a [Page.Crumb] of its own.
type NavPage struct {
	// Page is the page it opens.
	Page NavTarget

	// Label is what the entry says. It is required.
	Label string

	// Icon is the name of a Font Awesome Free solid icon, "house" for fa-house, drawn beside the label. An unknown name is a compile error. Without one, the place of an icon is
	// left empty, so that the labels line up.
	Icon string
}

// NavPages is an entry for a page that lists an entry for each of several addresses: the sites of a fleet, the projects of a team. The page is the one it opens, and A its arguments,
// so what is listed is addresses of that page and nothing else: Load returns them, and Label says what the entry for each is called. Load is called whenever a full page is drawn. It has a
// deadline of two seconds, the first fifty addresses are kept, an address that Label gives no words is left out, and a failure ([Failed]) or a panic, in Load or in Label, leaves the page's entries
// out and is logged. A [Loaded.RefreshAfter] means nothing in a sidebar and is ignored. The page's Guard is asked with each address, and an entry that it refuses is not drawn. All of them have
// the Icon given here.
type NavPages[A any] struct {
	// Page is the page it opens, with the arguments A.
	Page PageRef[A]

	// Load returns the addresses of the page that are listed: its arguments, with the path ones filled in.
	Load func(ctx context.Context) ([]A, Loaded)

	// Label is what the entry for an address says. It is required, and an address that it gives no words is left out.
	Label func(a A) string

	// Icon is drawn beside each entry, as [NavPage.Icon] is.
	Icon string
}

// NavURL is an entry that opens an address: a path on the same site, such as "/logout", or an http, https or mailto address. The address is used as written, and the mount prefix is
// not added, so it is no way to open a page of the app: give the page a [NavPage] for that. It is the way to a page of the program's own, which is a route beside the app's handler. An address on the web opens in a tab of its own, so the app stays where it was; a path
// on the same site and a mail address open in the same tab.
type NavURL struct {
	// Label is what the entry says. It is required.
	Label string

	// Icon is drawn beside the label, as [NavPage.Icon] is.
	Icon string

	// URL is where it goes: an http, https or mailto address, or a path on the same site. Nothing else is one: a script or a data address in a link is how a stored value becomes code.
	URL string
}

// NavGroup is a category: a name, an icon and the entries under it, which the visitor opens and shuts. In the sidebar it is open when it holds the page on screen, and shut
// otherwise, and what the visitor opens is not kept; only one is open at a time, since the browser shuts the one that was open when another is opened; in the menu it is a caption above its entries. A group holds entries and not other groups, and one with nothing in it that the
// visitor may open is not drawn.
type NavGroup struct {
	// Label is the category's name. It is required.
	Label string

	// Icon is drawn before the name, as [NavPage.Icon] is.
	Icon string

	// Items are the entries under it: a [NavPage], a [NavPages] or a [NavURL] (a [NavPages] only in the sidebar).
	Items []NavItem
}

// navCheck is what the entries are checked against, and where the problems go.
type navCheck struct {
	errs    []CompileError
	mounted map[string]bool   // the paths of the pages that the app serves
	seen    map[string]string // the entries of the list being checked by the page they open, to say where a page is in it twice
	menu    bool              // the list is the menu: it cannot list several pages
	inGroup bool
}

func (c *navCheck) bad(at, detail, fix string) {
	c.errs = append(c.errs, CompileError{Detail: at + " " + detail, Fix: fix})
}

func (c *navCheck) label(at, label string) {
	if label == "" {
		c.bad(at, "has no Label", "Set Label; it is the entry's text.")
	}
}

func (c *navCheck) icon(at, icon string) {
	if icon == "" || assets.HasIcon(icon) {
		return
	}
	fix := "Use the name of a Font Awesome Free solid icon, such as \"book\", or leave Icon empty."
	if after, ok := strings.CutPrefix(icon, "fa-"); ok {
		fix = fmt.Sprintf("Write the name without the fa- prefix: %q.", after)
	}
	c.bad(at, fmt.Sprintf("has the Icon %q, which is not a Font Awesome Free solid icon", icon), fix)
}

// page checks the page that an entry opens: that there is one, that the app serves it, and that no other entry of the list opens it.
func (c *navCheck) page(at, path string, nilRef bool) {
	switch {
	case nilRef:
		c.bad(at, "has no Page", "Set Page to the page it opens, or to its PageID.")
	case !c.mounted[path]:
		c.bad(at, fmt.Sprintf("opens the page %q, which is not in App.Pages", path), "Add the page to App.Pages, or open a page that is there.")
	default:
		if first, ok := c.seen[path]; ok {
			c.bad(at, fmt.Sprintf("opens the page %q, which %s already opens", path, first), "List a page once: its highlight and its Guard are the page's, and are one.")
		} else {
			c.seen[path] = at
		}
	}
}

func (n NavPage) validateNav(c *navCheck, at string) {
	c.label(at, n.Label)
	c.icon(at, n.Icon)
	path := ""
	if n.Page != nil {
		path = n.Page.pagePath()
	}
	c.page(at, path, n.Page == nil)
	if path != "" && len(args.Placeholders(path)) > 0 {
		c.bad(at, fmt.Sprintf("opens the page %q, whose path has arguments and so has no single address", path),
			"Use a NavPages to list an entry for each address of the page, or open a page whose path has none.")
	}
}

func (n NavPages[A]) validateNav(c *navCheck, at string) {
	c.icon(at, n.Icon)
	if c.menu {
		c.bad(at, "is a NavPages, which lists an entry for each of several addresses and so belongs in App.Navigation", "Move it to App.Navigation, or list one address with a NavPage or a NavURL.")
	}
	if n.Load == nil {
		c.bad(at, "has no Load", "Set Load to func(ctx) ([]A, webui.Loaded): the arguments of the page that it lists.")
	}
	if n.Label == nil {
		c.bad(at, "has no Label", "Set Label to func(A) string: what the entry for each address says.")
	}
	path := ""
	if n.Page != nil {
		path = n.Page.pagePath()
	}
	c.page(at, path, n.Page == nil)
}

func (n NavURL) validateNav(c *navCheck, at string) {
	c.label(at, n.Label)
	c.icon(at, n.Icon)
	if !validURL(n.URL) {
		c.bad(at, fmt.Sprintf("has the URL %q, which is not a path on this site, or an http, https or mailto address", n.URL),
			"Use a path such as \"/logout\", or an address such as \"https://example.com/help\".")
	}
}

func (n NavGroup) validateNav(c *navCheck, at string) {
	c.label(at, n.Label)
	c.icon(at, n.Icon)
	if c.inGroup {
		c.bad(at, "is a group inside a group", "A group holds entries, not other groups: list them side by side.")
		return
	}
	if len(n.Items) == 0 {
		c.bad(at, "has no Items", "Put the entries under the group in Items, or remove it.")
	}
	c.inGroup = true
	for i, it := range n.Items {
		itemAt := fmt.Sprintf("%s.Items[%d]", at, i)
		if it == nil {
			c.bad(itemAt, "is nil", "Remove it, or give the entry.")
			continue
		}
		it.validateNav(c, itemAt)
	}
	c.inGroup = false
}

// validateNavigation checks the sidebar and the menu: each entry, and that a page is in the sidebar once.
func (a App) validateNavigation() []CompileError {
	c := &navCheck{mounted: map[string]bool{}}
	for _, p := range a.Pages {
		if p != nil {
			c.mounted[p.pagePath()] = true
		}
	}
	for _, list := range []struct {
		name  string
		items []NavItem
		menu  bool
	}{{"App.Navigation", a.Navigation, false}, {"App.Menu", a.Menu, true}} {
		c.seen, c.menu = map[string]string{}, list.menu
		for i, it := range list.items {
			at := fmt.Sprintf("%s[%d]", list.name, i)
			if it == nil {
				c.bad(at, "is nil", "Remove it, or give the entry.")
				continue
			}
			it.validateNav(c, at)
		}
	}
	return c.errs
}

// navLower is what lowering an entry tells the app: the pages that list several, and what each lists.
type navLower struct {
	entries map[string]func(context.Context) ([]ir.NavEntry, error)
}

func (n NavPage) lowerNav(*navLower) ir.NavItem {
	return ir.NavItem{Kind: ir.NavKindPage, Label: n.Label, Icon: n.Icon, Path: n.Page.pagePath()}
}

func (n NavPages[A]) lowerNav(l *navLower) ir.NavItem {
	path := n.Page.pagePath()
	l.entries[path] = func(ctx context.Context) ([]ir.NavEntry, error) {
		addresses, loaded := n.Load(ctx)
		if err := loaded.apply(ctx); err != nil { // a RefreshAfter does nothing here: only a panel's Load has a hint to keep it in
			return nil, err
		}
		return convert.Slice(addresses, func(a A) ir.NavEntry { return ir.NavEntry{Args: a, Label: n.Label(a)} }), nil
	}
	return ir.NavItem{Kind: ir.NavKindPages, Icon: n.Icon, Path: path}
}

func (n NavURL) lowerNav(*navLower) ir.NavItem {
	return ir.NavItem{Kind: ir.NavKindURL, Label: n.Label, Icon: n.Icon, URL: n.URL}
}

func (n NavGroup) lowerNav(l *navLower) ir.NavItem {
	out := ir.NavItem{Kind: ir.NavKindGroup, Label: n.Label, Icon: n.Icon}
	for _, it := range n.Items {
		out.Items = append(out.Items, it.lowerNav(l))
	}
	return out
}

// walkNav calls f for every entry that opens something, in order, with the group's items inside it.
func walkNav(items []ir.NavItem, f func(ir.NavItem)) {
	for _, it := range items {
		if it.Kind == ir.NavKindGroup {
			walkNav(it.Items, f)
			continue
		}
		f(it)
	}
}

// lowerNavigation resolves the sidebar and the menu to the IR's, and gives each page what it knows of its entry: its words, which are what a breadcrumb, a search and a tab call
// it; whether it has an entry of its own in the sidebar; and, when it has none, the entry that it lights, which is the nearest ancestor's: the longest proper prefix of its path,
// by segments, that is a page with an entry. The root is never an ancestor, so a landing page with an entry does not light for every page in the app.
func (a App) lowerNavigation(out *ir.App) {
	l := &navLower{entries: map[string]func(context.Context) ([]ir.NavEntry, error){}}
	for _, it := range a.Navigation {
		out.Navigation = append(out.Navigation, it.lowerNav(l))
	}
	for _, it := range a.Menu {
		out.Menu = append(out.Menu, it.lowerNav(l))
	}

	inSidebar := map[string]bool{}
	label := map[string]string{} // the words a page is called by, from the sidebar first and then the menu
	walkNav(out.Navigation, func(it ir.NavItem) {
		if it.Path != "" {
			inSidebar[it.Path] = true
			if it.Label != "" && label[it.Path] == "" {
				label[it.Path] = it.Label
			}
		}
	})
	walkNav(out.Menu, func(it ir.NavItem) {
		if it.Path != "" && it.Label != "" && label[it.Path] == "" {
			label[it.Path] = it.Label
		}
	})

	for _, page := range out.Pages {
		path := page.PathTemplate
		dynamic := l.entries[path] != nil
		page.NavEntries = l.entries[path]
		page.Nav = ir.Nav{Label: label[path], Hidden: !inSidebar[path], Dynamic: dynamic}
		if inSidebar[path] {
			continue
		}
		segs := strings.Split(strings.Trim(path, "/"), "/")
		for i := len(segs) - 1; i >= 1; i-- {
			if prefix := "/" + strings.Join(segs[:i], "/"); inSidebar[prefix] {
				page.Nav.Shadow = prefix
				break
			}
		}
	}
}
