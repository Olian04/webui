package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// The weighted columns page has a row for each weighting it shows, and each column carries its weight.
func TestTheWeightedColumnsPageHasEachWeightingInItsOwnRow(t *testing.T) {
	rec := get(handler(t), "/admin/layout/weights")
	assert.Equal(t, rec.Code, http.StatusOK)
	p := page.Parse(t, rec.Body.String())

	// A panel for each column of each weighting, in the order they are declared; how wide each is is measured in the browser suite.
	assert.DeepEqual(t, titles(p), []string{"What this page shows",
		"Alerts: 1", "Devices: 3", "Devices: 3", "Collector", "Alerts: 25", "Devices: 50", "Collector", "One", "Two", "Three", "Four"})
	assert.DeepEqual(t, current(t, p), []page.Link{{Name: "Weighted columns", Href: "/admin/layout/weights", Current: true}})
}

// The long-list page puts a short list beside a long one, so the height cap and the scroll are on the screen.
func TestTheLongListPagePutsAShortListBesideALongOne(t *testing.T) {
	rec := get(handler(t), "/admin/layout/list")
	assert.Equal(t, rec.Code, http.StatusOK)
	p := page.Parse(t, rec.Body.String())

	assert.DeepEqual(t, current(t, p), []page.Link{{Name: "A long list", Href: "/admin/layout/list", Current: true}})
	few, many := panel(t, p, "A few items"), panel(t, p, "Many items")
	assert.Equal(t, len(few.ByRole("listitem", "")), shortListCount)
	assert.Equal(t, len(many.ByRole("listitem", "")), longListCount)
	assert.False(t, strings.Contains(few.AllText(), "Show more"))
	assert.False(t, strings.Contains(many.AllText(), "Show more"))
}
