package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestViewerIsRefusedWhereGuarded(t *testing.T) {
	h := handlerAs(t, Viewer)

	// A page Guard: nothing was loaded, and the sidebar does not offer the page.
	assert.False(t, strings.Contains(get(h, "/admin/device").Body.String(), "Audit log"))
	rec := get(h, "/admin/audit")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Contains(t, rec.Body.String(), "requires the editor role")

	// Guarded controls: an ordinary error leaves them out, and with nothing to act with there is nothing to select with
	// either: no checkbox column, no bar, no column of actions.
	body := get(h, "/admin/alert").Body.String()
	assert.False(t, strings.Contains(body, ">Dismiss</button>"))
	assert.False(t, strings.Contains(body, `class="gate"`))
	for _, absent := range []string{`class="pick"`, `class="actionbar"`, `class="actions"`, `name="_sel"`} {
		assert.False(t, strings.Contains(body, absent))
	}

	alert := get(h, "/admin/alert/alt_000").Body.String() // a form: its submit is not offered, a way back is
	assert.False(t, strings.Contains(alert, ">Acknowledge</button>"))
	assert.Contains(t, alert, `href="/admin/alert">Back</a>`)

	// The same goes for a device's form: no Save and no Cancel, but a way back.
	assert.False(t, strings.Contains(get(h, "/admin/settings").Body.String(), ">Save</button>"))
	device := get(h, "/admin/device/dev_27c38b").Body.String()
	assert.False(t, strings.Contains(device, ">Save</button>"))
	assert.False(t, strings.Contains(device, ">Cancel</a>"))
	assert.Contains(t, device, `href="/admin/device">Back</a>`)
	assert.False(t, strings.Contains(device, `name="f0_1"`)) // the IP is shown, not offered for editing
}

// An editor is offered what a viewer is not, and a Guard that says Disabled draws the button disabled, with its sentence, for the rows it
// says it of: here a critical alert, which is acknowledged and not dismissed.
func TestADisabledActionIsDrawnWithItsReasonAndStillRefused(t *testing.T) {
	h := handlerAs(t, Editor)
	body := get(h, "/admin/alert").Body.String()
	assert.Contains(t, body, `class="pick"`)
	assert.Contains(t, body, `class="actionbar"`)
	assert.Contains(t, body, `<span class="gate" data-guard="A critical alert is acknowledged, not dismissed">`)

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
	assert.Contains(t, body, `value="row:0:`+other+`">Dismiss</button>`) // the button is a dialog's, whose own button submits

	// A request that is made by hand is refused for the same reason,.
	rec := post(h, "/admin/alert", url.Values{"_leaf": {"p"}, "_act": {"row:0:" + critical}})
	assert.Equal(t, flashOf(rec), "eA critical alert is acknowledged, not dismissed")
	_, still := service.Alert(critical)
	assert.True(t, still)
}

// A device, a site, an alert or a file that is not there is a 404 that says so, with a way back to the list.
func TestAnUnknownRecordIsA404WithALinkBack(t *testing.T) {
	for target, want := range map[string]struct{ text, href, label string }{
		"/admin/device/dev_nope": {`There is no device &#34;dev_nope&#34;.`, "/admin/device", "Back to the devices"},
		"/admin/site/Nowhere":    {`There is no site &#34;Nowhere&#34;.`, "/admin/site", "Back to the sites"},
		"/admin/alert/nope":      {`There is no alert &#34;nope&#34;.`, "/admin/alert", "Back to the alerts"},
		"/admin/files/logs/nope": {`There is nothing at /logs/nope.`, "/admin/files", "Back to the files"},
	} {
		rec := get(handler(t), target)
		assert.Equal(t, rec.Code, http.StatusNotFound)
		body := rec.Body.String()
		assert.Contains(t, body, `<p class="state-title">Not found</p>`)
		assert.Contains(t, body, want.text)
		assert.Contains(t, body, `<a class="btn btn-ghost btn-sm" href="`+want.href+`">`+want.label+`</a>`)
	}
}
