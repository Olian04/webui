package main

import (
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// The Activity page is what is new: a list of alerts that lead to their own pages, releases that are on another site, the people on call with their pictures,
// and the dashboards as cards in a row that scrolls.
func TestTheActivityPageIsListsAndCards(t *testing.T) {
	body := get(handler(t), "/admin/activity").Body.String()

	assert.Contains(t, body, `class="nav-item active" href="/admin/activity"`)
	for _, title := range []string{"Open alerts", "People", "Dashboards"} {
		assert.Contains(t, body, `<h2 class="panel-title">`+title+`</h2>`)
	}

	// The alerts are the first tab, four of them, each a link to the alert's own page, with the severity as a badge in words.
	assert.Equal(t, strings.Count(body[strings.Index(body, `<h2 class="panel-title">Open alerts`):strings.Index(body, `<h2 class="panel-title">People`)], `<li class="item">`), 4)
	assert.Contains(t, body, `<a class="item-link" href="/admin/alert/`)
	assert.Contains(t, body, `class="badge badge-bad"`)
	assert.Contains(t, body, `href="/admin/activity?open-alerts.limit=8"`) // and there are more

	// The pictures are on this site, so no origin is trusted and the page's policy is the library's own.
	assert.Contains(t, body, `<img class="item-img" src="/art/ada-lovelace.png"`)
	assert.Contains(t, body, `<img class="item-img" src="/art/ingest-overview.png"`)
	assert.Equal(t, strings.Count(body, `<li class="item item-card">`), 4)
	assert.Contains(t, body, `<div class="cards-scroller" tabindex="0" role="region" aria-label="Dashboards, scrolls sideways">`)
}

func TestTheReleasesAreOnAnotherSiteAndOpenATabOfTheirOwn(t *testing.T) {
	body := get(handler(t), "/admin/activity?tabs.tab=releases").Body.String()
	assert.Contains(t, body, `<a class="item-link ext" href="https://example.com/releases/2.4.1" target="_blank" rel="noopener noreferrer">2.4.1: fixes a stall in the ingest queue`)
	assert.Contains(t, body, "(opens in a new tab)")
	assert.Contains(t, body, "1204 downloads")
	assert.Contains(t, body, "3h ago")
	assert.False(t, strings.Contains(body, "Open alerts</h2>")) // the other tab is not read
	assert.Contains(t, body, `href="/admin/activity?releases.limit=8&amp;tabs.tab=releases"`)

	// More of them is more in the address, and the tab stays.
	more := get(handler(t), "/admin/activity?tabs.tab=releases&releases.limit=8").Body.String()
	releases := more[strings.Index(more, `<h2 class="panel-title">Releases`):strings.Index(more, `<h2 class="panel-title">People`)]
	assert.Equal(t, strings.Count(releases, `<li class="item">`), 7) // all seven
	assert.False(t, strings.Contains(releases, "releases.limit="))   // and there is no more to show
}

// A viewer has the same list, and each alert is a link for them too, since the alert's own page lets a viewer in: an item whose page refuses the visitor is
// text and not a link.
func TestAViewerHasTheSameListAndNoMoreThanTheyMayOpen(t *testing.T) {
	body := get(handlerAs(t, Viewer), "/admin/activity").Body.String()
	assert.Contains(t, body, `<a class="item-link" href="/admin/alert/`)
}

func TestThePicturesOfThePageAreMadeFromTheirNamesAndAreTheSameEveryTime(t *testing.T) {
	h := handler(t)
	first := get(h, "/art/ada-lovelace.png")
	assert.Equal(t, first.Code, http.StatusOK)
	assert.Equal(t, first.Header().Get("Content-Type"), "image/png")
	assert.Equal(t, first.Header().Get("X-Content-Type-Options"), "nosniff")
	img, err := png.Decode(first.Body)
	assert.NoError(t, err)
	assert.Equal(t, img.Bounds().Dx(), 320)

	again := get(h, "/art/ada-lovelace.png")
	other := get(h, "/art/grace-hopper.png")
	b1, b2 := get(h, "/art/ada-lovelace.png").Body.Bytes(), other.Body.Bytes()
	assert.Equal(t, again.Code, http.StatusOK)
	assert.True(t, string(b1) != string(b2)) // another name, another picture
	assert.Equal(t, string(b1), get(h, "/art/ada-lovelace.png").Body.String())

	assert.Equal(t, get(h, "/art/"+strings.Repeat("x", 80)+".png").Code, http.StatusNotFound)
	assert.Equal(t, get(h, "/art/.png").Code, http.StatusNotFound)
}
