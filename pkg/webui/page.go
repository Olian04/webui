package webui

import (
	"context"
	"reflect"
	"regexp"
	"strings"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/convert"
	"github.com/Olian04/webui/internal/ir"
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

	// Guard decides whether the visitor may open the page. It runs before anything
	// is loaded, on every request to the page, with the decoded arguments. A
	// non-nil error answers 403, and its text is shown as the reason, unless it is
	// a [Refusal], which answers with its own status (404, 401 or 403) and says what it
	// says, with a link on if it has one. The pages a Guard refuses are also left out
	// of the sidebar and the search.
	//
	// A Guard is asked for an address once in a request, whoever asks: the request for the page,
	// the sidebar's entry for it, a breadcrumb, Cancel and Back, and every link that leads there
	// (a table of a hundred rows that lead to three pages runs the Guard of that page three
	// times, not a hundred) share one answer, so a page agrees with itself even when the Guard's
	// answer changes in the middle of a request. What the address has besides the page's own
	// arguments (a table's sort, where the visitor came from) is not part of it. The Guards of the
	// pages in the sidebar are asked at the same time, up to eight at once, so a Guard is
	// called from several goroutines and must be safe for that, as the application's other
	// functions are. An answer that came of a visitor who left, or of a deadline that ran
	// out, is not kept.
	Guard func(ctx context.Context, a A) error

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
	// are shown under the words of the page's entry in [App.Navigation], or of the entry it borrows its place from.
	//
	// Match however the source can, ignoring case, against whatever the visitor sees of a
	// result. [SearchRows] makes one from rows held in memory. If it returns an error the
	// page is left out of the search, the cause is logged, and the rest are searched.
	// A page with no Search is not searched, though its entry in [App.Navigation] is still listed.
	Search func(ctx context.Context, query string, limit int) ([]Hit[A], error)

	// View is what the page shows: a [Table], a [Form], or a layout of them.
	View View
}

// View is what a page shows, and what a layout holds: a panel, which loads its own data ([Table], [Form], [Editor], [Diff], [Markdown], [List], [Cards],
// [Calendar], [Toolbar] or [FileUpload]), or a layout, which arranges other Views ([Stack], [Split], [Columns], [When] or [Tabs]). Only this package implements it.
type View interface {
	validateView(v *viewValidator)
	lowerView(at ir.Pos) ir.Node
}

// NavEntry is one entry that a [NavPages] lists: an address of the page, and what it says.
type NavEntry[A any] struct {
	// Args are the page's arguments that the entry opens, with the path ones filled in.
	Args A

	// Label is what the entry says. An entry with none is left out.
	Label string
}

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
	v := &viewValidator{page: path, args: typeName(typ), argType: typ, facts: f}

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

	if p.Crumb != nil {
		segs := strings.Split(strings.Trim(path, "/"), "/")
		if _, rest, ok := args.Placeholder(segs[len(segs)-1]); ok && rest {
			v.add("the page has a Crumb and its path ends in a {name...}",
				"The crumbs of a path that takes the rest are its segments, one for each. Remove Crumb, or end the path in a placeholder that takes one segment.")
		}
	}
	if p.View == nil {
		v.add("View is nil", "Set View to "+viewKinds+".")
		return v.errs
	}
	p.View.validateView(v)
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

func (p Page[A]) lowerPage() *ir.Page {
	typ := reflect.TypeFor[A]()
	specs, _ := args.Spec(typ, string(p.Path))
	codec := args.NewCodec(typ, specs)

	page := &ir.Page{
		PathTemplate: string(p.Path),
		Args:         specs,
		Decode:       codec.Decode,
		Encode:       codec.Encode,
	}
	if p.Guard != nil {
		page.Guard = func(ctx context.Context, a any) error { return refused(p.Guard(ctx, a.(A))) }
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
	page.Root = p.View.lowerView(ir.Pos{})
	assignViewIDs(page.Root)
	return page
}
