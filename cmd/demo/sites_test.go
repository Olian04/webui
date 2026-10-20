package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestSiteDetailHasTwoTablesWithTheirOwnState(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/site/Stockholm?devices.sort=id&open-alerts.sort=severity&open-alerts.desc=true").Body.String()
	assert.Equal(t, strings.Count(body, `class="panel grows"`), 2)
	assert.Contains(t, body, `aria-sort="ascending"`)  // devices, by its sort
	assert.Contains(t, body, `aria-sort="descending"`) // alerts, by its own
	assert.Contains(t, body, `name="devices.filter.id"`)
	assert.Contains(t, body, `name="open-alerts.filter.device"`)

	rec := get(h, "/admin/site/Nowhere")
	assert.Equal(t, rec.Code, http.StatusNotFound)
	assert.Contains(t, rec.Body.String(), `There is no site &#34;Nowhere&#34;.`)
}
