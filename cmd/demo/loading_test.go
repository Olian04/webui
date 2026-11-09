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

// The Layout pages are combinations that no other page of the demo has, and each is the layout it says.
func TestTheLayoutPagesAreTheCombinationsTheySay(t *testing.T) {
	h := handler(t)

	// A rail: a Stack in a Split, beside a table.
	assert.DeepEqual(t, panelsOf(pageAt(t, h, "/admin/layout/rail")), [][2]string{
		{"p.0", "What this page shows"}, {"p.1.0", "Devices"}, {"p.1.1.0", "Collector"}, {"p.1.1.1", "Open alerts"}})

	// Columns in columns: a Split that is the first child of a Split.
	columns := panelsOf(pageAt(t, h, "/admin/layout/columns"))
	assert.DeepEqual(t, columns[1:4], [][2]string{{"p.1.0.0", "Alerts"}, {"p.1.0.1", "Collector"}, {"p.1.1", "Devices"}})

	// Tabs each have a name of their own in the address, and only the selected tab of each is built.
	tabs := pageAt(t, h, "/admin/layout/tabs?tabs-2.tab=status")
	for _, name := range []string{"tabs.tab=", "tabs-2.tab=", "tabs-3.tab="} {
		found := false
		for _, l := range tabs.Links() {
			found = found || strings.Contains(l.Href, name)
		}
		assert.True(t, found)
	}
	assert.True(t, shown(tabs, "Open alerts"))
	assert.False(t, shown(tabs, "Mean rate"))

	// Five layouts down, and the choice of an inner Tabs is in the address.
	deep := get(h, "/admin/layout/deep?tabs-2.tab=status-and-a-form").Body.String()
	assert.Contains(t, deep, "A note")
	assert.False(t, strings.Contains(deep, "Mean rate"))

	// A page with an editor, a diff and Markdown links what they need in its head.
	review := get(h, "/admin/layout/review").Body.String()
	assert.Contains(t, review, `<script type="module" src="/admin/_webui/monaco/code-`)
	reviewed := page.Parse(t, review)
	assert.True(t, shown(reviewed, "Collector configuration"))
	assert.True(t, shown(reviewed, "Changes since the last deploy"))
	assert.True(t, len(pageAt(t, h, "/admin/layout/runbook").ByRole("article", "")) > 0)
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

// A Tabs is not limited to a few tabs: the page has strips of three, six and sixteen, and the one that is chosen is the one that
// is read, wherever it is in the strip.
func TestTheManyTabsPageHasStripsOfThreeSixAndSixteenTabs(t *testing.T) {
	h := handler(t)
	strips := pageAt(t, h, "/admin/layout/many-tabs").ByRoleContaining("navigation", "Views:")
	assert.Equal(t, len(strips), 4)
	var counts []int
	for _, s := range strips {
		counts = append(counts, len(s.Links()))
	}
	assert.DeepEqual(t, counts, []int{3, 6, 6, 16})

	// The last of the sixteen is the one that is read when it is asked for, and the first is not.
	last := get(h, "/admin/layout/many-tabs?tabs-4.tab=system-status").Body.String()
	assert.Contains(t, last, "Sixteen / System status")
	assert.False(t, strings.Contains(last, "Sixteen / Overview"))
}
