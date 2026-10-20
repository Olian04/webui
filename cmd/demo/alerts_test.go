package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestAnAlertHasItsOwnPageAndItsRowsOpenIt(t *testing.T) {
	h := handler(t)
	assert.Contains(t, get(h, "/admin/alert").Body.String(), `href="/admin/alert/alt_000?webui.from=%2Fadmin%2Falert"`)
	assert.Contains(t, get(h, "/admin/site/Stockholm").Body.String(), `href="/admin/alert/alt_000?webui.from=%2Fadmin%2Fsite%2FStockholm"`)

	// It shows the alert, lights Alerts in the sidebar, and cancels to the list.
	page := get(h, "/admin/alert/alt_000").Body.String()
	assert.Contains(t, page, "Ingest lag above 5s")
	assert.Contains(t, page, `class="nav-item active" href="/admin/alert"`)
	assert.Equal(t, cancelHref(t, h, "/admin/alert/alt_000"), "/admin/alert")

	assert.Equal(t, get(h, "/admin/alert/nope").Code, http.StatusNotFound) // the Guard: no such alert
}

func TestTheAlertPageLinksToItsDeviceWithAQueryArgument(t *testing.T) {
	body := get(handler(t), "/admin/alert/alt_000").Body.String()
	assert.Contains(t, body, `href="/admin/device/dev_27c38b?minutes=15&amp;webui.from=%2Fadmin%2Falert%2Falt_000"`)
}

func TestAcknowledgingFromTheAlertPageReturnsToWhereItWasOpenedFrom(t *testing.T) {
	h := handler(t)
	rec := post(h, "/admin/alert/alt_001?webui.from=%2Fadmin%2Falert", url.Values{"_leaf": {"p.0"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/alert")
	assert.False(t, strings.Contains(get(h, "/admin/alert").Body.String(), "alt_001"))
}

func TestActionsRowBulkAndDestructive(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/alert").Body.String()
	assert.Contains(t, body, `name="_act" value="bulk:0">Acknowledge</button>`)
	assert.Contains(t, body, `class="btn btn-danger btn-sm"`) // the destructive role
	assert.Contains(t, body, `value="bulk:1">Delete</button>`)
	assert.Contains(t, body, `name="_act" value="row:0:alt_000"`)

	rec := post(h, "/admin/alert", url.Values{"_leaf": {"p"}, "_act": {"bulk:1"}, "_sel": {"alt_010"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.False(t, strings.Contains(get(h, "/admin/alert").Body.String(), "alt_010"))
}

func TestAcknowledgingACriticalAlertWarns(t *testing.T) {
	h := handler(t)
	// alt_003 is critical, alt_005 is info, and alt_007 a warning.
	rec := post(h, "/admin/alert", url.Values{"_leaf": {"p"}, "_act": {"bulk:0"}, "_sel": {"alt_003", "alt_005"}})
	assert.Equal(t, flashOf(rec), "wAcknowledged 2, including 1 critical")
	rec = post(h, "/admin/alert", url.Values{"_leaf": {"p"}, "_act": {"bulk:0"}, "_sel": {"alt_007"}}) // a warning
	assert.Equal(t, flashOf(rec), "oAcknowledged 1")
}
