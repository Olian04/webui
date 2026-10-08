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
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
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
}

// Toast is a message shown once, in the bottom-right region.
type Toast struct {
	Title string
	Desc  string
	Error bool
}

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
	hasCSS bool // theme tokens present, so theme.css is linked
	search bool // some page offers Search, so the page script has something to ask
	logo   bool
}

type navEntry struct {
	Label   string
	Icon    string
	Section string
	Path    string // path template, the identity used for Active
	Href    string
}

// New derives the navigation and asset links from app.
func New(app *ir.App, prefix string) *Renderer {
	r := &Renderer{
		app: app, prefix: prefix,
		hasCSS: len(app.Theme.Tokens) > 0,
		logo:   len(app.Brand.Logo) > 0,
	}
	for _, p := range app.Pages {
		r.search = r.search || p.Search != nil
	}
	for _, p := range app.Pages {
		if p.Nav.Hidden {
			continue
		}
		r.nav = append(r.nav, navEntry{
			Label: p.Nav.Label, Icon: p.Nav.Icon, Section: p.Nav.Section, Path: p.PathTemplate, Href: r.Href(p.PathTemplate),
		})
	}
	return r
}

// FirstHref is the address of the first navigable page, or "".
func (r *Renderer) FirstHref() string {
	if len(r.nav) == 0 {
		return ""
	}
	return r.nav[0].Href
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

// AssetHref is the address of a framework asset.
func (r *Renderer) AssetHref(name string) string { return r.prefix + AssetDir + "/" + name }

// Home is the first breadcrumb: the brand, linking to the root.
func (r *Renderer) Home() Crumb {
	name := r.app.Brand.Name
	if name == "" {
		name = "Home"
	}
	return Crumb{Label: name, Href: r.Href("/")}
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
	if r.hasCSS {
		s = append(s, r.AssetHref("theme.css"))
	}
	return s
}

// Write renders comp into a buffer first, so a template failure can still
// produce a clean 500 instead of a half-written page, then writes it with the
// given status.
func Write(ctx context.Context, w http.ResponseWriter, status int, comp templ.Component) error {
	var buf bytes.Buffer
	if err := comp.Render(ctx, &buf); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write(buf.Bytes())
	return err
}

// Fragment is Write for a response that is not a whole document.
func Fragment(ctx context.Context, w http.ResponseWriter, status int, comp templ.Component) error {
	return Write(ctx, w, status, comp)
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

// Forbidden is the content of a guard rejection. The wording matters: the check
// ran before anything was loaded, so nothing was read.
func Forbidden(reason string) templ.Component {
	desc := "The guard for this page ran before anything was loaded, so no data was read."
	if reason != "" {
		desc += " " + reason
	}
	return c.StatePage(c.StatePageProps{Icon: c.IconLock, Title: "Not permitted", Desc: desc})
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

// themeCSS renders the token overrides. Selector specificity beats the
// stylesheet's theme blocks, so an override applies in every mode.
func themeCSS(tokens map[string]string) []byte {
	names := make([]string, 0, len(tokens))
	for k := range tokens {
		names = append(names, k)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("html:root:root {\n")
	for _, k := range names {
		fmt.Fprintf(&b, "  --%s: %s;\n", k, tokens[k])
	}
	b.WriteString("}\n")
	return []byte(b.String())
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
