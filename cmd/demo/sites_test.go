package main

import (
	"net/http"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func TestSiteDetailHasTwoTablesWithTheirOwnState(t *testing.T) {
	h := handler(t)
	p := pageAt(t, h, "/admin/site/Stockholm?devices.sort=id&open-alerts.sort=severity&open-alerts.desc=true")
	assert.DeepEqual(t, titles(p), []string{"Devices", "Open alerts"})
	sorted := func(table, column string) string {
		return page.One(t, column, page.One(t, table, p.ByRole("table", table)).ByRole("columnheader", column)).Attr("aria-sort")
	}
	assert.Equal(t, sorted("Devices", "ID"), "ascending")            // devices, by its sort
	assert.Equal(t, sorted("Open alerts", "Severity"), "descending") // alerts, by its own
	assert.Equal(t, sorted("Open alerts", "ID"), "none")
	for _, name := range []string{"devices.filter.id", "open-alerts.filter.device"} {
		_, ok := p.Field(name)
		assert.True(t, ok)
	}

	rec := get(h, "/admin/site/Nowhere")
	assert.Equal(t, rec.Code, http.StatusNotFound)
	assert.True(t, shown(page.Parse(t, rec.Body.String()), `There is no site "Nowhere".`))
}
