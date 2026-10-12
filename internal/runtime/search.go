package runtime

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Olian04/webui/internal/ir"
)

const (
	searchPerPage  = 8               // hits kept from each page
	searchMaxChars = 200             // of the query: it is typed by a person
	searchTimeout  = 5 * time.Second // for every page together
)

type searchHit struct {
	Group string `json:"group"`
	Title string `json:"title"`
	Desc  string `json:"desc,omitempty"`
	Href  string `json:"href"`
}

// search answers the global search with what every page that offers itself to it finds.
// A page's Search is handed what was typed, and answers with the arguments of the page
// that each result leads to. A page that fails is left out, and the rest answer.
//
// Every result is then checked against the page it leads to, with that page's own
// arguments and Guard, so the search never offers what following it would refuse:
// a device the visitor may not see is not a hit, however the search that found it
// was written.
func (p *Program) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if runes := []rune(query); len(runes) > searchMaxChars {
		query = string(runes[:searchMaxChars])
	}

	hits := []searchHit{}
	if query != "" {
		ctx, cancel := context.WithTimeout(r.Context(), searchTimeout)
		defer cancel()
		seen := map[string]bool{} // a thing found by two pages is one result, not two
		for _, page := range p.App.Pages {
			hits = append(hits, p.searchPage(ctx, page, query, seen)...)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"results": hits})
}

// searchPage is the hits of one page's search. The page is searched with the arguments it has
// when nothing is selected, which a Search that is about all of the page's things does not
// read; the page's Guard is not asked, since with no arguments it would refuse for the wrong
// reason, and each hit is checked against it with its own.
func (p *Program) searchPage(ctx context.Context, page *ir.Page, query string, seen map[string]bool) []searchHit {
	if page.Search == nil {
		return nil
	}
	raw := map[string]string{}
	args, err := page.Decode(raw)
	if err != nil {
		return nil
	}
	ctx = With(ctx, &Request{program: p, Args: args, Raw: raw})

	found, err := p.found(ctx, page, query)
	if err != nil {
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
		if seen[href] || !p.mayOpen(ctx, href) {
			continue // a result is only for what this visitor could open: the destination's Guard has the last word
		}
		seen[href] = true
		hits = append(hits, searchHit{Group: group, Title: cmp.Or(hit.Title, href), Desc: hit.Desc, Href: href})
	}
	return hits
}

// found is what a page's search returns, with a panic in it answered as an error: a search is
// asked on every keystroke, and one that fails must not take the others with it.
func (p *Program) found(ctx context.Context, page *ir.Page, query string) (hits []ir.Hit, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in search: %v", r)
		}
	}()
	return page.Search(ctx, query, searchPerPage) //nolint:wrapcheck // the caller logs it with the page
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
