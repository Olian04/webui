// Package render turns the IR and request-time data into HTML.
//
// It owns how things look and nothing else: the runtime decides what data to
// show and hands it over as plain values, and the component library in
// templates/components owns the markup. Every colour, radius and elevation is
// a theme token in the stylesheet; nothing here writes a literal value.
//
// render never imports the runtime, and nothing here knows an HTTP request.
package render

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/assetserve"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render/assets"
	c "github.com/Olian04/webui/internal/render/templates/components"
)

// Crumb is one breadcrumb. The last has no link.
type Crumb = c.Crumb

// Doc is one full document: the shell plus the page's content.
type Doc struct {
	Title   string
	Crumbs  []Crumb
	Active  string // path template of the nav entry to light, or ""
	Content templ.Component
	Method  string
	URL     string // as the browser asked for it, shown in the status bar
	Toasts  []Toast

	// Home is where the brand leads, or "" when there is nowhere to go and it is plain text.
	// The first breadcrumb leads there too, unless it is the page on screen.
	Home string

	// HideNav is the paths of the navigation entries this visitor may not open, so
	// the sidebar and the search that reads it leave them out.
	HideNav map[string]bool

	// Built is when the HTML was built, and BuildTime how long that took from the request's
	// arrival, which the status bar shows. A zero Built says nothing.
	Built     time.Time
	BuildTime time.Duration

	// Streaming is a page that is sent as it is built. Its status bar comes in two parts, the request with the shell
	// and how the page came to be when all of it has been drawn, and a mark says that the content is still arriving.
	Streaming bool

	// Timing, when it is set, is asked for the time of the status bar when the bar is drawn, in place of Built and
	// BuildTime: a page that is sent as it is built draws the end of the bar last, and says how long all of it took.
	Timing func() (built time.Time, took time.Duration)

	// Here is the page the response is for, and the arguments of its address, so an entry
	// of a page that lists several can tell which of them is the one on screen.
	Here Here

	// NavLinks are the entries of the pages that list several in the navigation, by page
	// path template: loaded for this response, for this visitor, and already checked
	// against the page's Guard.
	NavLinks map[string][]NavLink

	// Code is what the page's editors and diffs need loaded.
	Code CodeNeeds
}

// Here is a page and the arguments of the address it was asked at, by name: the path ones,
// and the query ones with the table and tab state that rides along in the address.
type Here struct {
	Page  string // the page's path template
	Path  map[string]string
	Query map[string]string
}

// NavLink is one entry of a page that lists several in the navigation: where it goes, what it says,
// and whether it is the one that is on screen.
type NavLink struct {
	Label  string
	Href   string
	Active bool
}

// Toast is a message shown once, in the bottom-right region.
type Toast struct {
	Title string
	Desc  string
	Tone  ToastTone
}

// ToastTone is what a toast says about the outcome: confirmed, to be aware of, or
// failed. A toast with none reads as confirmed.
type ToastTone = c.Tone

// The tones.
const (
	ToastOK      = c.ToneOK
	ToastWarning = c.ToneWarning
	ToastError   = c.ToneCritical
)

// TabLink is one entry in a tab strip.
type TabLink = c.Tab

// Problem is one compile failure, as the failure page shows it.
type Problem = c.Problem

// Renderer is built once per compiled app. It holds everything derived from
// the IR that does not change per request, so a request does no discovery.
type Renderer struct {
	app    *ir.App
	prefix string
	nav    []navEntry

	// files is each asset by the name it is served as, with the hash of its contents in it, and
	// served is that name by the one it is known by here.
	files  map[string]*assetserve.Blob
	served map[string]string
	search bool // some table is searchable, so the page script has something to ask

	// contexts is every file an editor's Context holds, by the name it is served as, and secret keys
	// the addresses that unlock them.
	contexts map[string]*contextEntry
	secret   []byte

	// monaco is the editor's entry script, stylesheet and plain worker, which a page
	// links only when it has an editor or a diff.
	monaco assets.Monaco
}

type navEntry struct {
	Label   string
	Icon    string
	Section string
	Path    string // path template, the identity used for Active
	Href    string

	// Dynamic is the place of a page that lists several entries, which are not known until a
	// response is drawn: Label and Href are empty, and the entries are the response's NavLinks.
	// Lit is whether one of those is the entry on screen.
	Dynamic bool
	Lit     bool
}

// New derives the navigation and asset links from app.
//
// secret keys the addresses of the files an editor is given: only a page drawn for a visitor who may edit
// carries a key.
func New(app *ir.App, prefix string, secret []byte) (*Renderer, error) {
	r := &Renderer{app: app, prefix: prefix, secret: secret}
	if err := r.buildAssets(); err != nil {
		return nil, err
	}
	r.monaco, _ = assets.MonacoFiles() // embedded by the build: it is there
	r.buildContexts()
	for _, p := range app.Pages {
		r.search = r.search || p.Search != nil
	}
	for _, p := range app.Pages {
		if p.Nav.Hidden {
			continue
		}
		e := navEntry{Label: p.Nav.Label, Icon: p.Nav.Icon, Section: p.Nav.Section, Path: p.PathTemplate, Href: r.Href(p.PathTemplate)}
		if p.Nav.Dynamic {
			e.Label, e.Href, e.Dynamic = "", "", true
		}
		r.nav = append(r.nav, e)
	}
	return r, nil
}

// hasTheme is whether the app restyles its colours, so a stylesheet of its own is linked, and hasLogo
// whether it has a logo, which is then an asset.
func (r *Renderer) hasTheme() bool { return r.app.Theme != (ir.Theme{}) }
func (r *Renderer) hasLogo() bool  { return len(r.app.Brand.Logo) > 0 }

// FirstEntry is the address of the first navigation entry the visitor may
// open, or "". hide is the paths of the entries they may not.
func (r *Renderer) FirstEntry(hide map[string]bool) string {
	if shown := r.shown(hide, nil); len(shown) > 0 {
		return shown[0].Href // the first the sidebar lists, which sections reorder
	}
	return ""
}

// NavPaths is the path template of every navigation entry, in order, so the runtime
// can ask which of them the visitor may open.
func (r *Renderer) NavPaths() []string {
	out := make([]string, 0, len(r.nav))
	for _, e := range r.nav {
		if !e.Dynamic { // those are decided for each entry, with its own arguments, when they are loaded
			out = append(out, e.Path)
		}
	}
	return out
}

// Href prefixes an app-relative path with the mount prefix. The root page is
// the prefix itself with a trailing slash, so a link to it works whether or not
// the prefix is empty.
func (r *Renderer) Href(path string) string {
	if path == "/" {
		return r.prefix + "/"
	}
	return r.prefix + path
}

// AssetHref is the address of a framework asset, by the name it is known by here
// ("app.css"). The address carries a short hash of the file's contents, so it
// changes when the file does and a browser can keep it for good. A name that is no
// asset, such as the search, is its own address.
func (r *Renderer) AssetHref(name string) string {
	if served, ok := r.served[name]; ok {
		name = served
	}
	return r.prefix + AssetDir + "/" + name
}

// Home is the first breadcrumb: the brand. Its address is the page the brand leads to, which
// the response fills in.
func (r *Renderer) Home() Crumb {
	return Crumb{Label: r.brand(), Href: r.Href("/")}
}

func (r *Renderer) title(page string) string {
	brand := r.app.Brand.Name
	if brand == "" {
		brand = "webui"
	}
	if page == "" {
		return brand
	}
	return page + " — " + brand
}

func (r *Renderer) styles() []string {
	s := []string{r.AssetHref("fontawesome.css"), r.AssetHref("app.css")}
	if r.hasTheme() {
		s = append(s, r.AssetHref("theme.css"))
	}
	return s
}

// NotFound is the content of a 404.
func NotFound() templ.Component {
	return c.StatePage(c.StatePageProps{
		Icon: c.IconSearch, Title: "Page not found",
		Desc: "Nothing in this app is served at that address.",
	})
}

// NoPages is the content of an app that declares no pages.
func NoPages() templ.Component {
	return c.EmptyState(c.EmptyStateProps{
		Icon: c.IconGear, Title: "No pages declared",
		Desc: "Add a page to App.Pages and it appears here.",
	})
}

// NoStart is the content of the root of an app that has pages, but none at "/" and
// none in the navigation that the visitor may open, so the brand has nowhere to lead.
func NoStart() templ.Component {
	return c.EmptyState(c.EmptyStateProps{
		Icon: c.IconInfo, Title: "Nothing to open from here",
		Desc: "This app has no page at \"/\" and no navigation entry you may open. Declare a page at \"/\", or give a page a Nav entry.",
	})
}

// Forbidden is the content of a guard rejection: what the guard said, which is for the visitor to read, or, when it
// said nothing, that they may not open the page. How the check was made is not for them.
func Forbidden(reason string) templ.Component {
	if reason == "" {
		reason = "You do not have access to this page."
	}
	return c.StatePage(c.StatePageProps{Icon: c.IconLock, Title: "Not permitted", Desc: reason})
}

// BadRequest is the content of a 400: an argument that did not parse.
func BadRequest() templ.Component {
	return c.StatePage(c.StatePageProps{
		Icon: c.IconAlert, Title: "That address is not valid",
		Desc: "One of the arguments in it could not be read. Remove the arguments from the address and try again.",
	})
}

// ServerError is the content of a 500. The cause is logged, never shown.
func ServerError() templ.Component {
	return c.StatePage(c.StatePageProps{
		Icon: c.IconAlert, Title: "Something went wrong",
		Desc: "The page could not be built. The cause was logged on the server.",
	})
}

// themeCSS renders the app's colours as overrides of the design's tokens, for
// each of light and dark. A hover colour, a readable text tint and a badge's
// faint background and border are derived from the one colour, per mode, with
// color-mix, so the app names four colours and the design stays coherent.
//
// The selectors are as specific as the stylesheet's theme blocks and come after
// them: a stored choice (data-theme) wins, then the viewer's system preference.
func themeCSS(t ir.Theme) []byte {
	var b strings.Builder
	rule := func(selector string, decls []string) {
		if len(decls) > 0 {
			fmt.Fprintf(&b, "%s {\n  %s\n}\n", selector, strings.Join(decls, "\n  "))
		}
	}
	dark, light := themeDecls(t, true), themeDecls(t, false)
	rule("html:root:root:not([data-theme]),\nhtml:root:root[data-theme='dark']", dark)
	rule("html:root:root[data-theme='light']", light)
	if len(light) > 0 {
		b.WriteString("@media (prefers-color-scheme: light) {\n")
		fmt.Fprintf(&b, "  html:root:root:not([data-theme]) {\n    %s\n  }\n", strings.Join(light, "\n    "))
		b.WriteString("}\n")
	}
	return []byte(b.String())
}

// themeDecls are the token declarations for one mode. In dark a status colour
// is lightened to read on a dark surface; in light it is deepened to read on a
// light one.
func themeDecls(t ir.Theme, dark bool) []string {
	var out []string
	mix := func(c string, pct int, with string) string {
		return fmt.Sprintf("color-mix(in srgb, %s %d%%, %s)", c, pct, with)
	}
	if c := t.Accent; c != "" {
		// The hover is deeper in both modes: a lighter fill under a white label
		// loses the contrast it needs.
		if dark {
			out = append(out, "--blue: "+c+";", "--blue-hover: "+mix(c, 88, "black")+";", "--blue-text: "+mix(c, 62, "white")+";")
		} else {
			out = append(out, "--blue: "+c+";", "--blue-hover: "+mix(c, 86, "black")+";", "--blue-text: "+mix(c, 80, "black")+";")
		}
	}
	for _, s := range []struct{ name, c string }{{"green", t.OK}, {"orange", t.Warning}, {"red", t.Critical}} {
		if s.c == "" {
			continue
		}
		text := mix(s.c, 85, "black")
		if dark {
			text = mix(s.c, 60, "white")
		}
		out = append(out,
			"--"+s.name+": "+text+";",
			"--"+s.name+"-bg: "+mix(s.c, 14, "transparent")+";",
			"--"+s.name+"-bd: "+mix(s.c, 30, "transparent")+";")
		if s.name == "red" {
			// The colour as given is the fill of a destructive button, which carries a
			// white label; the text tint above is for text on the page.
			out = append(out, "--red-solid: "+s.c+";", "--red-solid-hover: "+mix(s.c, 88, "black")+";")
		}
	}
	return out
}

// brand is the app's name as the sidebar shows it, "Home" when it has none, so
// the link to the landing page always has a name.
func (r *Renderer) brand() string {
	if r.app.Brand.Name == "" {
		return "Home"
	}
	return r.app.Brand.Name
}

// initial is the first letter of a label, capitalised: what an entry with no
// icon shows when the sidebar is a rail.
func initial(label string) string {
	for _, c := range label {
		return string(unicode.ToUpper(c))
	}
	return ""
}

// shown is the navigation entries the visitor may open, in the order the sidebar
// lists them, which is grouped by Section and not by position: the entries with no
// Section come first, then each section in the order it is first named, each with
// its entries in the order given and its caption on the first of them. A section
// with nothing left in it has no caption either.
func (r *Renderer) shown(hide map[string]bool, links map[string][]NavLink) []navEntry {
	var open []navEntry
	for _, e := range r.nav {
		switch {
		case hide[e.Path]:
		case e.Dynamic:
			// The place of a page that lists several: an entry for each, in the order it gave them.
			for _, l := range links[e.Path] {
				open = append(open, navEntry{Label: l.Label, Icon: e.Icon, Section: e.Section, Path: e.Path, Href: l.Href, Dynamic: true, Lit: l.Active})
			}
		default:
			open = append(open, e)
		}
	}
	var out []navEntry
	for _, g := range bySection(open, func(e navEntry) string { return e.Section }) {
		for i, e := range g.items {
			e.Section = ""
			if i == 0 {
				e.Section = g.section
			}
			out = append(out, e)
		}
	}
	return out
}

// lit is whether an entry is the one on screen: a page that lists several says which of them, and any
// other is lit when it is the page's own entry, or the one the page borrows its place from.
func (e navEntry) lit(active string) bool {
	if e.Dynamic {
		return e.Lit
	}
	return e.Path == active
}

// group is the entries of one section, in the order they were given.
type group[T any] struct {
	section string
	items   []T
}

// bySection groups entries by the section each names: the ones that name none
// first, then each section in the order it is first named. An entry belongs to a
// section only by naming it, wherever it stands in the list.
func bySection[T any](items []T, section func(T) string) []group[T] {
	index := map[string]int{}
	groups := []group[T]{{}}
	index[""] = 0
	for _, it := range items {
		s := section(it)
		i, ok := index[s]
		if !ok {
			i = len(groups)
			index[s] = i
			groups = append(groups, group[T]{section: s})
		}
		groups[i].items = append(groups[i].items, it)
	}
	return groups
}

// earlyCSS is the canvas colour of each theme, inline, ahead of the stylesheet. The
// values are the stylesheet's --canvas tokens; a test keeps the two equal.
const earlyCSS = `html{background:#111217;color-scheme:dark}` +
	`html[data-theme='light']{background:#f4f5f5;color-scheme:light}` +
	`@media (prefers-color-scheme:light){html:not([data-theme]){background:#f4f5f5;color-scheme:light}}`

// linkEntries are the external links as the component draws them, grouped by Section
// like the sidebar: the entries with no Section first, then each section in the order it
// is first named, each with its caption above the first of its entries. A link to a web
// address opens in a tab of its own; a path on this site and a mail address do not.
func (r *Renderer) linkEntries() []c.LinkEntry {
	out := make([]c.LinkEntry, 0, len(r.app.ExternalLinks))
	for _, g := range bySection(r.app.ExternalLinks, func(e ir.ExternalLink) string { return e.Section }) {
		for i, e := range g.items {
			entry := c.LinkEntry{Label: e.Label, Href: e.URL, Icon: c.IconName(e.Icon), NewTab: isWebAddress(e.URL)}
			if i == 0 {
				entry.Section = g.section
			}
			out = append(out, entry)
		}
	}
	return out
}

// isWebAddress is whether an address is on the web, as opposed to a path on this site or a
// mail address: the links that leave the app, and so open in a tab of their own.
func isWebAddress(address string) bool {
	a := strings.ToLower(address)
	return strings.HasPrefix(a, "http://") || strings.HasPrefix(a, "https://")
}

// statusBar is the status bar of a page, with the time it was built at the moment it is drawn when the page says so.
// bar makes the bar from its props: the whole of it, or the end of it that a page which is sent as it is built draws last.
func (r *Renderer) statusBar(d Doc, bar func(c.StatusBarProps) templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		built, took := d.Built, d.BuildTime
		if d.Timing != nil {
			built, took = d.Timing()
		}
		return bar(c.StatusBarProps{Method: d.Method, URL: d.URL, Built: built, BuildTime: took}).Render(ctx, w) //nolint:wrapcheck // a component's own error
	})
}

// PanelFailed is a panel that could not be built, drawn in place of it once the page has begun to be sent and the
// answer can no longer be an error page: the panel says so, and the page stands.
func PanelFailed(title string) templ.Component {
	return c.Panel(c.PanelProps{Title: title, Failed: true})
}

// refusalTitle is what a refusal is called, by its status.
func refusalTitle(status int) string {
	switch status {
	case 404:
		return "Not found"
	case 401:
		return "Not signed in"
	}
	return "Not permitted"
}

// refusedOf is a panel's refusal as the panel says it, or nil.
func refusedOf(ref *ir.Refusal) *c.Refused {
	if ref == nil {
		return nil
	}
	return &c.Refused{Title: refusalTitle(ref.Status), Desc: ref.Message, Label: ref.Label, Href: ref.Href}
}

// refusalIcon is the icon of a refusal: a search for what is not found, and a lock for what is not allowed.
func refusalIcon(status int) c.IconName {
	if status == 404 {
		return c.IconSearch
	}
	return c.IconLock
}
