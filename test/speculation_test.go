package webui_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// A full page tells the browser where the rules are for fetching the app's links ahead, and the rules are
// served as the type a browser needs; nothing else the app serves carries them.
func TestAFullPageNamesTheRulesForFetchingLinksAhead(t *testing.T) {
	t.Parallel()

	s := newShop()
	page := serve(s.h, http.MethodGet, "/admin/device/dev1")
	rules := strings.Trim(page.Header().Get("Speculation-Rules"), `"`)
	assert.True(t, strings.HasPrefix(rules, "/admin/_webui/speculation."))

	file := serve(s.h, http.MethodGet, rules)
	assert.Equal(t, file.Code, http.StatusOK)
	assert.Equal(t, file.Header().Get("Content-Type"), "application/speculationrules+json")
	assert.Contains(t, file.Header().Get("Cache-Control"), "immutable")

	var parsed struct {
		Prefetch []struct {
			Where struct {
				And []map[string]any `json:"and"`
			} `json:"where"`
			Eagerness string `json:"eagerness"`
		} `json:"prefetch"`
	}
	assert.NoError(t, json.Unmarshal(file.Body.Bytes(), &parsed))
	assert.Equal(t, len(parsed.Prefetch), 1)
	assert.Equal(t, parsed.Prefetch[0].Eagerness, "moderate")
	assert.Equal(t, parsed.Prefetch[0].Where.And[0]["href_matches"], any("/admin/*"))
	body := file.Body.String()
	for _, left := range []string{"[download]", "[target]", "[data-leaf] th a", "[data-leaf] .pager a"} {
		assert.Contains(t, body, left)
	}
	assert.False(t, strings.Contains(body, "prerender")) // pages are no-store, which a browser does not prerender

	// Not on a panel that is fetched again, nor on what is not a page.
	req := httptest.NewRequest(http.MethodGet, "/admin/device/dev1", nil)
	req.Header.Set("X-Webui-Leaf", "p.0.1")
	panel := httptest.NewRecorder()
	s.h.ServeHTTP(panel, req)
	assert.Equal(t, panel.Header().Get("Speculation-Rules"), "")
	assert.Equal(t, file.Header().Get("Speculation-Rules"), "")
}

// A page fetched ahead takes the toast out of its cookie, and is thrown away when another link is followed
// instead: so a request made ahead of the visitor is refused while a toast waits, and the visitor's own
// request is the one that shows it.
func TestAPageFetchedAheadLeavesTheToastForTheVisitor(t *testing.T) {
	t.Parallel()

	s := newShop()
	ahead := func(cookie string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/admin/device/dev1", nil)
		req.Header.Set("Sec-Purpose", "prefetch")
		if cookie != "" {
			req.Header.Set("Cookie", "webui_flash="+cookie)
		}
		rec := httptest.NewRecorder()
		s.h.ServeHTTP(rec, req)
		return rec
	}

	refused := ahead("dGhpcw") // "this"
	assert.Equal(t, refused.Code, http.StatusServiceUnavailable)
	assert.Equal(t, len(refused.Result().Cookies()), 0) // the cookie is not cleared
	assert.False(t, strings.Contains(refused.Body.String(), "this"))

	req := httptest.NewRequest(http.MethodGet, "/admin/device/dev1", nil)
	req.Header.Set("Cookie", "webui_flash=dGhpcw")
	shown := httptest.NewRecorder()
	s.h.ServeHTTP(shown, req)
	assert.Equal(t, shown.Code, http.StatusOK)
	assert.Contains(t, shown.Body.String(), `class="toast`)
	assert.Equal(t, shown.Result().Cookies()[0].MaxAge, -1)

	// With nothing waiting, a request made ahead is an ordinary page.
	plain := ahead("")
	assert.Equal(t, plain.Code, http.StatusOK)
	assert.Contains(t, plain.Body.String(), "<!doctype html>")
}
