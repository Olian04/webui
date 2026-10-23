package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestAFeedPagesByACursorAndSortsOnlyThePageInHandWithNoFilters(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/audit").Body.String()
	assert.Contains(t, body, "Showing rows 1–8") // no total, but which rows
	assert.Contains(t, body, `href="/admin/audit?audit-log.after=8&amp;audit-log.back=1%3A&amp;audit-log.row=9" data-in-place>Next`)
	assert.Contains(t, body, `disabled data-in-place>First page</button>`)
	assert.False(t, strings.Contains(body, `aria-sort="ascending"`)) // the source's order, until a column is chosen
	assert.False(t, strings.Contains(body, `aria-sort="descending"`))
	assert.False(t, strings.Contains(body, `class="filter`))
	assert.Contains(t, body, `audit-log.sort=`) // a column can be chosen: it orders the page in hand
	assert.Contains(t, get(h, "/admin/audit?audit-log.sort=who").Body.String(), "sorted within this page")

	// The cursor comes back, and the last page has no Next.
	last := get(h, "/admin/audit?audit-log.after=40").Body.String()
	assert.Contains(t, last, `href="/admin/audit" data-in-place>First page`)
	assert.Contains(t, last, `disabled data-in-place>Next</button>`)

	// Actions are recorded: the newest entry is the one just made.
	post(h, "/admin/retention", url.Values{"_leaf": {"p"}, "f0": {"45"}})
	assert.Contains(t, get(h, "/admin/audit").Body.String(), "saved retention")
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
