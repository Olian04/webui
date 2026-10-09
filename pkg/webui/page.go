package webui

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/Olian04/webui/internal/args"
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
	// The last segment may be {name...}, which takes the rest of the address: for
	// "/bucket/{name}/{key...}" the key of "/bucket/photos/2026/summer" is
	// "2026/summer". Its field must be a string. Each segment of the value is a
	// breadcrumb that links to the page at that depth, so a nested folder reads as the
	// folders on the way to it.
	Path PageID[A]

	// Nav is the page's entry in the sidebar. A page with no Label has no entry.
	Nav Nav

	// Guard decides whether the visitor may open the page. It runs before anything
	// is loaded, on every request to the page, with the decoded arguments. A
	// non-nil error answers 403, and its text is shown as the reason. The pages a
	// Guard refuses are also left out of the sidebar and the search.
	Guard func(ctx context.Context, a A) error

	// Body is what the page shows: a [Table], a [Form], or a layout of them.
	Body PageBody
}

// Nav is a navbar entry. A page with an empty Label has no entry of its own, and
// lights the entry of its nearest ancestor: the page whose path is the longest
// prefix of its own that has a Label. A page at "/device/{id}" borrows the
// "Devices" entry of the page at "/device", so there is nothing to declare.
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

// PageBody is what a page shows: a leaf, [Table], [Form], [Editor] or [Diff], or a layout of them,
// [Stack], [Split] or [Tabs]. Only this package implements it.
type PageBody interface {
	isPageBody()
	validateBody(v *bodyValidator)
	lowerBody(at ir.Addr) ir.Node
}

func (Page[A]) isPage() {}

func (p Page[A]) pagePath() string { return string(p.Path) }

func (p Page[A]) pageNav() Nav { return p.Nav }

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
	v := &bodyValidator{page: path, args: typeName(typ), facts: f}

	if errs := validatePath(path); len(errs) > 0 {
		for _, e := range errs {
			v.add(e.Detail, e.Fix)
		}
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
			"A sidebar entry is a link with one address, and a path with {arguments} has none until they are filled in. Remove Nav, or link to the page from another page's table with a Link.")
	}
	if p.Nav.Icon != "" && !assets.HasIcon(p.Nav.Icon) {
		fix := "Use the name of a Font Awesome Free solid icon, such as \"house\", or leave Icon empty."
		if strings.HasPrefix(p.Nav.Icon, "fa-") {
			fix = fmt.Sprintf("Write the icon's name without the \"fa-\" prefix: %q.", strings.TrimPrefix(p.Nav.Icon, "fa-"))
		}
		v.add(fmt.Sprintf("Nav.Icon %q is not a Font Awesome Free solid icon", p.Nav.Icon), fix)
	}
	if p.Body == nil {
		v.add("Body is nil", "Set Body to a Table, Form, Editor, Diff, Stack, Split or Tabs.")
		return v.errs
	}
	p.Body.validateBody(v)
	return v.errs
}

func validatePath(path string) []CompileError {
	bad := func(detail, fix string) []CompileError { return []CompileError{{Detail: detail, Fix: fix}} }
	if path == "" || path[0] != '/' {
		return bad("Path must start with \"/\"", "Write the path as \"/device\" or \"/device/{id}\".")
	}
	if strings.ContainsAny(path, "?#") {
		return bad("Path must not contain a query or fragment",
			"Declare query parameters as fields on the argument struct.")
	}
	if path == reservedPrefix || strings.HasPrefix(path, reservedPrefix+"/") {
		return bad("Path is under the reserved prefix "+reservedPrefix,
			"Choose another path; "+reservedPrefix+" serves the framework's own assets.")
	}
	if path == "/" {
		return nil
	}
	segs := strings.Split(path[1:], "/")
	for i, seg := range segs {
		switch {
		case seg == "":
			return bad("Path has an empty segment", "Remove the doubled or trailing slash.")
		case strings.ContainsAny(seg, "{}"):
			name, rest, ok := args.Placeholder(seg)
			if !ok || !placeholderName.MatchString(name) {
				return bad("Path segment "+seg+" is not a valid placeholder",
					"A placeholder is a whole segment of the form {name}, or {name...} for the rest of the path.")
			}
			if rest && i != len(segs)-1 {
				return bad("Path segment "+seg+" takes the rest of the path but is not the last",
					"Only the last segment may be {name...}; it matches every segment from there on.")
			}
		}
	}
	return nil
}

func typeName(t reflect.Type) string {
	if n := t.Name(); n != "" {
		return n
	}
	return t.String()
}

func (p Page[A]) lowerPage(l *appLowerer) *ir.Page {
	typ := reflect.TypeFor[A]()
	specs, _ := args.Spec(typ, string(p.Path))
	codec := args.NewCodec(typ, specs)

	page := &ir.Page{
		PathTemplate: string(p.Path),
		Args:         specs,
		Nav:          l.nav(p.Nav, string(p.Path)),
		Decode:       codec.Decode,
		Encode:       codec.Encode,
	}
	if p.Guard != nil {
		page.Guard = func(ctx context.Context, a any) error { return p.Guard(ctx, a.(A)) }
	}
	page.Body = p.Body.lowerBody(ir.Addr{})
	assignViewIDs(page.Body)
	return page
}
