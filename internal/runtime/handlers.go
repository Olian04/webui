package runtime

import (
	"context"
	"net/http"

	"github.com/Olian04/webui/internal/render"
)

// root serves the prefix itself when no page claims "/": the first page in the
// navigation, or an empty shell when the app declares none.
func (p *Program) root(w http.ResponseWriter, r *http.Request) {
	if href := p.render.FirstHref(p.hiddenNav(r.Context())); href != "" {
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, href, http.StatusFound)
		return
	}
	p.write(w, r, http.StatusOK, render.Doc{Content: render.NoPages()})
}

// notFound is the 404 inside the shell, so the way back is on screen.
func (p *Program) notFound(w http.ResponseWriter, r *http.Request) {
	if _, pattern := p.probe.Handler(r); pattern != "" {
		w.Header().Set("Allow", p.allow[pattern])
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
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
	doc.Toasts = append(p.takeFlash(w, r), doc.Toasts...)
	w.Header().Set("Cache-Control", "no-store")
	if err := render.Write(r.Context(), w, status, p.render.Document(doc)); err != nil {
		p.log.Error("webui: render failed", "path", r.URL.Path, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// FailureHandler serves the compile problems at every address and for every
// method: the app did not compile, so there is nothing else to serve.
func FailureHandler(problems []render.Problem) http.Handler {
	return secure(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	for _, path := range p.render.GuardedPaths() {
		page := p.App.ByPath[path]
		if page == nil || page.Guard == nil {
			continue
		}
		args, err := page.Decode(map[string]string{})
		if err == nil {
			err = page.Guard(With(ctx, &Request{program: p, Args: args, Raw: map[string]string{}}), args)
		}
		if err == nil {
			continue
		}
		if ctxErr(ctx) {
			return nil // the visitor went away: nothing to draw for
		}
		if hide == nil {
			hide = map[string]bool{}
		}
		hide[path] = true
	}
	return hide
}
