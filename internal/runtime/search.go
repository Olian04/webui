package runtime

import (
	"cmp"
	"context"
	"encoding/json"
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

// search answers the global search with the rows of every table that offers
// itself to it. A table is searched under its own name, after its page's Guard has
// run with zero arguments — what a visitor who opened the page bare would be
// allowed. A page that refuses, or a table that fails, is left out, and the rest
// answer.
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
		seen := map[string]bool{} // a table on two pages is one result, not two
		for _, page := range p.App.Pages {
			hits = append(hits, p.searchPage(ctx, page, query, seen)...)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"results": hits})
}

// searchPage is the hits of one page's searchable tables.
func (p *Program) searchPage(ctx context.Context, page *ir.Page, query string, seen map[string]bool) []searchHit {
	var tables []*ir.Table
	for _, t := range ir.Tables(page.Body) {
		if t.Search {
			tables = append(tables, t)
		}
	}
	if len(tables) == 0 {
		return nil
	}
	args, err := page.Decode(map[string]string{})
	if err != nil {
		return nil
	}
	req := &Request{program: p, Args: args, Raw: map[string]string{}}
	ctx = With(ctx, req)
	if page.Guard != nil && page.Guard(ctx, args) != nil {
		return nil
	}

	var hits []searchHit
	for _, t := range tables {
		group := cmp.Or(t.Title, page.Nav.Label, page.PathTemplate)
		rows, _, err := t.Load(ctx, ir.Query{Limit: searchPerPage, Search: query})
		if err != nil && ctx.Err() == nil {
			p.log.Error("webui: search failed", "page", page.PathTemplate, "table", t.Title, "err", err)
		}
		for i, row := range rows {
			if i == searchPerPage {
				break
			}
			href, err := p.open(t.RowClick.Dest, t.RowClick.Args(ctx, row), "")
			if err != nil {
				p.log.Error("webui: search result dropped: its link cannot be built", "page", page.PathTemplate, "err", err)
				continue
			}
			if seen[href] || !p.mayOpen(ctx, href) {
				continue // a result is only for what this visitor could open: the destination's Guard has the last word
			}
			seen[href] = true
			title, desc := rowText(t, row)
			hits = append(hits, searchHit{Group: group, Title: title, Desc: desc, Href: href})
		}
	}
	return hits
}

// rowText is a row as a search result: its first column is the title, and the
// others, where they say something, are the line beneath it.
func rowText(t *ir.Table, row any) (title, desc string) {
	var rest []string
	for i, c := range t.Columns {
		if c.Get == nil {
			continue
		}
		switch text := c.Get(row); {
		case i == 0:
			title = text
		case text != "":
			rest = append(rest, text)
		}
	}
	return title, strings.Join(rest, " · ")
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
