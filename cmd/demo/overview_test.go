package main

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/test/util/assert"
)

// The landing page is the page that shows the library all at once: the collector's status in a toolbar, and four panels in a Split.
func TestTheLandingPageShowsTheCollectorInAToolbarAndFourPanels(t *testing.T) {
	body := get(handler(t), "/admin/").Body.String()

	bar := body[strings.Index(body, `<div class="toolbar"`):strings.Index(body, `<div class="panel`)] // the toolbar, up to the first panel after it
	for _, label := range []string{"Health", "Version", "Uptime", "Devices", "Queue depth", "Disk used (%)"} {
		assert.Contains(t, bar, `<span class="toolbar-lbl">`+label+`</span>`)
	}
	// An editor has the button at the right edge, with the gap that puts it there; the values are from the left.
	assert.True(t, strings.Index(bar, `toolbar-gap`) > strings.Index(bar, "Disk used"))
	assert.Contains(t, bar, `>Sync now</button>`)

	for _, title := range []string{"Devices", "Open alerts", "Recent events", "Collector settings"} {
		assert.Contains(t, body, `<h2 class="panel-title">`+title+`</h2>`)
	}
	assert.Contains(t, body, `1–9 of 37`) // the devices page, as the table on its own page does
	assert.Contains(t, body, `1–9 of 11`) // and the alerts
	assert.Contains(t, body, `1–8 of 8`)  // and the events, all of them
	assert.Contains(t, body, `>Dismiss</button>`)
	assert.Contains(t, body, `data-guard="A critical alert is acknowledged, not dismissed"`) // disabled, with its reason, for a critical alert
	assert.Contains(t, body, `>Save</button>`)
}

// A viewer sees what they may use: the toolbar is values only, with no button and no form, the alerts have no button, and the form is read.
func TestTheLandingPageForAViewerIsWhatTheyMayUse(t *testing.T) {
	body := get(handlerAs(t, Viewer), "/admin/").Body.String()
	bar := body[strings.Index(body, `<div class="toolbar"`):strings.Index(body, `<div class="panel`)] // the toolbar, up to the first panel after it
	assert.Contains(t, bar, `<span class="toolbar-lbl">Health</span>`)
	assert.False(t, strings.Contains(bar, "<form") || strings.Contains(bar, "btn"))
	assert.False(t, strings.Contains(body, ">Dismiss</button>"))
	assert.False(t, strings.Contains(body, ">Save</button>"))
	assert.False(t, strings.Contains(body, `class="gate"`))
}

// The form on the landing page is the Ingest page's, in part, and saves the same settings.
func TestTheSettingsFormOnTheLandingPageSavesTheSettings(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/").Body.String()
	leaf := regexp.MustCompile(`name="_leaf" value="([^"]*)"`).FindAllStringSubmatch(body[strings.Index(body, "Collector settings"):], 1)
	assert.Equal(t, len(leaf), 1)

	form := url.Values{"_leaf": {leaf[0][1]}, "f0_0": {"0.5"}, "f0_1": {"60"}, "f1_1": {"ops@example.com"}, "f2_1": {"#ff8800"}}
	rec := post(h, "/admin/", form)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, service.Settings().Contact, "ops@example.com")
	assert.Equal(t, service.Settings().Label, "#ff8800")

	refused := post(h, "/admin/", url.Values{"_leaf": {leaf[0][1]}, "f0_0": {"7"}})
	assert.Equal(t, refused.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, refused.Body.String(), "Must be at most 1.")
}

// Sync now starts a job and the panel that follows it says it is not final for as long as it goes on: it asks to be asked again, and when it is
// done it does not, which is what ends it.
func TestTheSyncPanelAsksToBeAskedAgainWhileTheJobGoesOn(t *testing.T) {
	h := handler(t)
	idle := get(h, "/admin/").Body.String()
	assert.Contains(t, idle, `>Idle<`)
	assert.False(t, strings.Contains(idle, "data-refresh-after"))

	service.StartSync()
	going := get(h, "/admin/").Body.String()
	assert.Contains(t, going, `data-refresh-after="2"`)
	assert.Contains(t, going, "Syncing")
	assert.Contains(t, going, "Updating, reload in 2 s")

	service.sync = time.Now().Add(-syncTime) // it has been going as long as it takes
	done := get(h, "/admin/").Body.String()
	assert.Contains(t, done, ">Done<")
	assert.False(t, strings.Contains(done, "data-refresh-after"))
}
