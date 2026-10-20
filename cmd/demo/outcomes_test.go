package main

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestAnOutcomeCanSayWhereToGoWhateverItsKind(t *testing.T) {
	h := handler(t)

	// Stay: the ingest form is one the user goes on tuning, so a save that was reached
	// from a link still stays on it, with where it came from kept in the address.
	own := "/admin/settings?webui.from=" + url.QueryEscape("/admin/system")
	rec := post(h, own, url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), own)

	// Back: someone else acknowledged the alert first. A Failure would show the form again,
	// for an alert with nothing left to do; Back leaves for the page the user came from, with
	// the reason in a toast, and for the parent when it was opened directly.
	from := "?webui.from=" + url.QueryEscape("/admin/alert")
	post(h, "/admin/alert/alt_001"+from, url.Values{"_leaf": {"p.0"}})
	rec = post(h, "/admin/alert/alt_001"+from, url.Values{"_leaf": {"p.0"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/alert")
	rec = post(h, "/admin/alert/alt_001", url.Values{"_leaf": {"p.0"}})
	assert.Equal(t, rec.Header().Get("Location"), "/admin/alert")
}
