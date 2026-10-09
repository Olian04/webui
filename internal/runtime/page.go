package runtime

import (
	"context"
	"errors"
	"net/http"

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

// addPage registers a page's routes.
func (p *Program) addPage(page *ir.Page) {
	leaves := leavesOf(page.Body)
	h := http.HandlerFunc(p.pageHandler(page, leaves))
	patterns := []string{p.pagePattern(page.PathTemplate)}
	if page.PathTemplate == "/" && p.Prefix != "" {
		patterns = append(patterns, p.Prefix) // mounted at "/admin" exactly
	}
	posts := actionable(leaves)
	for _, pattern := range patterns {
		p.add(http.MethodGet, pattern, h)
		p.probe.Handle(pattern, h)
		p.gate.Handle(pattern, p.gateHandler(page))
		p.where.Handle(pattern, whereHandler(page))
		p.allow[pattern] = "GET, HEAD"
		if posts {
			p.add(http.MethodPost, pattern, p.postHandler(page, leaves))
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
		raw, err := parser.Parse(r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		decoded, err := page.Decode(raw)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if page.Guard != nil {
			ctx := With(r.Context(), &Request{program: p, Args: decoded, Raw: raw})
			if page.Guard(ctx, decoded) != nil {
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
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

// begin decodes the request and runs the page Guard. It writes the response
// and returns false when the request must not go on.
func (p *Program) begin(w http.ResponseWriter, r *http.Request, page *ir.Page, parser args.ArgParser) (*Request, bool) {
	raw, err := parser.Parse(r)
	if err != nil {
		p.state(w, r, http.StatusBadRequest, page, nil, render.BadRequest())
		return nil, false
	}
	decoded, err := page.Decode(raw)
	if err != nil {
		p.state(w, r, http.StatusBadRequest, page, raw, render.BadRequest())
		return nil, false
	}
	req := &Request{program: p, Args: decoded, Raw: raw, Address: r.URL.RequestURI(), From: p.local(raw[args.FromKey])}
	*r = *r.WithContext(With(r.Context(), req))

	if page.Guard != nil {
		if err := page.Guard(r.Context(), decoded); err != nil {
			if ctxErr(r.Context()) {
				return nil, false
			}
			p.state(w, r, http.StatusForbidden, page, raw, render.Forbidden(err.Error()))
			return nil, false
		}
	}
	return req, true
}

func ctxErr(ctx context.Context) bool {
	return errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded)
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

// viewNodes maps each leaf that keeps view state in the address to its ID: the
// tables that page or sort, and every Tabs. The parser reads the parameters it
// implies.
func viewNodes(n ir.Node) map[string]ir.Node {
	out := map[string]ir.Node{}
	var walk func(ir.Node)
	walk = func(n ir.Node) {
		switch n := n.(type) {
		case *ir.Stack:
			for _, c := range n.Children {
				walk(c)
			}
		case *ir.Split:
			for _, c := range n.Children {
				walk(c)
			}
		case *ir.Tabs:
			out[n.ID] = n
			for _, t := range n.Tabs {
				walk(t.Body)
			}
		case *ir.Table:
			out[n.ID] = n
		}
	}
	if n != nil {
		walk(n)
	}
	return out
}

func sortable(n *ir.Table) bool {
	for _, col := range n.Columns {
		if col.Key != "" {
			return true
		}
	}
	return false
}

// viewKeys lists the address parameters the body's leaves keep state in, and
// which of them may repeat: a multi-select filter is one parameter per chosen
// option.
func viewKeys(n ir.Node) (keys, lists []string) {
	for id, node := range viewNodes(n) {
		switch node := node.(type) {
		case *ir.Tabs:
			keys = append(keys, args.ViewKey(id, "tab"))
		case *ir.Table:
			if node.Feed {
				keys = append(keys, args.ViewKey(id, "after"), args.RowKey(id), args.BackKey(id))
				lists = append(lists, args.BackKey(id))
				continue
			}
			keys = append(keys, args.ViewKey(id, "offset"))
			if sortable(node) {
				keys = append(keys, args.ViewKey(id, "sort"), args.ViewKey(id, "desc"))
			}
			for _, col := range node.Columns {
				if col.Key == "" {
					continue
				}
				if numeric(col) {
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
	}
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

func (p *Program) children(ctx context.Context, req *Request, page *ir.Page, nodes []ir.Node) ([]templ.Component, error) {
	out := make([]templ.Component, len(nodes))
	for i, n := range nodes {
		c, err := p.body(ctx, req, page, n)
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

// pageDoc is the shell fields every response for a page shares.
func (p *Program) pageDoc(page *ir.Page, req *Request, content templ.Component) render.Doc {
	pathVals := map[string]string{}
	if req != nil {
		pathVals, _ = req.encode(page)
	}
	crumbs := p.render.Crumbs(page, pathVals)
	doc := render.Doc{
		Title:   render.Title(crumbs),
		Crumbs:  crumbs,
		Active:  p.render.Active(page),
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
	if ctxErr(r.Context()) {
		return
	}
	p.log.Error("webui: request failed", "page", page.PathTemplate, "err", err)
	p.writePage(w, r, http.StatusInternalServerError, page, req, render.ServerError())
}
