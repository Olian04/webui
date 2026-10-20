package main

import (
	"net/http"
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

	// Gated controls: disabled, with the reason.
	body := get(h, "/admin/alert").Body.String()
	assert.Contains(t, body, `<span class="gate" data-guard="requires the editor role">`)
	assert.Contains(t, body, `disabled>Dismiss</button>`)

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
