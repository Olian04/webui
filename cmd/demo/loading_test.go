package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/test/util/assert"
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
	rail := get(h, "/admin/layout/rail").Body.String()
	assert.Contains(t, rail, `<div class="columns"><div class="col" style="--w:1;"><div class="panel grows"`)
	assert.Contains(t, rail, `</div><div class="col" style="--w:1;"><div class="stack">`)

	// Columns in columns: a Split that is the first child of a Split.
	assert.Contains(t, get(h, "/admin/layout/columns").Body.String(), `<div class="columns"><div class="col" style="--w:1;"><div class="columns">`)

	// Tabs each have a name of their own in the address, and only the selected tab of each is built.
	tabs := get(h, "/admin/layout/tabs?tabs-2.tab=status").Body.String()
	for _, name := range []string{"tabs.tab=", "tabs-2.tab=", "tabs-3.tab="} {
		assert.Contains(t, tabs, name)
	}
	assert.Contains(t, tabs, "Open alerts")
	assert.False(t, strings.Contains(tabs, "Mean rate"))

	// Five layouts down, and the choice of an inner Tabs is in the address.
	deep := get(h, "/admin/layout/deep?tabs-2.tab=status-and-a-form").Body.String()
	assert.Contains(t, deep, "A note")
	assert.False(t, strings.Contains(deep, "Mean rate"))

	// A page with an editor, a diff and Markdown links what they need in its head.
	review := get(h, "/admin/layout/review").Body.String()
	assert.Contains(t, review, `<script type="module" src="/admin/_webui/monaco/code-`)
	assert.Contains(t, review, "Collector configuration")
	assert.Contains(t, review, "Changes since the last deploy")
	assert.Contains(t, get(h, "/admin/layout/runbook").Body.String(), `<article class="md"`)
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
	assert.Contains(t, body, `<div class="arriving"`)
	assert.Contains(t, body, "Panel 1 (1 s) 1")
	assert.False(t, strings.Contains(body, "Panel 2 (2 s)"))
	assert.False(t, strings.Contains(body, `class="arrived"`))
}

// The tabs page is read with its tab, and a request for the Tabs alone is read without the rest: the stamp of a panel outside
// it is the moment that panel was read, and only the panels of the tab that was asked for have a new one.
func TestTheTabsPageReadsOnlyTheTabThatIsAskedFor(t *testing.T) {
	h := handler(t)
	page := get(h, "/admin/loading/tabs").Body.String()
	assert.Contains(t, page, "Above the tabs (read with the page)")
	assert.Contains(t, page, "Below the tabs (read with the page)")
	assert.Contains(t, page, "First, left (500 ms)")

	req := httptest.NewRequest(http.MethodGet, "/admin/loading/tabs?tabs.tab=second", nil)
	req.Header.Set("X-Webui-Leaf", "p.2")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.True(t, strings.HasPrefix(body, `<div class="tabset" data-tabs="p.2"`))
	assert.Contains(t, body, "Second (500 ms)")
	assert.False(t, strings.Contains(body, "Above the tabs"))
	assert.False(t, strings.Contains(body, "First, left"))
}

// A Tabs is not limited to a few tabs: the page has strips of three, six and sixteen, and the one that is chosen is the one that
// is read, wherever it is in the strip.
func TestTheManyTabsPageHasStripsOfThreeSixAndSixteenTabs(t *testing.T) {
	h := handler(t)
	page := get(h, "/admin/layout/many-tabs").Body.String()
	strips := regexp.MustCompile(`<nav class="tabs".*?</nav>`).FindAllString(page, -1)
	assert.Equal(t, len(strips), 4)
	var counts []int
	for _, s := range strips {
		counts = append(counts, strings.Count(s, `<a class="tab`))
	}
	assert.DeepEqual(t, counts, []int{3, 6, 6, 16})

	// The last of the sixteen is the one that is read when it is asked for, and the first is not.
	last := get(h, "/admin/layout/many-tabs?tabs-4.tab=system-status").Body.String()
	assert.Contains(t, last, "Sixteen / System status")
	assert.False(t, strings.Contains(last, "Sixteen / Overview"))
}
