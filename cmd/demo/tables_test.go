package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func TestAFeedPagesByACursorAndSortsOnlyThePageInHandWithNoFilters(t *testing.T) {
	h := handler(t)
	p := pageAt(t, h, "/admin/audit")
	assert.True(t, shown(p, "Showing rows 1–8")) // no total, but which rows
	assert.True(t, hasLink(p, "Next", "/admin/audit?audit-log.after=8&audit-log.back=1%3A&audit-log.row=9"))
	assert.True(t, page.One(t, "First page", p.ByRole("button", "First page")).Has("disabled"))
	assert.Equal(t, len(p.ByAttr("aria-sort", "ascending"))+len(p.ByAttr("aria-sort", "descending")), 0) // the source's order, until a column is chosen
	assert.Equal(t, len(p.ByRoleContaining("button", "Filter")), 0)
	sortable := false
	for _, l := range p.Links() {
		sortable = sortable || strings.Contains(l.Href, "audit-log.sort=")
	}
	assert.True(t, sortable) // a column can be chosen: it orders the page in hand
	assert.True(t, shown(pageAt(t, h, "/admin/audit?audit-log.sort=who"), "sorted within this page"))

	// The cursor comes back, and the last page has no Next.
	last := pageAt(t, h, "/admin/audit?audit-log.after=40")
	assert.True(t, hasLink(last, "First page", "/admin/audit"))
	assert.True(t, page.One(t, "Next", last.ByRole("button", "Next")).Has("disabled"))

	// Actions are recorded: the newest entry is the one just made.
	post(h, "/admin/retention", url.Values{"_view": {"p"}, "f0": {"45"}})
	assert.True(t, shown(pageAt(t, h, "/admin/audit"), "saved retention"))
}

func TestTablesExportTheirRowsWhateverTheSource(t *testing.T) {
	h := handler(t)

	// Load: the device table is 10 to a page, and the export is every device the filter keeps.
	rec := get(h, "/admin/device?devices.export=csv&devices.sort=id&devices.desc=true&devices.offset=10")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, rec.Header().Get("Content-Disposition"), "attachment; filename=devices.csv")
	assert.Equal(t, strings.Count(rec.Body.String(), "\n"), 38) // the header and 37 devices, and a line ends each

	// Rows: the open alerts, as JSON.
	var alerts []map[string]any
	assert.NoError(t, json.Unmarshal(get(h, "/admin/alert?alerts.export=json").Body.Bytes(), &alerts))
	assert.True(t, len(alerts) > 0)

	// Feed: the whole audit log, not the page of eight.
	audit := get(h, "/admin/audit?audit-log.export=csv")
	assert.Equal(t, audit.Header().Get("Content-Disposition"), "attachment; filename=audit-log.csv")
	assert.True(t, strings.Count(audit.Body.String(), "\n") > 9)

	// The viewer who may not see the audit log may not download it.
	assert.Equal(t, get(handlerAs(t, Viewer), "/admin/audit?audit-log.export=csv").Code, http.StatusForbidden)
}
