package render

import (
	"sort"
	"strings"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
)

// PageHref is the address of page with the given argument values, prefix
// included. path and query are the encoded values, keyed by argument name.
func (r *Renderer) PageHref(page *ir.Page, path, query map[string]string) string {
	return r.prefix + args.Href(page.PathTemplate, path, query)
}

// Active is the path template of the nav entry that is lit while page is open:
// the page's own, or the one it shadows. Empty when neither exists.
func (r *Renderer) Active(page *ir.Page) string {
	if page.Nav.Shadow != "" {
		return page.Nav.Shadow
	}
	if !page.Nav.Hidden {
		return page.PathTemplate
	}
	return ""
}

// Crumbs is the breadcrumb trail, derived from the path template's segments.
// A parent is a link when a page is mounted at that prefix of the path and
// every argument it needs is already in this path; a placeholder shows the
// value it took.
func (r *Renderer) Crumbs(page *ir.Page, path map[string]string) []Crumb {
	crumbs := []Crumb{r.Home()}
	if page.PathTemplate == "/" {
		if page.Nav.Label != "" {
			return append(crumbs, Crumb{Label: page.Nav.Label})
		}
		crumbs[0].Href = ""
		return crumbs
	}

	segs := strings.Split(strings.Trim(page.PathTemplate, "/"), "/")
	for i, seg := range segs {
		prefix := "/" + strings.Join(segs[:i+1], "/")
		crumb := Crumb{Label: humanise(seg)}
		if name, ok := placeholder(seg); ok {
			crumb.Label = path[name]
		} else if parent := r.app.ByPath[prefix]; parent != nil && parent.Nav.Label != "" {
			crumb.Label = parent.Nav.Label
		}
		if parent := r.app.ByPath[prefix]; parent != nil && i < len(segs)-1 && hasAll(prefix, path) {
			crumb.Href = r.prefix + args.Href(prefix, path, nil)
		}
		crumbs = append(crumbs, crumb)
	}
	return crumbs
}

func placeholder(seg string) (string, bool) {
	if len(seg) > 2 && seg[0] == '{' && seg[len(seg)-1] == '}' {
		return seg[1 : len(seg)-1], true
	}
	return "", false
}

func hasAll(template string, path map[string]string) bool {
	for _, name := range args.Placeholders(template) {
		if path[name] == "" {
			return false
		}
	}
	return true
}

func humanise(seg string) string {
	seg = strings.NewReplacer("-", " ", "_", " ").Replace(seg)
	if seg == "" {
		return seg
	}
	return strings.ToUpper(seg[:1]) + seg[1:]
}

// Title is the last breadcrumb's label: the name of the thing on screen.
func Title(crumbs []Crumb) string {
	if len(crumbs) == 0 {
		return ""
	}
	return crumbs[len(crumbs)-1].Label
}

type pill struct {
	Label     string
	Name      string
	Kind      ir.ValueKind
	Value     string
	ClearHref string
	Hidden    []hidden
}

type hidden struct{ Name, Value string }

type toolbarView struct {
	Action   string
	Pills    []pill
	ClearAll string
	Refresh  string
}

// isOffset reports whether an address parameter is a table's offset. Setting
// any argument returns every table to its first page: the old offset may no
// longer exist.
func isOffset(key string) bool { return strings.HasSuffix(key, args.ViewSep+"offset") }

// Toolbar renders a page's arguments as controls. Path arguments are identity
// and live in the breadcrumb. What is left is the page's view settings, one
// pill each. query is everything in the address after the path: the page's
// arguments and the view state the library keeps for tables and tabs, which
// is carried along but never shown as a pill.
func (r *Renderer) Toolbar(page *ir.Page, path, query map[string]string) templ.Component {
	tv := toolbarView{
		Action:  r.PageHref(page, path, nil),
		Refresh: r.PageHref(page, path, query),
	}
	set := false
	for _, a := range page.Args {
		if a.InPath {
			continue
		}
		p := pill{Label: a.Field, Name: a.Name, Kind: a.Kind, Value: query[a.Name]}
		set = set || p.Value != ""

		// Setting one argument keeps everything else in the address.
		rest := map[string]string{}
		for k, v := range query {
			if k != a.Name && !isOffset(k) {
				rest[k] = v
			}
		}
		for _, k := range sortedKeys(rest) {
			p.Hidden = append(p.Hidden, hidden{k, rest[k]})
		}
		if p.Value != "" {
			p.ClearHref = r.PageHref(page, path, rest)
		}
		tv.Pills = append(tv.Pills, p)
	}
	if set {
		// Clear all resets the filters, and the paging that depended on them.
		// Sort and tab are how the page is viewed, not filters, so they stay.
		keep := map[string]string{}
		for k, v := range query {
			if args.IsViewKey(k) && !isOffset(k) {
				keep[k] = v
			}
		}
		tv.ClearAll = r.PageHref(page, path, keep)
	}
	return toolbar(tv)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func numberStep(k ir.ValueKind) string {
	if k == ir.KindFloat {
		return "any"
	}
	return "1"
}
