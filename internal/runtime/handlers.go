package runtime

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// root serves the prefix itself when no page claims "/". The visitor who types the address
// is sent to the page the brand leads to, see [Program.start]. With none, a page says so, or
// that the app declares no pages at all.
func (p *Program) root(w http.ResponseWriter, r *http.Request) {
	if href := p.start(r.Context(), p.hiddenNav(r.Context())); href != "" {
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, href, http.StatusFound)
		return
	}
	if len(p.App.Pages) > 0 {
		p.write(w, r, http.StatusOK, render.Doc{Content: render.NoStart()})
		return
	}
	p.write(w, r, http.StatusOK, render.Doc{Content: render.NoPages()})
}

// start is the address the brand leads to, and where the root goes: the page at "/" if the
// visitor may open it, else the first navigation entry they may open, else the first page
// declared whose path has no arguments and whose guard lets them in. hide is the entries they
// may not open. It is "" when there is nowhere to go, and the brand is then not a link.
func (p *Program) start(ctx context.Context, hide map[string]bool) string {
	if page := p.App.ByPath["/"]; page != nil {
		if href := p.render.Href("/"); page.Guard == nil || p.mayOpen(ctx, href) {
			return href
		}
	}
	if href, _ := p.render.FirstEntry(hide); href != "" {
		return href
	}
	for _, page := range p.App.Pages {
		if len(args.Placeholders(page.PathTemplate)) > 0 {
			continue // it has no address until its arguments are filled in
		}
		if href := p.Prefix + args.Href(page.PathTemplate, nil, nil); p.mayOpen(ctx, href) {
			return href
		}
	}
	return ""
}

// notFound is the 404 inside the shell, so the way back is on screen.
func (p *Program) notFound(w http.ResponseWriter, r *http.Request) {
	if _, pattern := p.gate.Handler(r); pattern != "" {
		w.Header().Set("Allow", p.allow[pattern])
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	p.missing(w, r)
}

// missing is the 404 in the shell, for an address that is nothing of the app's.
func (p *Program) missing(w http.ResponseWriter, r *http.Request) {
	p.write(w, r, http.StatusNotFound, render.Doc{
		Title:   "Not found",
		Crumbs:  []render.Crumb{p.render.Home(), {Label: "Not found"}},
		Content: render.NotFound(),
	})
}

// write renders doc as a full page. Fields every page shares are filled here.
func (p *Program) write(w http.ResponseWriter, r *http.Request, status int, doc render.Doc) {
	if doc.Crumbs == nil {
		home := p.render.Home()
		home.Href = ""
		doc.Crumbs = []render.Crumb{home}
	}
	doc.Method, doc.URL = r.Method, r.URL.RequestURI()
	if doc.HideNav == nil {
		doc.HideNav = p.hiddenNav(r.Context())
	}
	doc.Home = p.start(r.Context(), doc.HideNav)
	if len(doc.Crumbs) > 0 && doc.Crumbs[0].Href != "" {
		// The brand crumb leads where the sidebar's brand does, and is plain text when that is nowhere.
		doc.Crumbs = slices.Clone(doc.Crumbs)
		doc.Crumbs[0].Href = doc.Home
	}
	if doc.NavLinks == nil {
		doc.NavLinks = p.navLinks(r.Context(), doc.Here, doc.Active)
	}
	doc.Toasts = append(p.takeFlash(w, r), doc.Toasts...)
	w.Header().Set("Cache-Control", "no-store")
	doc.Built, doc.BuildTime = timing(w, r)
	if err := render.Write(r.Context(), w, status, p.render.Document(doc)); err != nil {
		p.log.Error("webui: render failed", "path", r.URL.Path, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// FailureHandler serves the compile problems at every address and for every
// method: the app did not compile, so there is nothing else to serve.
func FailureHandler(problems []render.Problem) http.Handler {
	return securedFor(csp, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if err := render.Write(r.Context(), w, http.StatusInternalServerError, render.CompileFailure(problems)); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}))
}

// hiddenNav is the navigation entries this visitor may not open: the pages with a
// Guard that refuses them, asked with no arguments, as a visitor who opened the
// page bare would be. A page without a Guard costs nothing. An entry that cannot
// be decided with no arguments is hidden too, since it could not be opened bare
// either. The sidebar leaves these out, and the search, which reads the sidebar,
// with it; the page itself still answers 403 to anyone who types the address.
func (p *Program) hiddenNav(ctx context.Context) map[string]bool {
	var hide map[string]bool
	for _, path := range p.render.NavPaths() {
		page := p.App.ByPath[path]
		if page == nil || page.Guard == nil {
			continue
		}
		args, err := page.Decode(map[string]string{})
		if err == nil {
			err = page.Guard(p.reqCtx(ctx, args, map[string]string{}), args)
		}
		if err == nil {
			continue
		}
		if ctx.Err() != nil {
			return nil // the visitor went away: nothing to draw for
		}
		if hide == nil {
			hide = map[string]bool{}
		}
		hide[path] = true
	}
	return hide
}

const (
	// maxNavEntries bounds the entries of one page that lists several, so a source of thousands
	// does not make a sidebar of them, nor the page that has them slow to draw.
	maxNavEntries = 50

	// navEntriesTimeout bounds the loading of a page's entries. It is on the way of every full page.
	navEntriesTimeout = 2 * time.Second
)

// navLinks loads the entries of every page that lists several in the navigation, for the visitor
// this response is for. They are asked for each time a full page is drawn, a page at a time and
// each within a deadline; one that fails, panics or is too slow is left out, and the page is
// drawn as if it had none, since a sidebar must not be what takes a page down. An entry the page's
// Guard would refuse, with the entry's own arguments, is left out, and the first maxNavEntries
// are kept.
func (p *Program) navLinks(ctx context.Context, here render.Here, active string) map[string][]render.NavLink {
	var out map[string][]render.NavLink
	for _, page := range p.App.Pages {
		if page.NavEntries == nil {
			continue
		}
		links := p.pageNavLinks(ctx, page, here, active)
		if len(links) == 0 {
			continue
		}
		if out == nil {
			out = map[string][]render.NavLink{}
		}
		out[page.PathTemplate] = links
	}
	return out
}

func (p *Program) pageNavLinks(ctx context.Context, page *ir.Page, here render.Here, active string) []render.NavLink {
	ctx, cancel := context.WithTimeout(ctx, navEntriesTimeout)
	defer cancel()
	raw := map[string]string{}
	zero, err := page.Decode(raw)
	if err != nil {
		return nil
	}
	entries, err := guarded("NavEntries", func() ([]ir.NavEntry, error) { return page.NavEntries(p.reqCtx(ctx, zero, raw)) })
	if err != nil {
		if ctx.Err() == nil || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			p.m.NavFailed(page.PathTemplate)
			p.log.Error("webui: a page's navigation entries failed, so it has none", "page", page.PathTemplate, "err", err)
		}
		return nil
	}
	if len(entries) > maxNavEntries {
		p.log.Warn("webui: a page has more navigation entries than are shown", "page", page.PathTemplate, "entries", len(entries), "shown", maxNavEntries)
		entries = entries[:maxNavEntries]
	}

	var links []render.NavLink
	best := -1
	for _, e := range entries {
		if e.Label == "" {
			continue
		}
		if page.Guard != nil && page.Guard(p.reqCtx(ctx, e.Args, raw), e.Args) != nil {
			continue // the visitor could not open it
		}
		path, query, err := page.Encode(e.Args)
		if err != nil {
			p.log.Error("webui: a navigation entry is dropped: its address cannot be built", "page", page.PathTemplate, "label", e.Label, "err", err)
			continue
		}
		link := render.NavLink{Label: e.Label, Href: p.Prefix + args.Href(page.PathTemplate, path, query)}
		if score := entryScore(page, here, active, path, query); score >= 0 {
			// The entries that fit the address best are the one on screen, so a general entry is not
			// lit beside a more particular one that is.
			if score > best {
				best = score
				for i := range links {
					links[i].Active = false
				}
			}
			link.Active = score == best
		}
		links = append(links, link)
	}
	return links
}

// entryScore is how well an entry of page fits the address on screen, or -1 when it does not: its
// page is the one lit (the page itself, or the one that borrows its place), and every argument it
// has, in the path and in the query, is the one in the address. An argument the address does not
// have is not the entry's, so on a page that borrows the place an entry with query arguments that
// the page does not carry is not lit, and the general one is. Table and tab state in the address is
// not an argument and does not count against it. More arguments that agree is a better fit.
func entryScore(page *ir.Page, here render.Here, active string, path, query map[string]string) int {
	if active != page.PathTemplate {
		return -1
	}
	score := 0
	for name, value := range path {
		if here.Path[name] != value {
			return -1
		}
		score++
	}
	for name, value := range query {
		if here.Query[name] != value {
			return -1
		}
		score++
	}
	return score
}
