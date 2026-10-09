package runtime

import (
	"context"
	"net/http"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// LeafHeader names the leaf a request wants refreshed. Framework parameters
// travel as headers, never in the URL: the URL carries user state, so nothing
// framework-owned appears in an address anyone might copy.
const LeafHeader = "X-Webui-Leaf"

// leaf answers a refresh of one panel. It is the same URL as the page, so the
// page Guard has already run, the arguments are the page's, and the markup is
// what the full page would hold for that leaf — one implementation, two
// callers. An id that names no leaf is a 404, not a fall back to the page.
func (p *Program) leaf(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n ir.Node) {
	if n == nil {
		http.NotFound(w, r)
		return
	}
	body, err := p.body(r.Context(), req, page, n)
	if err != nil {
		p.fail(w, r, page, req, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if err := render.Fragment(r.Context(), w, http.StatusOK, body); err != nil {
		p.log.Error("webui: render failed", "path", r.URL.Path, "leaf", r.Header.Get(LeafHeader), "err", err)
	}
}

// tabs builds the strip and loads only the selected panel. An empty or unknown
// value selects the first tab, so a stale link still lands somewhere.
func (p *Program) tabs(ctx context.Context, req *Request, page *ir.Page, n *ir.Tabs) (templ.Component, error) {
	selected := 0
	for i, t := range n.Tabs {
		if t.Key == req.Raw[args.ViewKey(n.ID, "tab")] {
			selected = i
		}
	}
	path, query := req.encode(page)

	strip := make([]render.TabLink, len(n.Tabs))
	for i, t := range n.Tabs {
		q := make(map[string]string, len(query)+1)
		for k, v := range query {
			q[k] = v
		}
		if i == 0 {
			delete(q, args.ViewKey(n.ID, "tab")) // the default tab needs no parameter
		} else {
			q[args.ViewKey(n.ID, "tab")] = t.Key
		}
		strip[i] = render.TabLink{Label: t.Label, Href: p.render.PageHref(page, path, q), Active: i == selected}
	}

	content, err := p.body(ctx, req, page, n.Tabs[selected].Body)
	if err != nil {
		return nil, err
	}
	return render.Tabs("Views", strip, content), nil
}
