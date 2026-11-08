package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// The landing page is the page that shows the library all at once: the collector's status in a toolbar, and four panels in a Split.
func TestTheLandingPageShowsTheCollectorInAToolbarAndFourPanels(t *testing.T) {
	p := pageAt(t, handler(t), "/admin/")

	bar := p.Views()[0] // the toolbar, which is the first panel
	system := valuesOf(page.One(t, "the System", bar.ByRole("group", "System")))
	for _, label := range []string{"Health", "Version", "Uptime", "Devices", "Queue depth", "Disk used (%)"} {
		_, ok := system[label]
		assert.True(t, ok)
	}
	// An editor has the button at the right edge (the browser suite measures it); the values are before it.
	sync := page.One(t, "the Sync group", bar.ByRole("group", "Sync"))
	assert.True(t, page.One(t, "the System", bar.ByRole("group", "System")).Before(sync))
	assert.True(t, len(bar.ByRole("button", "Sync now")) > 0)

	assert.DeepEqual(t, titles(p), []string{"Devices", "Open alerts", "Recent events", "Collector settings"})
	assert.True(t, shown(p, "1–9 of 37")) // the devices page, as the table on its own page does
	assert.True(t, shown(p, "1–9 of 11")) // and the alerts
	assert.True(t, shown(p, "1–8 of 8"))  // and the events, all of them
	assert.True(t, len(p.ByRoleContaining("button", "Dismiss")) > 0)
	assert.True(t, len(p.ByAllText("Not available: A critical alert is acknowledged, not dismissed")) > 0) // disabled, with its reason, for a critical alert
	assert.Equal(t, len(p.ByRole("button", "Save")), 1)
}

// A viewer sees what they may use: the toolbar is values only, with no button and no form, the alerts have no button, and the form is read.
func TestTheLandingPageForAViewerIsWhatTheyMayUse(t *testing.T) {
	p := pageAt(t, handlerAs(t, Viewer), "/admin/")
	bar := p.Views()[0] // the toolbar, which is the first panel
	_, ok := valuesOf(page.One(t, "the System", bar.ByRole("group", "System")))["Health"]
	assert.True(t, ok)
	assert.Equal(t, len(bar.ByRole("button", "")), 0)
	assert.Equal(t, len(bar.Fields()), 0)
	assert.Equal(t, len(p.ByRoleContaining("button", "Dismiss")), 0)
	assert.Equal(t, len(p.ByRole("button", "Save")), 0)
	assert.False(t, strings.Contains(p.Root().AllText(), "Not available"))
}

// The form on the landing page is the Ingest page's, in part, and saves the same settings.
func TestTheSettingsFormOnTheLandingPageSavesTheSettings(t *testing.T) {
	h := handler(t)
	settings := panel(t, pageAt(t, h, "/admin/"), "Collector settings")
	view, ok := settings.Field("_view")
	assert.True(t, ok)

	form := url.Values{"_view": {view.Value}, "f0_0": {"0.5"}, "f0_1": {"60"}, "f1_1": {"ops@example.com"}, "f2_1": {"#ff8800"}}
	rec := post(h, "/admin/", form)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, service.Settings().Contact, "ops@example.com")
	assert.Equal(t, service.Settings().Label, "#ff8800")

	refused := post(h, "/admin/", url.Values{"_view": {view.Value}, "f0_0": {"7"}})
	assert.Equal(t, refused.Code, http.StatusUnprocessableEntity)
	assert.True(t, shown(page.Parse(t, refused.Body.String()), "Must be at most 1."))
}

// Sync now starts a job and the toolbar that follows it says it is not final for as long as it goes on: it asks to be asked again, and when it is
// done it does not, which is what ends it.
func TestTheToolbarAsksToBeAskedAgainWhileTheSyncJobGoesOn(t *testing.T) {
	h := handler(t)
	bar := func() page.El { return pageAt(t, h, "/admin/").Views()[0] }
	idle := bar()
	assert.Equal(t, valuesOf(page.One(t, "the Sync", idle.ByRole("group", "Sync")))["Sync"], "Idle")
	assert.False(t, idle.Has("data-refresh-after"))

	service.StartSync()
	going := bar()
	assert.Equal(t, going.Attr("data-view"), "p.0")
	assert.Equal(t, going.Attr("data-refresh-after"), "2")
	assert.True(t, strings.HasPrefix(valuesOf(page.One(t, "the Sync", going.ByRole("group", "Sync")))["Sync"], "Syncing, 0 of "))
	assert.False(t, strings.Contains(strings.ToLower(going.AllText()), "reload in")) // the row has no room for a note

	service.sync = time.Now().Add(-syncTime) // it has been going as long as it takes
	done := bar()
	assert.Equal(t, valuesOf(page.One(t, "the Sync", done.ByRole("group", "Sync")))["Sync"], "Done")
	assert.False(t, done.Has("data-refresh-after"))
}

// The toolbar of the landing page has a Trend: the latency's shape over the day, and how much it moved since yesterday. A latency that falls is good news,
// whichever way the arrow points.
func TestTheLandingPageToolbarShowsALatencyTrend(t *testing.T) {
	bar := pageAt(t, handler(t), "/admin/").Views()[0]
	latency := page.One(t, "the Throughput", bar.ByRole("group", "Throughput"))
	assert.Equal(t, valuesOf(latency)["Latency"], "−6.10% (better)") // latency is down, which is good, whichever way the arrow points
	assert.Equal(t, len(bar.ByRole("img", "Latency: 24 values, from 140 to 184, lowest 134, highest 210")), 1)
}
