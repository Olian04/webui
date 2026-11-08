package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// The demo is the library's showcase, so these tests are the list of what it
// shows. They share the demo's one in-memory service, so none of them runs in
// parallel.

// handlerAs is the whole site, with every request carrying a session for the role, as
// a browser that had signed in would.
func handlerAs(t *testing.T, role Role) http.Handler {
	t.Helper()

	service = newService() // each test starts from the seed data, however many times it runs

	compiled, err := app.Compile("/admin")
	assert.NoError(t, err)
	site := routes(compiled)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//nolint:gosec // G124: a request the test makes, not a cookie the app sets.
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: sign(role)}) // beside what the request carries, such as the flash
		site.ServeHTTP(w, r)
	})
}

// handler is the site for an editor, who may do everything.
// searchResult is one result of the search, as the box lists it.
type searchResult struct{ Group, Title, Href string }

// searchResults are what the search finds for q, in order, read out of the HTML that it answers script with: a group for each page that found
// something, named for it, and an option under it for each result.
func searchResults(t *testing.T, h http.Handler, q string) []searchResult {
	t.Helper()
	box := page.Parse(t, get(searchBox(h), "/admin/_webui/search?q="+url.QueryEscape(q)).Body.String())
	var out []searchResult
	for _, group := range box.ByRole("group", "") {
		for _, option := range group.ByRole("option", "") {
			out = append(out, searchResult{Group: group.Name(), Href: option.Attr("href"), Title: option.Children()[0].Text()})
		}
	}
	return out
}

// pageAt reads the page that a GET of target is answered with.
func pageAt(t *testing.T, h http.Handler, target string) *page.Page {
	t.Helper()
	return page.Parse(t, get(h, target).Body.String())
}

// mainOf is the main part of a page.
func mainOf(t *testing.T, p *page.Page) page.El {
	t.Helper()
	return page.One(t, "main", p.ByRole("main", ""))
}

// titles are the titles of the panels of a page, in order.
func titles(p *page.Page) []string {
	var out []string
	for _, v := range p.Views() {
		if hs := v.ByRole("heading", ""); len(hs) > 0 {
			out = append(out, hs[0].Name())
		}
	}
	return out
}

// current are the entries of the sidebar that say they are where the visitor is, by name and address.
func current(t *testing.T, p *page.Page) []page.Link {
	t.Helper()
	var out []page.Link
	for _, l := range page.One(t, "the sidebar", p.ByRole("navigation", "Main")).Links() {
		if l.Current {
			out = append(out, l)
		}
	}
	return out
}

// panel is the panel that a title heads.
func panel(t *testing.T, p *page.Page, title string) page.El {
	t.Helper()
	var found []page.El
	for _, v := range p.Views() {
		if hs := v.ByRole("heading", title); len(hs) > 0 {
			found = append(found, v)
		}
	}
	return page.One(t, "the panel titled "+title, found)
}

// shown is whether a visitor reads text in the page.
func shown(p *page.Page, text string) bool { return len(p.ByText(text)) > 0 }

func handler(t *testing.T) http.Handler {
	t.Helper()
	return handlerAs(t, Editor)
}

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func post(h http.Handler, target string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// follow answers the redirect that rec is, as a browser does: a GET of its Location with the cookies it set. Every POST
// is answered with one, so that a reload does not send it again.
func follow(h http.Handler, rec *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, rec.Header().Get("Location"), nil)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge >= 0 {
			req.AddCookie(c)
		}
	}
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	return out
}

func flashOf(rec *httptest.ResponseRecorder) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "webui_flash" {
			raw, _ := base64.RawURLEncoding.DecodeString(c.Value)
			return string(raw)
		}
	}
	return ""
}

// searchBox is the handler as script asks it for the search box's list: with the header that a request for one panel carries, named for the box.
func searchBox(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Webui-View", "search")
		h.ServeHTTP(w, r)
	})
}

// linksTo are the links below the element, or the page, that lead to href.
func linksTo(e interface{ Links() []page.Link }, href string) []page.Link {
	var out []page.Link
	for _, l := range e.Links() {
		if l.Href == href {
			out = append(out, l)
		}
	}
	return out
}

// hasLink is whether there is a link of that name to that address.
func hasLink(e interface{ Links() []page.Link }, name, href string) bool {
	for _, l := range e.Links() {
		if l.Name == name && l.Href == href {
			return true
		}
	}
	return false
}

// toast is a message that a page gives the visitor about what they did: an alert when it is a failure, a status otherwise, and what it says.
type toast struct{ Role, Text string }

// toasts are the messages of a page, in order. The region that holds them is always there, and says nothing until one is put in it.
func toasts(t *testing.T, body string) []toast {
	t.Helper()
	var out []toast
	for _, e := range page.Parse(t, body).Live() {
		if e.Text() != "" {
			out = append(out, toast{e.Role(), e.Text()})
		}
	}
	return out
}

// named is how many of the links have that name.
func named(links []page.Link, name string) int {
	n := 0
	for _, l := range links {
		if l.Name == name {
			n++
		}
	}
	return n
}

// moment is a time that a page shows: the instant it holds, and the text a visitor reads for it.
type moment struct{ At, Text string }

// moments are the times below the element, in the order they are met.
func moments(e page.El) []moment {
	var out []moment
	for _, t := range e.ByAttr("datetime", "") {
		out = append(out, moment{At: t.Attr("datetime"), Text: t.Text()})
	}
	return out
}

// cells are the values of one column of the table, by its header, in the order of the rows.
func cells(t *testing.T, table page.El, header string) []string {
	t.Helper()
	i := slices.Index(table.Headers(), header)
	if i < 0 {
		t.Fatalf("no column %q in %v", header, table.Headers())
	}
	var out []string
	for _, row := range table.Rows() {
		out = append(out, row[i])
	}
	return out
}

// valuesOf is what the values of a toolbar's form say, by the label of each: the first thing in a value is its label and the last is what it reads.
func valuesOf(g page.El) map[string]string {
	got := map[string]string{}
	for _, value := range g.Children() {
		kids := value.Children()
		if len(kids) >= 2 {
			got[kids[0].Text()] = kids[len(kids)-1].Text()
		}
	}
	return got
}

// panelsOf are the panels of a page by the address of each (data-view) and its title, in the order they are met: where each is in the layouts.
func panelsOf(p *page.Page) [][2]string {
	var out [][2]string
	for _, v := range p.Views() {
		title := ""
		if hs := v.ByRole("heading", ""); len(hs) > 0 {
			title = hs[0].Name()
		}
		out = append(out, [2]string{v.Attr("data-view"), title})
	}
	return out
}
