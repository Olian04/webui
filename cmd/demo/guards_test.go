package main

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func TestViewerIsRefusedWhereGuarded(t *testing.T) {
	h := handlerAs(t, Viewer)

	// A page Guard: nothing was loaded, and the sidebar does not offer the page.
	assert.False(t, strings.Contains(pageAt(t, h, "/admin/device").Root().AllText(), "Audit log"))
	rec := get(h, "/admin/audit")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.True(t, shown(page.Parse(t, rec.Body.String()), "requires the editor role"))

	// Guarded controls: an ordinary error leaves them out, and with nothing to act with there is nothing to select with
	// either: no checkbox column, no bar, no column of actions.
	alerts := pageAt(t, h, "/admin/alert")
	assert.Equal(t, len(alerts.ByRole("button", "Dismiss alt_000")), 0)
	assert.Equal(t, len(alerts.ByAttr("name", "_act")), 0)
	_, picks := alerts.Field("_sel")
	assert.False(t, picks)
	table := page.One(t, "the alerts", alerts.ByRole("table", ""))
	assert.False(t, slices.Contains(table.Headers(), "Select")) // no column to pick with
	assert.False(t, slices.Contains(table.Headers(), "Actions"))

	alert := pageAt(t, h, "/admin/alert/alt_000") // a form: its submit is not offered, a way back is
	assert.Equal(t, len(alert.ByRole("button", "Acknowledge")), 0)
	assert.True(t, hasLink(alert, "Back", "/admin/alert"))

	// The same goes for a device's form: no Save and no Cancel, but a way back.
	assert.Equal(t, len(pageAt(t, h, "/admin/settings").ByRole("button", "Save")), 0)
	device := pageAt(t, h, "/admin/device/dev_27c38b")
	assert.Equal(t, len(device.ByRole("button", "Save")), 0)
	assert.Equal(t, named(device.Links(), "Cancel"), 0)
	assert.True(t, hasLink(device, "Back", "/admin/device"))
	_, offered := device.Field("f0_1")
	assert.False(t, offered) // the IP is shown, not offered for editing
}

// An editor is offered what a viewer is not, and a Guard that says Disabled draws the button disabled, with its sentence, for the rows it
// says it of: here a critical alert, which is acknowledged and not dismissed.
func TestADisabledActionIsDrawnWithItsReasonAndStillRefused(t *testing.T) {
	h := handlerAs(t, Editor)
	alerts := pageAt(t, h, "/admin/alert")
	assert.True(t, slices.Contains(page.One(t, "the alerts", alerts.ByRole("table", "")).Headers(), "Select"))
	assert.True(t, len(alerts.ByAttr("name", "_act")) > 0)
	assert.True(t, len(alerts.ByAllText("Not available: A critical alert is acknowledged, not dismissed")) > 0)

	var critical, other string
	for _, a := range service.OpenAlerts() {
		if a.Severity == "critical" && critical == "" {
			critical = a.ID
		}
		if a.Severity != "critical" && other == "" {
			other = a.ID
		}
	}
	assert.True(t, critical != "" && other != "")
	var posts []string
	for _, b := range alerts.ByAttr("name", "_act") {
		posts = append(posts, b.Attr("value"))
	}
	assert.True(t, slices.Contains(posts, "row:0:"+other)) // the button is a dialog's, whose own button submits

	// A request that is made by hand is refused for the same reason,.
	rec := post(h, "/admin/alert", url.Values{"_view": {"p"}, "_act": {"row:0:" + critical}})
	assert.Equal(t, flashOf(rec), "eA critical alert is acknowledged, not dismissed")
	_, still := service.Alert(critical)
	assert.True(t, still)
}

// A device, a site, an alert or a file that is not there is a 404 that says so, with a way back to the list.
func TestAnUnknownRecordIsA404WithALinkBack(t *testing.T) {
	for target, want := range map[string]struct{ text, href, label string }{
		"/admin/device/dev_nope": {`There is no device "dev_nope".`, "/admin/device", "Back to the devices"},
		"/admin/site/Nowhere":    {`There is no site "Nowhere".`, "/admin/site", "Back to the sites"},
		"/admin/alert/nope":      {`There is no alert "nope".`, "/admin/alert", "Back to the alerts"},
		"/admin/files/logs/nope": {`There is nothing at /logs/nope.`, "/admin/files", "Back to the files"},
	} {
		rec := get(handler(t), target)
		assert.Equal(t, rec.Code, http.StatusNotFound)
		refused := page.Parse(t, rec.Body.String())
		assert.Equal(t, len(refused.ByRole("heading", "Not found")), 1)
		assert.True(t, shown(refused, want.text))
		assert.True(t, hasLink(refused, want.label, want.href))
	}
}
