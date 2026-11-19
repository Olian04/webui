package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// The Loading pages are the cases of a page that is sent as it is built, and each says what it is for.
func TestTheLoadingPagesShowTheirCases(t *testing.T) {
	h := handler(t)

	// Panels are drawn in the order the page has them, whichever answered first.
	order := get(h, "/admin/loading/order").Body.String()
	last := -1
	for _, panel := range []string{"Quick (10 ms) 1", "Slow (400 ms) 1", "Medium (250 ms) 1", "Last (50 ms) 1"} {
		at := strings.Index(order, panel)
		assert.True(t, at > last)
		last = at
	}

	// Layouts inside layouts.
	nested := get(h, "/admin/loading/nested").Body.String()
	for _, panel := range []string{"Split, left (300 ms) 1", "Split, right (20 ms) 1", "Selected tab (200 ms) 1", "After the layouts (100 ms) 1"} {
		assert.Contains(t, nested, panel)
	}

	// A panel that fails, by error or by panic, says so, and the panel after it is there.
	failures := get(h, "/admin/loading/failures")
	assert.Equal(t, failures.Code, http.StatusOK)
	assert.Equal(t, strings.Count(failures.Body.String(), "Could not load"), 2)
	assert.Contains(t, failures.Body.String(), "After the failures (10 ms) 1")
	assert.False(t, strings.Contains(failures.Body.String(), "fell over"))

	// A page with an editor is built whole.
	assert.Contains(t, get(h, "/admin/loading/whole").Body.String(), `<script type="module" src="/admin/_webui/monaco/code-`)

	// A Guard that refuses is a 403, and the page is for editors.
	assert.Equal(t, get(handlerAs(t, Viewer), "/admin/loading/guarded").Code, http.StatusForbidden)
	assert.Equal(t, get(h, "/admin/loading/guarded").Code, http.StatusOK)
	assert.Contains(t, get(h, "/admin/loading/fast").Body.String(), "Instant 1")
}

// The staircase has ten panels, the first after a second and each next one a second later, so a visitor who goes
// after a second and a half has the first of them and the spinner, and none of the rest.
func TestTheStaircaseArrivesOnePanelAtATime(t *testing.T) {
	h := handler(t)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/loading/staircase", nil).WithContext(ctx))

	body := rec.Body.String()
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.True(t, shown(page.Parse(t, body), "Panel 1 (1 s) 1"))
	assert.False(t, strings.Contains(body, "Panel 2 (2 s)"))
}

// The tabs page is read with its tab, and a request for the Tabs alone is read without the rest: the stamp of a panel outside
// it is the moment that panel was read, and only the panels of the tab that was asked for have a new one.
func TestTheTabsPageReadsOnlyTheTabThatIsAskedFor(t *testing.T) {
	h := handler(t)
	whole := pageAt(t, h, "/admin/loading/tabs")
	assert.True(t, shown(whole, "Above the tabs (read with the page)"))
	assert.True(t, shown(whole, "Below the tabs (read with the page)"))
	assert.True(t, shown(whole, "First, left (500 ms)"))

	req := httptest.NewRequest(http.MethodGet, "/admin/loading/tabs?tabs.tab=second", nil)
	req.Header.Set("X-Webui-View", "p.2")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	assert.Equal(t, rec.Code, http.StatusOK)
	alone := page.Parse(t, body)
	assert.True(t, page.IsFragment(body))
	assert.Equal(t, len(alone.ByAttr("data-tabs", "p.2")), 1)
	assert.True(t, shown(alone, "Second (500 ms)"))
	assert.False(t, strings.Contains(body, "Above the tabs"))
	assert.False(t, strings.Contains(body, "First, left"))
}
