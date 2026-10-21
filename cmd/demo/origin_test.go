package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func cancelHref(t *testing.T, h http.Handler, target string) string {
	t.Helper()

	body := get(h, target).Body.String()
	const marker = `<a class="btn btn-ghost btn-sm" href="`
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	body = body[i+len(marker):]
	return strings.ReplaceAll(body[:strings.Index(body, `"`)], "&amp;", "&")
}

func TestTheDeviceIsOpenedFromSeveralPagesAndEachRemembersItsOwn(t *testing.T) {
	h := handler(t)
	// Every page that links to a device remembers the address the user is on.
	for _, from := range []string{
		"/admin/device?devices.sort=ip&devices.offset=10",
		"/admin/site/Stockholm?open-alerts.sort=severity&devices.sort=id",
		"/admin/alert",
	} {
		list := get(h, from).Body.String()
		assert.Contains(t, list, "webui.from="+url.QueryEscape(from))
	}

	// And the device page's Cancel returns there, view state and all.
	from := "/admin/site/Stockholm?devices.sort=id&devices.offset=5"
	assert.Equal(t, cancelHref(t, h, "/admin/device/dev_27c38b?webui.from="+url.QueryEscape(from)), from)
	assert.Equal(t, cancelHref(t, h, "/admin/device/dev_27c38b?webui.from="+url.QueryEscape("/admin/alert")), "/admin/alert")
}

func TestCancelFallsBackToTheListWhenThereIsNoOrigin(t *testing.T) {
	h := handler(t)
	assert.Equal(t, cancelHref(t, h, "/admin/device/dev_27c38b"), "/admin/device")
	// A search result carries none, since a search is not a page.
	search := get(h, "/admin/_webui/search?fragment=1&q=dev_27c38b").Body.String()
	assert.Contains(t, search, `href="/admin/device/dev_27c38b"`)
	assert.False(t, strings.Contains(search, "webui.from"))
	// An address that is not in this app is ignored.
	assert.Equal(t, cancelHref(t, h, "/admin/device/dev_27c38b?webui.from="+url.QueryEscape("https://evil.example/")), "/admin/device")
}

func TestSavingReturnsToWhereTheDeviceWasOpenedFrom(t *testing.T) {
	h := handler(t)
	from := "/admin/site/Malm\u00f6?open-alerts.sort=severity"
	rec := post(h, "/admin/device/dev_27c75c?webui.from="+url.QueryEscape(from), url.Values{"_leaf": {"p.0.0"}, "f1_0": {"healthy"}, "f0_1": {"10.8.8.8"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	to, err := url.Parse(rec.Header().Get("Location")) // http.Redirect percent-encodes non-ASCII
	assert.NoError(t, err)
	assert.Equal(t, to.Path, "/admin/site/Malmö")
	assert.Equal(t, to.RawQuery, "open-alerts.sort=severity")

	// A rejection keeps the Cancel it had.
	taken, _ := service.Device("dev_27c38b")
	rec = post(h, "/admin/device/dev_27c75c?webui.from="+url.QueryEscape(from), url.Values{"_leaf": {"p.0.0"}, "f1_0": {"healthy"}, "f0_1": {taken.IP}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), `>Cancel</a>`)
}
