package webui_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// columnsLeftOut is a table of devices whose columns leave themselves out of what a table does with all of them: the
// ID by nothing, the IP of the sort, the occurrences of the filter, the status of both.
func columnsLeftOut() []webui.Accessor[Device] {
	return []webui.Accessor[Device]{
		webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }},
		webui.String[Device]{Label: "IP", NoSort: true, Load: func(d Device) string { return d.Ip }},
		webui.Int[Device]{Label: "Occurrences", NoFilter: true, Load: func(d Device) int { return d.Count }},
		webui.Select[Device]{
			Label: "Status", NoSort: true, NoFilter: true, Options: []webui.Option{{Value: "up"}, {Value: "down"}},
			Load: func(d Device) []string { return []string{map[bool]string{true: "up", false: "down"}[d.Count%2 == 0]} },
		},
	}
}

func leftOutApp(source string) (http.Handler, *webui.Query) {
	var (
		mu   sync.Mutex
		last webui.Query
	)
	devices := []Device{{Id: "b", Ip: "10.0.0.2", Count: 1}, {Id: "a", Ip: "10.0.0.1", Count: 2}, {Id: "c", Ip: "10.0.0.3", Count: 3}}
	table := webui.Table[Device]{Title: "Devices", Columns: columnsLeftOut()}
	if source == "rows" {
		table.Rows = func(context.Context) ([]Device, webui.Loaded) { return devices, webui.Ready() }
	} else {
		table.Load = func(_ context.Context, q webui.Query) (webui.Window[Device], webui.Loaded) {
			mu.Lock()
			last = q
			mu.Unlock()
			return webui.Window[Device]{Items: devices, Total: len(devices)}, webui.Ready()
		}
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/d", Body: table}}}.MustCompile("/admin"), &last
}

func TestAColumnThatLeavesItselfOutHasNoSortLinkAndNoFilter(t *testing.T) {
	t.Parallel()

	h, _ := leftOutApp("load")
	body := serve(h, http.MethodGet, "/admin/d").Body.String()

	// Sort links: ID and Occurrences. Filters: ID and IP. The status has neither.
	assert.Equal(t, strings.Count(body, `aria-sort="none"`), 2)
	assert.Contains(t, body, `href="/admin/d?devices.sort=id"`)
	assert.Contains(t, body, `href="/admin/d?devices.sort=occurrences"`)
	assert.False(t, strings.Contains(body, `devices.sort=ip`))
	assert.False(t, strings.Contains(body, `devices.sort=status`))

	assert.Equal(t, strings.Count(body, `<details class="filter">`), 2)
	assert.Contains(t, body, `aria-label="Filter ID"`)
	assert.Contains(t, body, `aria-label="Filter IP"`)
	assert.False(t, strings.Contains(body, `aria-label="Filter Occurrences"`))
	assert.False(t, strings.Contains(body, `aria-label="Filter Status"`))
	assert.False(t, strings.Contains(body, `devices.min.occurrences`))

	// Its header is only its name.
	assert.Contains(t, body, `<th class=""><span class="th-in">Status </span></th>`)
}

// The address is the visitor's to edit, so a column that is left out is not a way in: what it asks for never reaches
// the source.
func TestAnAddressCannotAskForWhatAColumnLeftOut(t *testing.T) {
	t.Parallel()

	h, last := leftOutApp("load")

	serve(h, http.MethodGet, "/admin/d?devices.sort=ip&devices.desc=true")
	assert.Equal(t, last.Sort, "")
	assert.Equal(t, last.Desc, false)
	serve(h, http.MethodGet, "/admin/d?devices.sort=status")
	assert.Equal(t, last.Sort, "")
	serve(h, http.MethodGet, "/admin/d?devices.sort=occurrences")
	assert.Equal(t, last.Sort, "Occurrences")

	serve(h, http.MethodGet, "/admin/d?devices.min.occurrences=2&devices.max.occurrences=3&devices.filter.status=up")
	assert.Equal(t, len(last.Ranges), 0)
	assert.Equal(t, len(last.Filters), 0)
	serve(h, http.MethodGet, "/admin/d?devices.filter.ip=10.0&devices.filter.id=a")
	assert.DeepEqual(t, last.Filters, map[string][]string{"IP": {"10.0"}, "ID": {"a"}})
}

// A table that holds its rows sorts and filters them itself, and does not for a column that left itself out.
func TestATableThatHoldsItsRowsLeavesOutWhatAColumnLeftOut(t *testing.T) {
	t.Parallel()

	h, _ := leftOutApp("rows")
	rows := func(target string) []string {
		body := serve(h, http.MethodGet, target).Body.String()
		var ids []string
		for _, part := range strings.Split(body, `<tr class=""><td class="">`)[1:] {
			id, _, _ := strings.Cut(part, `</td>`)
			ids = append(ids, id)
		}
		return ids
	}

	assert.DeepEqual(t, rows("/admin/d"), []string{"b", "a", "c"})                          // as the source has them
	assert.DeepEqual(t, rows("/admin/d?devices.sort=id"), []string{"a", "b", "c"})          // a column that sorts
	assert.DeepEqual(t, rows("/admin/d?devices.sort=ip"), []string{"b", "a", "c"})          // one that left the sort out: the source's order
	assert.DeepEqual(t, rows("/admin/d?devices.filter.id=a"), []string{"a"})                // a column that filters
	assert.DeepEqual(t, rows("/admin/d?devices.filter.status=up"), []string{"b", "a", "c"}) // one that left the filter out: every row
	assert.DeepEqual(t, rows("/admin/d?devices.min.occurrences=2"), []string{"b", "a", "c"})
}

// In a form there is no sort and no filter to leave out, so the fields say nothing.
func TestLeavingAColumnOutHasNoEffectInAForm(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Fields = []webui.Accessor[Device]{webui.String[Device]{Label: "IP", NoSort: true, NoFilter: true, Load: func(d Device) string { return d.Ip }, Store: func(d *Device, v string) { d.Ip = v }}}
	body := serve(formApp(f), http.MethodGet, "/admin/device/dev1").Body.String()
	assert.Contains(t, body, `name="f0" type="text" value="10.0.0.1"`)
}
