package render

import (
	"maps"
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
// value it took. A {name...} placeholder, which takes the rest of the path, is a
// crumb for each of its segments, each linking to this page at that prefix, so a
// nested folder reads as the folders on the way to it.
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
		if name, rest, ok := args.Placeholder(seg); ok && rest {
			crumbs = append(crumbs, restCrumbs(r, page, name, path)...)
			continue
		} else if ok {
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

// restCrumbs are the crumbs for a {name...} placeholder: one for each segment of the
// value, all but the last a link to this page with the value cut off there.
func restCrumbs(r *Renderer, page *ir.Page, name string, path map[string]string) []Crumb {
	parts := strings.Split(path[name], "/")
	crumbs := make([]Crumb, len(parts))
	for i, part := range parts {
		crumbs[i] = Crumb{Label: part}
		if i < len(parts)-1 {
			upto := maps.Clone(path)
			upto[name] = strings.Join(parts[:i+1], "/")
			crumbs[i].Href = r.prefix + args.Href(page.PathTemplate, upto, nil)
		}
	}
	return crumbs
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
