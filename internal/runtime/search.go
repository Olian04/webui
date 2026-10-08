package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
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

// search answers the global search with every page's own results. Each page
// that offers Search is asked in turn, under its own name, after its Guard has
// run with zero arguments — what a visitor who opened the page bare would be
// allowed. A page that fails, or refuses, is left out, and the rest answer.
//
// Every result is then checked against the page it leads to, with that page's own
// arguments and Guard, so the search never offers what following it would refuse:
// a device the visitor may not see is not a hit, however the page that found it
// was allowed.
func (p *Program) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if runes := []rune(query); len(runes) > searchMaxChars {
		query = string(runes[:searchMaxChars])
	}

	hits := []searchHit{}
	if query != "" {
		ctx, cancel := context.WithTimeout(r.Context(), searchTimeout)
		defer cancel()
		for _, page := range p.App.Pages {
			if page.Search == nil {
				continue
			}
			args, err := page.Decode(map[string]string{})
			if err != nil {
				continue
			}
			ctx := With(ctx, &Request{program: p, Args: args, Raw: map[string]string{}})
			if page.Guard != nil && page.Guard(ctx, args) != nil {
				continue
			}
			results, err := page.Search(ctx, query)
			if err != nil && ctx.Err() == nil {
				p.log.Error("webui: search failed", "page", page.PathTemplate, "err", err)
			}
			group := page.Nav.Label
			if group == "" {
				group = page.PathTemplate
			}
			for i, res := range results {
				if i == searchPerPage {
					break
				}
				if !safeRedirect(res.Href) {
					p.log.Error("webui: search result refused: not an address on this host", "page", page.PathTemplate, "href", res.Href)
					continue
				}
				if !p.mayOpen(ctx, res.Href) {
					continue // a result is only for what this visitor could open: the destination's Guard has the last word
				}
				hits = append(hits, searchHit{Group: group, Title: res.Title, Desc: res.Desc, Href: res.Href})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"results": hits})
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
