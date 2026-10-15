package runtime

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"runtime/debug"
	"slices"
	"sync"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// pagePattern is the mux pattern for a page template under the prefix.
func (p *Program) pagePattern(template string) string {
	if template == "/" {
		return p.Prefix + "/{$}"
	}
	return p.Prefix + template
}

// pictureOrigins are the origins the page's Markdown leaves trust to serve pictures, once
// each, in the order they are first named. They come from the declaration, so they are the
// same for every request and every tab, selected or not.
func pictureOrigins(body ir.Node) []string {
	var out []string
	for _, m := range ir.Markdowns(body) {
		for _, o := range m.TrustedOrigins {
			if !slices.Contains(out, o) {
				out = append(out, o)
			}
		}
	}
	return out
}

// addPage registers a page's routes.
func (p *Program) addPage(page *ir.Page) {
	leaves := leavesOf(page.Body)
	h := http.HandlerFunc(p.pageHandler(page, leaves))
	patterns := []string{p.pagePattern(page.PathTemplate)}
	if page.PathTemplate == "/" && p.Prefix != "" {
		patterns = append(patterns, p.Prefix) // mounted at "/admin" exactly
	}
	posts := actionable(leaves)
	policy := policy(pictureOrigins(page.Body))
	for _, pattern := range patterns {
		p.handle(p.mux, http.MethodGet, pattern, page.PathTemplate, policy, h)
		p.gate.Handle(pattern, p.gateHandler(page))
		p.allow[pattern] = "GET, HEAD"
		if posts {
			p.handle(p.mux, http.MethodPost, pattern, page.PathTemplate, policy, p.postHandler(page, leaves))
			p.allow[pattern] = "GET, HEAD, POST"
		}
	}
}

// gateHandler answers whether the visitor may open a page at the address it is
// asked about: 204 when they may, 400 when the address does not decode, 403 when
// the page's Guard refuses. It is the front of pageHandler, run alone, so the
// answer is the one a real request would get.
func (p *Program) gateHandler(page *ir.Page) http.HandlerFunc {
	parser := argParser(page)
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, status, _ := p.admit(r, page, parser)
		w.WriteHeader(cmp.Or(status, http.StatusNoContent))
	}
}

// pageHandler is the request pipeline of one page. The order is the contract:
// decode the arguments, run the page Guard, and only then load anything, so an
// unauthorised page is never read.
func (p *Program) pageHandler(page *ir.Page, leaves map[string]ir.Node) func(http.ResponseWriter, *http.Request) {
	parser := argParser(page)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", LeafHeader)
		req, ok := p.begin(w, r, page, parser)
		if !ok {
			return
		}
		ctx := r.Context()
		if table, format, bad := exportOf(r, leaves); bad {
			p.state(w, r, http.StatusBadRequest, page, req.Raw, render.BadRequest())
			return
		} else if table != nil {
			p.export(w, r, page, req, table, format)
			return
		}
		if id := r.Header.Get(LeafHeader); id != "" {
			p.leaf(w, r, page, req, leaves[id])
			return
		}
		body, err := p.body(ctx, req, page, page.Body)
		if err != nil {
			p.fail(w, r, page, req, err)
			return
		}
		p.writePage(w, r, http.StatusOK, page, req, body)
	}
}

// admit reads a request's arguments and runs the page's Guard, which is the front of every request to a
// page and all of asking whether a visitor may open one. It installs the request in r's context, so the
// Guard and everything after it can reach it. When the request must not go on it says why: status is 400
// for an address that does not decode, and 403 for a Guard that refuses, with its error.
func (p *Program) admit(r *http.Request, page *ir.Page, parser args.ArgParser) (req *Request, raw map[string]string, status int, refusal error) {
	raw, err := parser.Parse(r)
	if err != nil {
		return nil, nil, http.StatusBadRequest, nil
	}
	decoded, err := page.Decode(raw)
	if err != nil {
		return nil, raw, http.StatusBadRequest, nil
	}
	req = &Request{program: p, Args: decoded, Raw: raw, Address: r.URL.RequestURI(), From: p.local(raw[args.FromKey])}
	*r = *r.WithContext(With(r.Context(), req))
	if page.Guard != nil {
		if err := page.Guard(r.Context(), decoded); err != nil {
			return req, raw, http.StatusForbidden, err
		}
	}
	return req, raw, 0, nil
}

// begin decodes the request and runs the page Guard. It writes the response
// and returns false when the request must not go on.
func (p *Program) begin(w http.ResponseWriter, r *http.Request, page *ir.Page, parser args.ArgParser) (*Request, bool) {
	req, raw, status, refusal := p.admit(r, page, parser)
	switch {
	case status == http.StatusBadRequest:
		p.state(w, r, status, page, raw, render.BadRequest())
	case status == http.StatusForbidden && r.Context().Err() == nil: // a visitor who has gone is answered nothing
		p.state(w, r, status, page, raw, render.Forbidden(refusal.Error()))
	}
	return req, status == 0
}

// argParser reads the page's arguments, and the view state its tables and tabs
// keep in the address. Any other parameter is ignored, so an address cannot
// smuggle state into a leaf the page does not have.
func argParser(page *ir.Page) args.ArgParser {
	var parser args.ArgParser
	for _, a := range page.Args {
		if a.InPath {
			parser.PathKeys = append(parser.PathKeys, a.Name)
		} else {
			parser.QueryKeys = append(parser.QueryKeys, a.Name)
		}
	}
	view, lists := viewKeys(page.Body)
	parser.QueryKeys = append(parser.QueryKeys, args.FromKey)
	parser.QueryKeys = append(parser.QueryKeys, view...)
	parser.ListKeys = lists
	return parser
}

// viewKeys lists the address parameters the body's leaves keep state in, and
// which of them may repeat: a multi-select filter is one parameter per chosen
// option. They are the tables that page or sort, and every Tabs.
func viewKeys(n ir.Node) (keys, lists []string) {
	ir.Walk(n, func(node ir.Node) {
		switch node := node.(type) {
		case *ir.Tabs:
			keys = append(keys, args.ViewKey(node.ID, "tab"))
		case *ir.Table:
			id := node.ID
			if node.Feed {
				keys = append(keys, args.ViewKey(id, "after"), args.RowKey(id), args.BackKey(id))
				lists = append(lists, args.BackKey(id))
			} else {
				keys = append(keys, args.ViewKey(id, "offset"))
			}
			if slices.ContainsFunc(node.Columns, func(col ir.Field) bool { return col.Key != "" }) {
				keys = append(keys, args.ViewKey(id, "sort"), args.ViewKey(id, "desc"))
			}
			if node.Feed {
				return // one sequence: nothing to filter
			}
			for _, col := range node.Columns {
				if col.Key == "" {
					continue
				}
				if col.Ranged() {
					lo, hi := args.RangeKeys(id, col.Key)
					keys = append(keys, lo, hi)
					continue
				}
				key := args.FilterKey(id, col.Key)
				keys = append(keys, key)
				if col.Options != nil || col.OpenSet {
					lists = append(lists, key)
				}
			}
		}
	})
	return keys, lists
}

// body loads and builds the page's content. Layouts only arrange; leaves are
// built where they load.
func (p *Program) body(ctx context.Context, req *Request, page *ir.Page, n ir.Node) (templ.Component, error) {
	switch n := n.(type) {
	case *ir.Stack:
		kids, err := p.children(ctx, req, page, n.Children)
		return render.Stack(kids), err
	case *ir.Split:
		kids, err := p.children(ctx, req, page, n.Children)
		return render.Split(kids), err
	case *ir.Tabs:
		return p.tabs(ctx, req, page, n)
	case *ir.Table:
		return p.table(ctx, req, page, n), nil
	case *ir.Diff:
		return p.diff(ctx, req, page, n), nil
	case *ir.Markdown:
		return p.markdown(ctx, req, page, n), nil
	case *ir.Form:
		var sub *submission
		if req.sub != nil && req.subLeaf == render.LeafID(n.At) {
			sub = req.sub
		}
		return p.form(ctx, req, page, n, sub), nil
	default:
		return nil, errors.New("runtime: node kind not served")
	}
}

// children builds the panels of a layout all at once, and not one after the other: a page is then as slow
// as its slowest panel, and not as the sum of them. Each panel is built with a request of its own for what
// it records (the editors it has), and what they recorded is put together when they are all done, so the
// page has what it would have had in any order. A panic in one is caught where it happened, so that it
// does not take the process down, and raised again here, where the request's own recovery answers it.
func (p *Program) children(ctx context.Context, req *Request, page *ir.Page, nodes []ir.Node) ([]templ.Component, error) {
	if len(nodes) == 1 {
		c, err := p.body(ctx, req, page, nodes[0])
		return []templ.Component{c}, err
	}
	type built struct {
		comp   templ.Component
		err    error
		code   render.CodeNeeds
		caught *workerPanic
	}
	results := make([]built, len(nodes))
	var wg sync.WaitGroup
	for i := range nodes {
		wg.Go(func() {
			defer func() {
				if v := recover(); v != nil {
					results[i].caught = &workerPanic{value: v, stack: debug.Stack()}
				}
			}()
			own := *req
			own.code = render.CodeNeeds{}
			results[i].comp, results[i].err = p.body(ctx, &own, page, nodes[i])
			results[i].code = own.code
		})
	}
	wg.Wait()

	out := make([]templ.Component, len(nodes))
	for i := range results {
		if results[i].caught != nil {
			results[i].caught.raise()
		}
	}
	for i := range results {
		if results[i].err != nil {
			return nil, results[i].err
		}
		out[i] = results[i].comp
		req.code.Merge(results[i].code)
	}
	return out, nil
}

// pageDoc is the shell fields every response for a page shares.
func (p *Program) pageDoc(page *ir.Page, req *Request, content templ.Component) render.Doc {
	pathVals, query := map[string]string{}, map[string]string{}
	if req != nil {
		pathVals, query = req.encode(page)
	}
	crumbs := p.render.Crumbs(page, pathVals)
	doc := render.Doc{
		Title:   render.Title(crumbs),
		Crumbs:  crumbs,
		Active:  p.render.Active(page),
		Here:    render.Here{Page: page.PathTemplate, Path: pathVals, Query: query},
		Content: content,
	}
	if req != nil {
		doc.Code = req.code
	}
	return doc
}

func (p *Program) writePage(w http.ResponseWriter, r *http.Request, status int, page *ir.Page, req *Request, content templ.Component, toasts ...render.Toast) {
	doc := p.pageDoc(page, req, content)
	doc.Toasts = toasts
	p.write(w, r, status, doc)
}

// state answers with a full-page state inside the shell. raw may be nil when
// the arguments never decoded, in which case the breadcrumb falls back to the
// template's own segments.
func (p *Program) state(w http.ResponseWriter, r *http.Request, status int, page *ir.Page, raw map[string]string, content templ.Component) {
	var req *Request
	if raw != nil {
		if decoded, err := page.Decode(raw); err == nil {
			req = &Request{program: p, Args: decoded, Raw: raw}
		}
	}
	p.write(w, r, status, p.pageDoc(page, req, content))
}

// fail is a load or render failure: the cause is logged and never shown.
func (p *Program) fail(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, err error) {
	if r.Context().Err() != nil {
		return
	}
	p.log.Error("webui: request failed", "page", page.PathTemplate, "err", err)
	p.writePage(w, r, http.StatusInternalServerError, page, req, render.ServerError())
}
