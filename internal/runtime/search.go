package runtime

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/rank"
	"github.com/Olian04/webui/internal/render"
)

const (
	searchPerPage  = 8               // hits kept from each page
	searchMaxChars = 200             // of the query: it is typed by a person
	searchTimeout  = 2 * time.Second // for each page: they are asked at once, so it is also the most that the answer waits
)

type searchHit struct {
	Group string
	Title string
	Desc  string
	Href  string

	// Score is how well the result matches what was typed, from 0 to 1 (see [rank.Hit]), which is what orders them.
	Score float64
}

// search answers the global search with what every page that offers itself to it finds, as HTML: the box's list for script, which asks
// with fragment=1 and puts it beneath the box as it is typed, and for a request without it, a page of results in the shell of the app,
// which is what the box's button goes to when there is no script to list them.
//
// Every page is asked at once, each on a goroutine of its own, so the answer takes as long as the slowest and not as long as all of them,
// and a page that is not answered within searchTimeout is left out, and the rest answer. The answer is sent whole and in order: results
// that arrived one by one would move under the pointer as they came, and the visitor would click the wrong one.
//
// A page's Search is handed what was typed and answers with the arguments of the page that each result leads to. A page that fails is
// left out. Every result is then checked against the page it leads to, with that page's own arguments and Guard, so the search never
// offers what following it would refuse: a device the visitor may not see is not a hit, however the search that found it was written.
func (p *Program) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if runes := []rune(query); len(runes) > searchMaxChars {
		query = string(runes[:searchMaxChars])
	}

	var groups []render.SearchGroup
	if query != "" {
		defer p.m.Search(time.Now())
		groups = searchGroups(ranked(p.searchAll(r.Context(), query)))
	}

	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Get("fragment") != "" {
		if err := writeHTML(r.Context(), w, http.StatusOK, render.SearchFragment(groups)); err != nil {
			p.log.Error("webui: render failed", "path", r.URL.Path, "err", err)
			plainFailure(w, r)
		}
		return
	}
	home := p.render.Home()
	p.write(w, r, http.StatusOK, render.Doc{
		Title: "Search", Query: query,
		Crumbs:  []render.Crumb{home, {Label: "Search"}},
		Content: render.SearchResults(query, groups),
	})
}

// searchAll asks every page that has a Search, at once, and is every result they found, in the order of the pages, without those of a page that
// did not answer in time and without a thing found by two pages twice.
func (p *Program) searchAll(parent context.Context, query string) []searchHit {
	ctx, cancel := context.WithTimeout(parent, searchTimeout)
	defer cancel() // what a page that is let go of does after this is not logged, and its results are dropped

	var slots []chan []searchHit
	for _, page := range p.App.Pages {
		if page.Search == nil {
			continue
		}
		slot := make(chan []searchHit, 1) // room for the answer of a page that is no longer waited for
		slots = append(slots, slot)
		go func() {
			defer func() {
				if v := recover(); v != nil { // a panic here would end the process: the page is left out
					p.log.Error("webui: a search panicked, so its page is left out", "page", page.PathTemplate, "panic", v)
					slot <- nil
				}
			}()
			slot <- p.searchPage(ctx, page, query)
		}()
	}

	var all []searchHit
	for _, slot := range slots {
		select {
		case hits := <-slot:
			all = append(all, hits...)
		default:
			select {
			case hits := <-slot:
				all = append(all, hits...)
			case <-ctx.Done():
				select { // one that was ready as time ran out is still an answer
				case hits := <-slot:
					all = append(all, hits...)
				default:
				}
			}
		}
	}

	seen := map[string]bool{}
	out := all[:0]
	for _, h := range all {
		if !seen[h.Href] {
			seen[h.Href] = true
			out = append(out, h)
		}
	}
	return out
}

// searchGroups are the ranked results as the page shows them: under the name of the page that found them, a group's results together.
func searchGroups(hits []searchHit) []render.SearchGroup {
	var out []render.SearchGroup
	at := map[string]int{}
	for _, h := range hits {
		i, ok := at[h.Group]
		if !ok {
			i = len(out)
			at[h.Group] = i
			out = append(out, render.SearchGroup{Label: h.Group})
		}
		out[i].Hits = append(out[i].Hits, render.SearchHit{Title: h.Title, Desc: h.Desc, Href: h.Href})
	}
	return out
}

// searchPage is the hits of one page's search. The page is searched with the arguments it has
// when nothing is selected, which a Search that is about all of the page's things does not
// read; the page's Guard is not asked, since with no arguments it would refuse for the wrong
// reason, and each hit is checked against it with its own.
func (p *Program) searchPage(ctx context.Context, page *ir.Page, query string) []searchHit {
	if page.Search == nil {
		return nil
	}
	raw := map[string]string{}
	args, err := page.Decode(raw)
	if err != nil {
		return nil
	}
	ctx = p.reqCtx(ctx, args, raw)

	found, err := guarded("search", func() ([]ir.Hit, error) { return page.Search(ctx, query, searchPerPage) })
	if err != nil {
		if ctx.Err() == nil || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			p.m.SearchFailed(page.PathTemplate)
		}
		if ctx.Err() == nil {
			p.log.Error("webui: search failed", "page", page.PathTemplate, "err", err)
		}
		return nil
	}

	group := p.searchGroup(page)
	var hits []searchHit
	for i, hit := range found {
		if i == searchPerPage {
			break
		}
		href, err := p.open(page.PathTemplate, hit.Args, "")
		if err != nil {
			p.log.Error("webui: search result dropped: its link cannot be built", "page", page.PathTemplate, "err", err)
			continue
		}
		if !p.mayOpen(ctx, href) {
			continue // a result is only for what this visitor could open: the destination's Guard has the last word
		}
		title := cmp.Or(hit.Title, href)
		hits = append(hits, searchHit{Group: group, Title: title, Desc: hit.Desc, Href: href, Score: rank.Hit(query, title, hit.Desc)})
	}
	return hits
}

// ranked orders the results by how well they match: the best first inside each group, and the groups by their best result,
// so what the visitor most likely meant is at the top, under the name of what it is. Results that match as well are
// left in the order they were found in, and so are groups.
func ranked(hits []searchHit) []searchHit {
	var order []string // the groups, as they were first found
	best := map[string]float64{}
	by := map[string][]searchHit{}
	for _, h := range hits {
		if _, ok := by[h.Group]; !ok {
			order = append(order, h.Group)
		}
		by[h.Group] = append(by[h.Group], h)
		best[h.Group] = max(best[h.Group], h.Score)
	}
	slices.SortStableFunc(order, func(a, b string) int { return cmp.Compare(best[b], best[a]) })
	out := make([]searchHit, 0, len(hits))
	for _, g := range order {
		group := by[g]
		slices.SortStableFunc(group, func(a, b searchHit) int { return cmp.Compare(b.Score, a.Score) })
		out = append(out, group...)
	}
	return out
}

// searchGroup is what a page's results are listed under: the page's name in the navigation, or the
// name of the entry it borrows its place from, so the results of a device's page are "Devices".
func (p *Program) searchGroup(page *ir.Page) string {
	if page.Nav.Label != "" {
		return page.Nav.Label
	}
	if shadowed := p.App.ByPath[page.Nav.Shadow]; shadowed != nil && shadowed.Nav.Label != "" {
		return shadowed.Nav.Label
	}
	// A page that lists entries of its own has no label and borrows no place: it is the entries.
	// Its results are listed under the nearest page above it that has a name, such as "Sites" for
	// "/site/{name}", which is what the visitor knows the things as.
	segs := strings.Split(strings.Trim(page.PathTemplate, "/"), "/")
	for i := len(segs) - 1; i >= 1; i-- {
		if above := p.App.ByPath["/"+strings.Join(segs[:i], "/")]; above != nil && above.Nav.Label != "" {
			return above.Nav.Label
		}
	}
	return page.PathTemplate
}

// mayOpen reports whether the visitor could open the page at href: it is a page
// of this app, its arguments decode, and its Guard allows them. An address that
// is none of these is not a result worth showing.
func (p *Program) mayOpen(ctx context.Context, href string) bool {
	//nolint:gosec // G704: an in-process request to this app's own gate, and no connection: href is an address of this app, which a from address has been checked to be (see safeRedirect).
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, href, http.NoBody)
	if err != nil {
		return false
	}
	var verdict verdictWriter
	p.gate.ServeHTTP(&verdict, r)
	return verdict.status == http.StatusNoContent
}

// verdictWriter keeps only the status a gate handler answers with.
type verdictWriter struct {
	status int
	header http.Header
}

func (w *verdictWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *verdictWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
}

func (w *verdictWriter) Write(b []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return len(b), nil
}
