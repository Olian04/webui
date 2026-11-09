package main

import (
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// The Activity page is what is new: a list of alerts that lead to their own pages, releases that are on another site, the people on call with their pictures,
// and the dashboards as cards in a row that scrolls.
func TestTheActivityPageIsListsAndCards(t *testing.T) {
	p := pageAt(t, handler(t), "/admin/activity")

	assert.DeepEqual(t, current(t, p), []page.Link{{Name: "Activity", Href: "/admin/activity", Current: true}})
	for _, title := range []string{"Open alerts", "People", "Dashboards"} {
		panel(t, p, title)
	}

	// The alerts are the first tab, four of them, each a link to the alert's own page, with the severity as a badge in words.
	alerts := panel(t, p, "Open alerts").ByRole("listitem", "")
	assert.Equal(t, len(alerts), 4)
	for _, item := range alerts {
		link := item.Links()
		assert.Equal(t, len(link), 1)
		assert.True(t, strings.HasPrefix(link[0].Href, "/admin/alert/"))
		assert.True(t, strings.Contains(item.Text(), "critical") || strings.Contains(item.Text(), "warning") || strings.Contains(item.Text(), "info"))
	}
	assert.Equal(t, len(linksTo(panel(t, p, "Open alerts"), "/admin/activity?open-alerts.limit=8")), 1) // and there are more

	// The pictures are on this site, so no origin is trusted and the page's policy is the library's own.
	assert.Equal(t, len(p.ByAttr("src", "/art/ada-lovelace.png")), 1)
	assert.Equal(t, len(p.ByAttr("src", "/art/ingest-overview.png")), 1)
	assert.Equal(t, len(panel(t, p, "Dashboards").ByRole("listitem", "")), 4)
	assert.Equal(t, len(p.ByRole("region", "Dashboards, scrolls sideways")), 1)
}

func TestTheReleasesAreOnAnotherSiteAndOpenATabOfTheirOwn(t *testing.T) {
	p := pageAt(t, handler(t), "/admin/activity?tabs.tab=releases")
	releases := panel(t, p, "Releases")
	first := releases.Links()[0]
	assert.DeepEqual(t, first, page.Link{Name: "2.4.1: fixes a stall in the ingest queue(opens in a new tab)", Href: "https://example.com/releases/2.4.1", NewTab: true})
	assert.True(t, shown(p, "1204 downloads"))
	assert.True(t, shown(p, "3h ago"))
	assert.Equal(t, len(p.ByRole("heading", "Open alerts")), 0) // the other tab is not read
	assert.Equal(t, len(linksTo(releases, "/admin/activity?releases.limit=8&tabs.tab=releases")), 1)

	// More of them is more in the address, and the tab stays.
	more := panel(t, pageAt(t, handler(t), "/admin/activity?tabs.tab=releases&releases.limit=8"), "Releases")
	assert.Equal(t, len(more.ByRole("listitem", "")), 7) // all seven
	for _, l := range more.Links() {
		assert.False(t, strings.Contains(l.Href, "releases.limit=")) // and there is no more to show
	}
}

// A viewer has the same list, and each alert is a link for them too, since the alert's own page lets a viewer in: an item whose page refuses the visitor is
// text and not a link.
func TestAViewerHasTheSameListAndNoMoreThanTheyMayOpen(t *testing.T) {
	p := pageAt(t, handlerAs(t, Viewer), "/admin/activity")
	for _, item := range panel(t, p, "Open alerts").ByRole("listitem", "") {
		assert.Equal(t, len(item.Links()), 1)
	}
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

// The release calendar is the month of the demo's day, with a mark on each day something was published and a link to the page that lists it.
func TestTheReleaseCalendarMarksTheDaysSomethingWasPublishedAndLinksToThem(t *testing.T) {
	p := pageAt(t, handler(t), "/admin/activity")
	calendar := panel(t, p, "Release calendar")
	assert.Equal(t, len(calendar.ByRole("heading", "October 2026")), 1)
	today := page.One(t, "today", calendar.ByAttr("aria-current", "date")) // the 9th, which is the day of the demo's data
	assert.True(t, strings.HasPrefix(today.Name(), "Friday 9 October 2026"))
	assert.True(t, hasLink(calendar, "Friday 9 October 2026, 1 item", "/admin/activity/releases/2026-10-09"))   // today, which has a release
	assert.True(t, hasLink(calendar, "Thursday 8 October 2026, 1 item", "/admin/activity/releases/2026-10-08")) // the beta, the day before
	assert.Equal(t, len(linksTo(calendar, "/admin/activity?release-calendar.month=2026-09")), 1)                // and the month before, which is in the address
	assert.Equal(t, len(linksTo(calendar, "/admin/activity/releases/2026-10-10")), 0)                           // a day with nothing on it leads nowhere
	table := page.One(t, "the month", calendar.ByRole("table", "October 2026"))
	assert.Equal(t, table.Rows()[1][0], "Week 41") // the week of the 5th
}

func TestADayOfTheCalendarLeadsToAPageThatListsWhatWasPublishedOnIt(t *testing.T) {
	h := handler(t)
	rec := get(h, "/admin/activity/releases/2026-10-03")
	assert.Equal(t, rec.Code, http.StatusOK)
	p := page.Parse(t, rec.Body.String())
	assert.True(t, shown(p, "2.4.0: a search that finds devices by tag"))
	assert.False(t, strings.Contains(p.Root().Text(), "2.4.1"))
	assert.DeepEqual(t, current(t, p), []page.Link{{Name: "Activity", Href: "/admin/activity", Current: true}}) // under the Activity page, so it lights it

	assert.Equal(t, get(h, "/admin/activity/releases/not-a-day").Code, http.StatusNotFound)
}

// The month before has what was published in it, and the link back to the current month has no month in it.
func TestTheCalendarOfAnotherMonthShowsItsOwnReleasesAndLinksBack(t *testing.T) {
	calendar := panel(t, pageAt(t, handler(t), "/admin/activity?release-calendar.month=2026-09"), "Release calendar")
	assert.Equal(t, len(calendar.ByRole("heading", "September 2026")), 1)
	assert.Equal(t, len(linksTo(calendar, "/admin/activity/releases/2026-09-25")), 1)
	today := page.One(t, "the link back", calendar.ByRole("link", "Today"))
	assert.Equal(t, today.Attr("href"), "/admin/activity")
}

// The escalation notice is a When: it is on the page while critical alerts are open, and not once they have been acknowledged.
// The escalation notice is a When: it is on the page while critical alerts are open, and not once they have been acknowledged.
func TestTheEscalationNoticeIsShownOnlyWhileCriticalAlertsAreOpen(t *testing.T) {
	h := handler(t)
	p := pageAt(t, h, "/admin/activity")
	assert.True(t, strings.Contains(panel(t, p, "Escalation").Text(), "critical alerts are open"))

	var critical []string
	for _, a := range service.OpenAlerts() {
		if a.Severity == "critical" {
			critical = append(critical, a.ID)
		}
	}
	assert.True(t, len(critical) > 0)
	service.Acknowledge(critical...)

	rec := get(h, "/admin/activity")
	assert.Equal(t, rec.Code, http.StatusOK)
	after := page.Parse(t, rec.Body.String())
	assert.Equal(t, len(after.ByRole("heading", "Escalation")), 0)
	panel(t, after, "Release calendar") // the rest of the page is as it was
}
