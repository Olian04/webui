package render

import (
	"strings"

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
