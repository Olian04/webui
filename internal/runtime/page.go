package runtime

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

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

// pictureOrigins are the origins the page's Markdown leaves and lists of items trust to serve pictures, once
// each, in the order they are first named. They come from the declaration, so they are the
// same for every request and every tab, selected or not.
func pictureOrigins(body ir.Node) []string {
	var out []string
	add := func(origins []string) {
		for _, o := range origins {
			if !slices.Contains(out, o) {
				out = append(out, o)
			}
		}
	}
	for _, m := range ir.Markdowns(body) {
		add(m.TrustedOrigins)
	}
	for _, i := range ir.Itemses(body) {
		add(i.TrustedOrigins)
	}
	return out
}

// addPage registers a page's routes.
func (p *Program) addPage(page *ir.Page) {
	leaves := leavesOf(page.Body)
	h := http.HandlerFunc(p.pageHandler(page, leaves, tabsOf(page.Body)))
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
func (p *Program) pageHandler(page *ir.Page, leaves map[string]ir.Node, tabs map[string]*ir.Tabs) func(http.ResponseWriter, *http.Request) {
	parser := argParser(page)
	mayStream := streamable(page.Body)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", LeafHeader)
		if p.render.NeedsPhone(page) {
			w.Header().Add("Vary", "Accept-Language") // the country a number starts at is the visitor's
		}
		req, ok := p.begin(w, r, page, parser)
		if !ok {
			return
		}
		ctx := r.Context()
		if table, format, bad := exportOf(r, leaves); bad {
			p.state(w, r, http.StatusBadRequest, page, req.Raw, render.BadRequest(bareAddress(r)))
			return
		} else if table != nil {
			p.export(w, r, page, req, table, format)
			return
		}
		if id := r.Header.Get(LeafHeader); id != "" {
			// A panel, or a Tabs: a request for one of them loads that and nothing else of the page.
			if t, ok := tabs[id]; ok {
				p.leaf(w, r, page, req, t)
				return
			}
			p.leaf(w, r, page, req, leaves[id])
			return
		}
		path, query := p.address(page, req)
		req.shell = p.startShell(ctx, render.Here{Page: page.PathTemplate, Path: path, Query: query}, p.render.Active(page))
		req.panels = &panelSet{}
		p.writeBuilt(w, r, page, req, p.panel(ctx, req, page, page.Body), mayStream)
	}
}

// admit reads a request's arguments and runs the page's Guard, which is the front of every request to a
// page and all of asking whether a visitor may open one. It installs the request in r's context, so the
// Guard and everything after it can reach it. When the request must not go on it says why: status is 400
// for an address that does not decode, and for a Guard that refuses 403, or the status of the [ir.Refusal] it
// returns, with its error.
func (p *Program) admit(r *http.Request, page *ir.Page, parser args.ArgParser) (req *Request, raw map[string]string, status int, refusal error) {
	raw, err := parser.Parse(r)
	if err != nil {
		return nil, nil, http.StatusBadRequest, nil
	}
	decoded, err := page.Decode(raw)
	if err != nil {
		return nil, raw, http.StatusBadRequest, nil
	}
	req = &Request{program: p, Args: decoded, Raw: raw, Address: r.URL.RequestURI(), From: p.local(raw[args.FromKey]), Languages: r.Header.Get("Accept-Language")}
	*r = *r.WithContext(With(r.Context(), req))
	if page.Guard != nil {
		if err := page.Guard(r.Context(), decoded); err != nil {
			if ref, ok := errors.AsType[*ir.Refusal](err); ok {
				return req, raw, ref.Status, err // its own: not found, not signed in, or not permitted
			}
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
	case status != 0 && r.Method == http.MethodPost:
		// A POST is answered with a redirect, to the page, which answers the same request with the 400 or the 403. A POST that script made, for a
		// panel, is answered with the refusal itself, as a fragment, since a redirect would give it the whole of the page.
		if r.Context().Err() == nil {
			if wantsFragment(r) {
				text, next := "That request was not understood.", ""
				var ref *ir.Refusal
				switch {
				case errors.As(refusal, &ref):
					text, next = ref.Message, ref.Href // what the Guard said, and where it leads on
				case status == http.StatusForbidden && refusal != nil:
					text = refusal.Error()
				}
				p.writeResult(w, r, status, render.ToastError, text, next)
			} else {
				p.again(w, r)
			}
		}
	case status == http.StatusBadRequest:
		p.state(w, r, status, page, raw, render.BadRequest(bareAddress(r)))
	case status != 0 && r.Context().Err() == nil: // a visitor who has gone is answered nothing
		if ref, ok := errors.AsType[*ir.Refusal](refusal); ok {
			p.state(w, r, status, page, raw, render.Refused(ref))
			return req, false
		}
		p.state(w, r, status, page, raw, render.Forbidden(refusal.Error()))
	}
	return req, status == 0
}

// argParser reads the page's arguments, and the view state its tables and tabs
// keep in the address. Any other parameter is ignored, so an address cannot
// smuggle state into a leaf the page does not have.
func argParser(page *ir.Page) args.ArgParser {
	var parser args.ArgParser
	var lists []string
	for _, a := range page.Args {
		if a.InPath {
			parser.PathKeys = append(parser.PathKeys, a.Name)
		} else {
			parser.QueryKeys = append(parser.QueryKeys, a.Name)
			if a.List {
				lists = append(lists, a.Name) // a parameter that repeats keeps every value it is given
			}
		}
	}
	view, viewLists := viewKeys(page.Body)
	lists = append(lists, viewLists...)
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
		case *ir.Items:
			keys = append(keys, args.ViewKey(node.ID, "limit"))
		case *ir.Calendar:
			keys = append(keys, args.ViewKey(node.ID, "month"))
		case *ir.Table:
			id := node.ID
			if node.Feed {
				keys = append(keys, args.ViewKey(id, "after"), args.RowKey(id), args.BackKey(id))
				lists = append(lists, args.BackKey(id))
			} else {
				keys = append(keys, args.ViewKey(id, "offset"))
			}
			if slices.ContainsFunc(node.Columns, func(col ir.Field) bool { return col.Key != "" && !col.NoSort }) {
				keys = append(keys, args.ViewKey(id, "sort"), args.ViewKey(id, "desc"))
			}
			if node.Feed {
				return // one sequence: nothing to filter
			}
			for _, col := range node.Columns {
				if col.Key == "" || col.NoFilter {
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
	case *ir.Columns:
		kids, err := p.children(ctx, req, page, n.Children)
		return render.Columns(kids, n.Weights), err
	case *ir.Tabs:
		return p.tabs(ctx, req, page, n)
	case *ir.Table:
		return refreshing(ctx, func(ctx context.Context) templ.Component { return p.table(ctx, req, page, n) }), nil
	case *ir.Diff:
		return refreshing(ctx, func(ctx context.Context) templ.Component { return p.diff(ctx, req, page, n) }), nil
	case *ir.Markdown:
		return refreshing(ctx, func(ctx context.Context) templ.Component { return p.markdown(ctx, req, page, n) }), nil
	case *ir.Stat:
		return refreshing(ctx, func(ctx context.Context) templ.Component { return p.stat(ctx, req, page, n) }), nil
	case *ir.Items:
		return refreshing(ctx, func(ctx context.Context) templ.Component { return p.items(ctx, req, page, n) }), nil
	case *ir.Calendar:
		return refreshing(ctx, func(ctx context.Context) templ.Component { return p.calendar(ctx, req, page, n) }), nil
	case *ir.Toolbar:
		return refreshing(ctx, func(ctx context.Context) templ.Component { return p.toolbar(ctx, req, page, n) }), nil
	case *ir.FileUpload:
		return refreshing(ctx, func(ctx context.Context) templ.Component { return p.fileUpload(ctx, req, page, n) }), nil
	case *ir.Form:
		var sub *submission
		if req.sub != nil && req.subLeaf == render.LeafID(n.At) {
			sub = req.sub
		}
		return refreshing(ctx, func(ctx context.Context) templ.Component { return p.form(ctx, req, page, n, sub) }), nil
	default:
		return nil, errors.New("runtime: node kind not served")
	}
}

// children builds the panels of a layout all at once, and not one after the other: a page is then as slow
// as its slowest panel, and not as the sum of them. Each is a [panel], drawn by waiting for it. When the page
// keeps every panel it starts (req.panels), whoever draws the page waits for them; otherwise they are waited for here,
// before the layout is drawn.
func (p *Program) children(ctx context.Context, req *Request, page *ir.Page, nodes []ir.Node) ([]templ.Component, error) {
	out := make([]templ.Component, len(nodes))
	var mine panelSet
	for i, n := range nodes {
		pn := p.panel(ctx, req, page, n)
		out[i] = pn
		if req.panels == nil {
			mine.add(pn)
		}
	}
	if req.panels != nil {
		return out, nil
	}
	return out, mine.wait(&req.code)
}

// pageDoc is the shell fields every response for a page shares.
func (p *Program) pageDoc(ctx context.Context, page *ir.Page, req *Request, content templ.Component) render.Doc {
	pathVals, query := p.address(page, req)
	crumbs := p.render.Crumbs(page, pathVals, p.crumbLabels(ctx, page, req, pathVals))
	for i := 1; i < len(crumbs); i++ {
		// A parent that the visitor may not open is its words and no link, as a link to a page that refuses them is left out of a toolbar: it would
		// lead them to a 403 that nothing on the page had told them of.
		if crumbs[i].Href != "" && !p.mayLead(ctx, page, crumbs[i].Href) {
			crumbs = slices.Clone(crumbs)
			crumbs[i].Href = ""
		}
	}
	doc := render.Doc{
		Title:   render.Title(crumbs),
		Crumbs:  crumbs,
		Active:  p.render.Active(page),
		Here:    render.Here{Page: page.PathTemplate, Path: pathVals, Query: query},
		Content: content,
	}
	doc.Phone, doc.File, doc.Upload = p.render.NeedsPhone(page), p.render.NeedsFile(page), p.render.NeedsUpload(page)
	if req != nil {
		doc.Code = req.code
		if req.sub != nil {
			doc.Refused = len(req.sub.errs)
		}
	}
	return doc
}

func (p *Program) writePage(w http.ResponseWriter, r *http.Request, status int, page *ir.Page, req *Request, content templ.Component, toasts ...render.Toast) {
	doc := p.pageDoc(r.Context(), page, req, content)
	doc.Toasts = toasts
	var shell *shellInputs
	if req != nil {
		shell = req.shell
	}
	p.writeWith(w, r, status, doc, shell)
}

// address is the page's path arguments and query as the address has them, which are empty for a request that
// never decoded.
func (p *Program) address(page *ir.Page, req *Request) (path, query map[string]string) {
	if req == nil {
		return map[string]string{}, map[string]string{}
	}
	return req.encode(page)
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
	p.write(w, r, status, p.pageDoc(r.Context(), page, req, content))
}

// bareAddress is the request's address without its query, to lead on to when an argument in it could not be read: "" when there was no query to drop.
func bareAddress(r *http.Request) string {
	if r.URL.RawQuery == "" {
		return ""
	}
	return r.URL.Path
}

// fail is a load or render failure: the cause is logged and never shown.
func (p *Program) fail(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, err error) {
	if r.Context().Err() != nil {
		return
	}
	p.log.Error("webui: request failed", "page", page.PathTemplate, "err", err)
	if r.Method == http.MethodPost {
		if wantsFragment(r) {
			p.writeResult(w, r, http.StatusInternalServerError, render.ToastError, "Something went wrong. The cause was logged on the server.", "")
			return
		}
		p.refuse(w, r, "Something went wrong. The cause was logged on the server.") // a POST is answered with a redirect
		return
	}
	p.writePage(w, r, http.StatusInternalServerError, page, req, render.ServerError(r.URL.RequestURI()))
}

// crumbTimeout bounds the Crumb funcs of one page, all of them together: they are on the way of every full page.
const crumbTimeout = 2 * time.Second

// crumbLabels asks the pages that are crumbs of this one, and have a Crumb, what theirs say: the page itself
// with the arguments it was opened with, a parent with the arguments the path gives it. A Crumb that fails, panics,
// is too slow or says nothing leaves the segment's own words, since a breadcrumb must not be what takes a page
// down. A parent whose Guard would refuse the visitor says nothing either: what a Crumb knows is not for them.
// It is nil when no page of the app has a Crumb.
func (p *Program) crumbLabels(ctx context.Context, page *ir.Page, req *Request, path map[string]string) func(prefix string) string {
	if !p.hasCrumbs {
		return nil
	}
	deadline := time.Now().Add(crumbTimeout)
	return func(prefix string) string {
		target := p.App.ByPath[prefix]
		if target == nil || target.Crumb == nil {
			return ""
		}
		var (
			a   any
			raw = path
		)
		if target == page && req != nil {
			a, raw = req.Args, req.Raw
		} else {
			decoded, err := target.Decode(path)
			if err != nil {
				return ""
			}
			a = decoded
		}
		ctx, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()
		rc := p.reqCtx(ctx, a, raw)
		if target.Guard != nil && target.Guard(rc, a) != nil {
			return ""
		}
		label, err := guarded("Crumb", func() (string, error) { return target.Crumb(rc, a), nil })
		if err != nil {
			p.log.Error("webui: a page's Crumb failed, so its crumb says what its path does", "page", target.PathTemplate, "err", err)
		}
		return label
	}
}
