package runtime

import (
	"net/http"

	"github.com/Olian04/webui/internal/render"
)

// root serves the prefix itself when no page claims "/": the first page in the
// navigation, or an empty shell when the app declares none.
func (p *Program) root(w http.ResponseWriter, r *http.Request) {
	if href := p.render.FirstHref(); href != "" {
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
