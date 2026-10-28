package webui

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/convert"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render/assets"
)

// NoArgs is the argument type of a page with no path or query parameters, as in
// Page[NoArgs].
type NoArgs struct{}

// PageID is a page's path, tied to its argument type. Declare one as a constant
// when two pages link to each other:
//
//	const DevicesPath webui.PageID[webui.NoArgs] = "/device"
//
// and use it as the page's Path and, from the other page, in Open or Link. A
// constant is not a package-level variable, so naming it does not make the pages
// depend on each other: referring back by the page variable would be an
// initialization cycle, which Go rejects at build time. A page and its ID agree
// on A, or it does not compile.
type PageID[A any] string

func (id PageID[A]) pagePath() string { return string(id) }

func (PageID[A]) accepts(A) {}

// PageRef is what Open and Link take to name a destination: a Page, or just its
// PageID. Both carry the argument type, so the arguments given for a page are
// checked against it. Only this package implements it.
type PageRef[A any] interface {
	pagePath() string
	accepts(A)
}

func (Page[A]) accepts(A) {}

// Page is a route. A is the argument struct: path fields match {placeholders},
// the rest are query parameters.
type Page[A any] struct {
	// Path is the page's address, such as "/device/{id}". It is typed by A, so a
	// PageID declared for one page's arguments cannot be given to another; a
	// string literal or an untyped constant converts without ceremony.
	//
	// The last segment may be {name...}, which takes the rest of the address, and only the
	// last may be one. Its field must be a []string with one element for each segment: for
	// "/bucket/{name}/{key...}" the key of "/bucket/photos/2026/summer" is
	// ["2026", "summer"]. Each segment is a breadcrumb that links to the page at that
	// depth, so a nested folder reads as the folders on the way to it.
	Path PageID[A]

	// Nav is the page's entry in the sidebar. A page with no Label has no entry.
	Nav Nav

	// Guard decides whether the visitor may open the page. It runs before anything
	// is loaded, on every request to the page, with the decoded arguments. A
	// non-nil error answers 403, and its text is shown as the reason, unless it is
	// a [Refusal], which answers with its own status (404, 401 or 403) and says what it
	// says, with a link on if it has one. The pages a Guard refuses are also left out
	// of the sidebar and the search.
	Guard func(ctx context.Context, a A) error

	// NavEntries gives a page with arguments entries of its own in the sidebar: one for each
	// address of the page that it returns, such as one for each site of a page at
	// "/site/{name}". A page with arguments has no entry otherwise, since an entry is a link with
	// one address and a path with {arguments} has none until they are filled in.
	//
	// Each [NavEntry] is the page's arguments and the words the entry says. [Nav]'s Icon and
	// Section are the icon and the group of all of them; its Label must be empty, since each
	// entry has its own. A page's pages that borrow its place, such as "/site/{name}/log", light
	// the entry of the same site.
	//
	// It is called whenever a full page is drawn, for every visitor, so it must be quick and
	// not rely on what the page's Guard would refuse: the library calls it with a deadline of two seconds,
	// keeps the first 50 entries, and runs the Guard for each, with the entry's arguments, leaving
	// out those that would be refused. A failure, or a panic, leaves the entries out and the page
	// as it is.
	NavEntries func(ctx context.Context) ([]NavEntry[A], error)

	// Crumb is what the page's crumb says in the breadcrumb and the tab title, for the arguments it was opened with,
	// where the path would say the value it took: a page at "/device/{id}" that returns the device's name reads
	// "Stockholm gateway" and not "dev_0041". It also names the page where it is a parent in another page's
	// breadcrumb, such as "/device/{id}/log", whose arguments the path then has all of. A page whose parent has an
	// argument the path does not have, such as one in the query, is asked with the argument empty.
	//
	// It is called whenever a full page is drawn, so it must be quick and may not rely on what the page's Guard
	// would refuse: the library calls it with a deadline of two seconds for all the crumbs of a page together, does
	// not call it for a parent whose Guard refuses the visitor, and a failure, a panic or an empty answer leaves
	// the words of the path as they were. It cannot be set on a page whose path ends in a {name...}, whose crumbs
	// are the segments of that value.
	Crumb func(ctx context.Context, a A) string

	// Search makes the things this page is the page of findable from the search box in
	// the top bar. It is handed what the visitor typed and the most it should return, and
	// returns the matches: each a [Hit], the arguments of this page that lead to it, and
	// what to say about it. A page for one device finds devices, and a result is a link
	// to the page at that device.
	//
	// It is called for every visitor whenever they search, with the arguments the page
	// has when nothing is selected, so it returns what any of them could look for and does
	// not rely on the page's Guard. The library runs that Guard for each hit, with the
	// hit's own arguments, and offers a result only if the visitor could open it. The results
	// are shown under the page's [Nav] label, or under the entry it borrows its place from.
	//
	// Match however the source can, ignoring case, against whatever the visitor sees of a
	// result. [SearchRows] makes one from rows held in memory. If it returns an error the
	// page is left out of the search, the cause is logged, and the rest are searched.
	// A page with no Search is not searched, though its [Nav] entry is still listed.
	Search func(ctx context.Context, query string, limit int) ([]Hit[A], error)

	// Body is what the page shows: a [Table], a [Form], or a layout of them.
	Body PageBody
}

// Nav is a navbar entry. A page with an empty Label has no entry of its own, and
// lights the entry of its nearest ancestor: the page whose path is the longest
// prefix of its own that has a Label. A page at "/device/{id}" borrows the
// "Devices" entry of the page at "/device", so there is nothing to declare. A page whose
// path has arguments has no single address to link to, so it has entries only through
// [Page.NavEntries], which lists one for each address.
type Nav struct {
	// Label is the entry's text. A page with no Label has no entry of its own.
	Label string

	// Icon is the name of a Font Awesome Free solid icon, "house" for fa-house,
	// drawn beside the label. An unknown name is a compile error. Without one, the
	// label's first letter, capitalised, is shown where the sidebar is collapsed to
	// icons.
	Icon string

	// Section is the caption of the group this entry is listed in. The list is
	// grouped by Section and not by position: the entries with no Section come
	// first, then each section in the order it is first named, with its entries in
	// the order the pages are given. An entry belongs to a section only by saying
	// so.
	Section string
}

// PageBody is what a page shows: a leaf, [Table], [Form], [Editor], [Diff], [Markdown], [Stat], [Toolbar] or [FileUpload], or a layout of them,
// [Stack], [Split] or [Tabs]. Only this package implements it.
type PageBody interface {
	validateBody(v *bodyValidator)
	lowerBody(at ir.Addr) ir.Node
}

// NavEntry is one entry in the sidebar of a [Page] that has [Page.NavEntries]: an address
// of the page, and what it says.
type NavEntry[A any] struct {
	// Args are the page's arguments that the entry opens, with the path ones filled in.
	Args A

	// Label is what the entry says. An entry with none is left out.
	Label string
}

// hasNavEntry is whether the page has an entry of its own in the sidebar, or lists several.
func (p Page[A]) hasNavEntry() bool { return p.Nav.Label != "" || p.NavEntries != nil }

func (p Page[A]) pagePath() string { return string(p.Path) }

func (p Page[A]) pageArgType() reflect.Type { return reflect.TypeFor[A]() }

// ASSERT: Page implements pageLike
var _ pageLike = Page[struct{}]{}

var (
	placeholderName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	reservedPrefix  = "/_webui"
)

func (p Page[A]) validatePage(f *facts) []CompileError {
	typ := reflect.TypeFor[A]()
	path := string(p.Path)
	v := &bodyValidator{page: path, args: typeName(typ), argType: typ, facts: f}

	if detail, fix := validatePath(path); detail != "" {
		v.add(detail, fix)
		return v.errs
	}
	if f.paths[path] > 1 {
		v.add("two pages declare this path",
			"Give each page a distinct Path; the router would pick one and the other would be dead.")
	}

	_, problems := args.Spec(typ, path)
	for _, pr := range problems {
		v.add(pr.Detail, pr.Fix)
	}
	if typ.Kind() != reflect.Struct {
		return v.errs // every later check reads A's fields
	}

	if p.Nav.Label != "" && len(args.Placeholders(path)) > 0 {
		v.add(fmt.Sprintf("the page has the navigation entry %q but its path has arguments", p.Nav.Label),
			"A sidebar entry is a link with one address, and a path with {arguments} has none until they are filled in. Remove Nav.Label, give the page NavEntries to list an entry for each address, or link to the page from another page's table with a Link.")
	}
	if p.NavEntries != nil && p.Nav.Label != "" {
		v.add(fmt.Sprintf("the page has NavEntries and the navigation label %q", p.Nav.Label),
			"Each entry says its own words, in NavEntry.Label. Remove Nav.Label; Nav.Icon and Nav.Section are the icon and the group of all the entries.")
	}
	if p.Crumb != nil {
		segs := strings.Split(strings.Trim(path, "/"), "/")
		if _, rest, ok := args.Placeholder(segs[len(segs)-1]); ok && rest {
			v.add("the page has a Crumb and its path ends in a {name...}",
				"The crumbs of a path that takes the rest are its segments, one for each. Remove Crumb, or end the path in a placeholder that takes one segment.")
		}
	}
	if p.Nav.Icon != "" && !assets.HasIcon(p.Nav.Icon) {
		fix := "Use the name of a Font Awesome Free solid icon, such as \"house\", or leave Icon empty."
		if after, ok := strings.CutPrefix(p.Nav.Icon, "fa-"); ok {
			fix = fmt.Sprintf("Write the icon's name without the \"fa-\" prefix: %q.", after)
		}
		v.add(fmt.Sprintf("Nav.Icon %q is not a Font Awesome Free solid icon", p.Nav.Icon), fix)
	}
	if p.Body == nil {
		v.add("Body is nil", "Set Body to a Table, Form, Editor, Diff, Markdown, Stat, Toolbar, FileUpload, Stack, Split or Tabs.")
		return v.errs
	}
	p.Body.validateBody(v)
	return v.errs
}

// validatePath is the first thing wrong with a path, and how to put it right; empty if nothing is.
func validatePath(path string) (detail, fix string) {
	if path == "" || path[0] != '/' {
		return "Path must start with \"/\"", "Write the path as \"/device\" or \"/device/{id}\"."
	}
	if strings.ContainsAny(path, "?#") {
		return "Path must not contain a query or fragment", "Declare query parameters as fields on the argument struct."
	}
	if path == reservedPrefix || strings.HasPrefix(path, reservedPrefix+"/") {
		return "Path is under the reserved prefix " + reservedPrefix,
			"Choose another path; " + reservedPrefix + " serves the framework's own assets."
	}
	if path == "/" {
		return "", ""
	}
	segs := strings.Split(path[1:], "/")
	for i, seg := range segs {
		switch {
		case seg == "":
			return "Path has an empty segment", "Remove the doubled or trailing slash."
		case strings.ContainsAny(seg, "{}"):
			name, rest, ok := args.Placeholder(seg)
			if !ok || !placeholderName.MatchString(name) {
				return "Path segment " + seg + " is not a valid placeholder",
					"A placeholder is a whole segment of the form {name}, or {name...} for the rest of the path."
			}
			if rest && i != len(segs)-1 {
				return "Path segment " + seg + " takes the rest of the path but is not the last",
					"Only the last segment may be {name...}; it matches every segment from there on."
			}
		}
	}
	return "", ""
}

func typeName(t reflect.Type) string {
	if n := t.Name(); n != "" {
		return n
	}
	return t.String()
}

func (p Page[A]) lowerPage(entries map[string]bool) *ir.Page {
	typ := reflect.TypeFor[A]()
	specs, _ := args.Spec(typ, string(p.Path))
	codec := args.NewCodec(typ, specs)

	page := &ir.Page{
		PathTemplate: string(p.Path),
		Args:         specs,
		Nav:          lowerNav(entries, p.Nav, string(p.Path), p.NavEntries != nil),
		Decode:       codec.Decode,
		Encode:       codec.Encode,
	}
	if p.Guard != nil {
		page.Guard = func(ctx context.Context, a any) error { return refused(p.Guard(ctx, a.(A))) }
	}
	if p.NavEntries != nil {
		page.NavEntries = func(ctx context.Context) ([]ir.NavEntry, error) {
			entries, err := p.NavEntries(ctx)
			if err != nil {
				return nil, err
			}
			return convert.Slice(entries, func(e NavEntry[A]) ir.NavEntry { return ir.NavEntry{Args: e.Args, Label: e.Label} }), nil
		}
	}
	if p.Crumb != nil {
		page.Crumb = func(ctx context.Context, a any) string { return p.Crumb(ctx, a.(A)) }
	}
	if p.Search != nil {
		page.Search = func(ctx context.Context, query string, limit int) ([]ir.Hit, error) {
			found, err := p.Search(ctx, query, limit)
			if err != nil {
				return nil, err
			}
			return convert.Slice(found, func(h Hit[A]) ir.Hit { return ir.Hit{Args: h.Args, Title: h.Title, Desc: h.Desc} }), nil
		}
	}
	page.Body = p.Body.lowerBody(ir.Addr{})
	assignViewIDs(page.Body)
	return page
}
