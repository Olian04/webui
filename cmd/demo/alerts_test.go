package main

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func TestAnAlertHasItsOwnPageAndItsRowsOpenIt(t *testing.T) {
	h := handler(t)
	assert.Equal(t, len(linksTo(pageAt(t, h, "/admin/alert"), "/admin/alert/alt_000?webui.from=%2Fadmin%2Falert")), 1)
	assert.Equal(t, len(linksTo(pageAt(t, h, "/admin/site/Stockholm"), "/admin/alert/alt_000?webui.from=%2Fadmin%2Fsite%2FStockholm")), 1)

	// It shows the alert, lights Alerts in the sidebar, and cancels to the list.
	alert := pageAt(t, h, "/admin/alert/alt_000")
	assert.True(t, shown(alert, "Ingest lag above 5s"))
	assert.DeepEqual(t, current(t, alert), []page.Link{{Name: "Alerts", Href: "/admin/alert", Current: true}})
	assert.Equal(t, cancelHref(t, h, "/admin/alert/alt_000"), "/admin/alert")

	assert.Equal(t, get(h, "/admin/alert/nope").Code, http.StatusNotFound) // the Guard: no such alert
}

func TestTheAlertPageLinksToItsDeviceWithAQueryArgument(t *testing.T) {
	p := pageAt(t, handler(t), "/admin/alert/alt_000")
	assert.Equal(t, len(linksTo(p, "/admin/device/dev_27c38b?minutes=15&webui.from=%2Fadmin%2Falert%2Falt_000")), 1)
}

func TestAcknowledgingFromTheAlertPageReturnsToWhereItWasOpenedFrom(t *testing.T) {
	h := handler(t)
	rec := post(h, "/admin/alert/alt_001?webui.from=%2Fadmin%2Falert", url.Values{"_view": {"p.0"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/alert")
	assert.False(t, shown(pageAt(t, h, "/admin/alert"), "alt_001"))
}

func TestActionsRowBulkAndDestructive(t *testing.T) {
	h := handler(t)
	posts := map[string]string{}
	for _, b := range pageAt(t, h, "/admin/alert").ByAttr("name", "_act") {
		posts[b.Name()] = b.Attr("value")
	}
	assert.Equal(t, posts["Acknowledge"], "bulk:0")
	assert.Equal(t, posts["Delete"], "bulk:1") // the destructive one, a button of its own
	assert.True(t, posts["Dismiss alt_000"] == "row:0:alt_000")

	rec := post(h, "/admin/alert", url.Values{"_view": {"p"}, "_act": {"bulk:1"}, "_sel": {"alt_010"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.False(t, shown(pageAt(t, h, "/admin/alert"), "alt_010"))
}

func TestAcknowledgingACriticalAlertWarns(t *testing.T) {
	h := handler(t)
	// alt_003 is critical, alt_005 is info, and alt_007 a warning.
	rec := post(h, "/admin/alert", url.Values{"_view": {"p"}, "_act": {"bulk:0"}, "_sel": {"alt_003", "alt_005"}})
	assert.Equal(t, flashOf(rec), "wAcknowledged 2, including 1 critical")
	rec = post(h, "/admin/alert", url.Values{"_view": {"p"}, "_act": {"bulk:0"}, "_sel": {"alt_007"}}) // a warning
	assert.Equal(t, flashOf(rec), "oAcknowledged 1")
}
