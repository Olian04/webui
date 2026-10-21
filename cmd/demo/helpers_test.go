package main

import (
	"encoding/base64"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
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

var (
	searchLabel = regexp.MustCompile(`<div class="pal-label">(.*?)</div>`)
	searchItem  = regexp.MustCompile(`<a class="pal-item"[^>]*? href="([^"]*)"[^>]*>\s*<span>(.*?)</span>`)
)

// searchResults are what the search finds for q, in order, read out of the HTML that it answers script with: a heading for each page
// that found something, and a link under it for each result.
func searchResults(h http.Handler, q string) []searchResult {
	body := get(h, "/admin/_webui/search?fragment=1&q="+url.QueryEscape(q)).Body.String()
	var out []searchResult
	group := ""
	for body != "" {
		l, it := searchLabel.FindStringSubmatchIndex(body), searchItem.FindStringSubmatchIndex(body)
		switch {
		case l == nil && it == nil:
			return out
		case it == nil || (l != nil && l[0] < it[0]):
			group, body = html.UnescapeString(body[l[2]:l[3]]), body[l[1]:]
		default:
			out = append(out, searchResult{Group: group, Href: html.UnescapeString(body[it[2]:it[3]]), Title: html.UnescapeString(body[it[4]:it[5]])})
			body = body[it[1]:]
		}
	}
	return out
}

func handler(t *testing.T) http.Handler { return handlerAs(t, Editor) }

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
