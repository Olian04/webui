package webui_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type tableArgs struct {
	Q string
}

type detailsArgs struct{ Id string }

// tableApp serves 25 devices, ten to a page, and records the Query the loader
// was handed.
func tableApp(load func(context.Context, webui.Query) (webui.Rows[Device], error)) (http.Handler, *webui.Query) {
	var (
		mu   sync.Mutex
		last webui.Query
	)
	details := webui.Page[detailsArgs]{Path: "/device/{id}", Body: webui.Stack{}}
	list := webui.Page[tableArgs]{
		Path: "/device",
		Nav:  webui.Nav{Label: "Devices"},
		Body: webui.Table[Device]{
			Title: "Devices",
			Load: func(ctx context.Context, q webui.Query) (webui.Rows[Device], error) {
				mu.Lock()
				last = q
				mu.Unlock()
				if load != nil {
					return load(ctx, q)
				}
				var all []Device
				for i := range 25 {
					all = append(all, Device{Id: fmt.Sprintf("dev%02d", i), Ip: "10.0.0.1", Count: i, Duration: 1})
				}
				end := min(q.Offset+q.Limit, len(all))
				return webui.Rows[Device]{Items: all[q.Offset:end], Total: len(all)}, nil
			},
			ID:       "devices",
			PageSize: 10,
			RowClick: webui.Link[Device, detailsArgs]{
				Page: details,
				Args: func(_ context.Context, d Device) detailsArgs { return detailsArgs{Id: d.Id} },
			},
			Columns: []webui.Accessor[Device]{
				webui.Sortable[Device]{Accessor: webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}, Key: "id"},
				webui.Badge[Device]{
					Label: "Status",
					Load: func(d Device) string {
						if d.Count%2 == 0 {
							return "healthy"
						}
						return "degraded"
					},
					Tones: map[string]webui.Tone{"healthy": webui.ToneOK, "degraded": webui.ToneWarning},
				},
				webui.Int[Device]{Label: "Occurrences", Load: func(d Device) int { return d.Count }},
				webui.Slider[Device]{Label: "Rate", Max: 4, Precision: 2, Load: func(d Device) float64 { return float64(d.Count) / 3 }},
			},
		},
	}
	h := webui.App{Pages: webui.Pages{list, details}}.MustCompile("/admin")
	return h, &last
}

func TestTableRendersRowsLinksAndCells(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()

	assert.Equal(t, strings.Count(body, `<tr class="clickable"`), 10)
	assert.Contains(t, body, `<a class="rowlink" href="/admin/device/dev00">dev00</a>`)
	assert.Equal(t, strings.Count(body, `class="rowlink"`), 10) // one anchor per row
	assert.Contains(t, body, `<td class="num">3</td>`)          // numbers align to the end
	assert.Contains(t, body, `class="badge badge-ok"`)
	assert.Contains(t, body, `class="badge badge-warn"`)
	assert.Contains(t, body, `<span>1.33</span>`) // Precision
	assert.Contains(t, body, `class="gauge-fill"`)
	assert.Contains(t, body, `<h2 class="panel-title">Devices</h2>`)
	assert.Contains(t, body, `data-leaf="p"`)
}

func TestTablePagerLinksAndRange(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)

	first := serve(h, http.MethodGet, "/admin/device?q=x").Body.String()
	assert.Contains(t, first, "1–10 of 25")
	assert.Contains(t, first, `href="/admin/device?devices.offset=10&amp;q=x"`) // keeps the filter
	assert.Equal(t, *last, webui.Query{Offset: 0, Limit: 10})

	mid := serve(h, http.MethodGet, "/admin/device?devices.offset=10").Body.String()
	assert.Contains(t, mid, "11–20 of 25")
	assert.Contains(t, mid, `href="/admin/device">Previous</a>`) // the first page carries no offset

	lastPage := serve(h, http.MethodGet, "/admin/device?devices.offset=20").Body.String()
	assert.Contains(t, lastPage, "21–25 of 25")
	assert.Contains(t, lastPage, `disabled>Next</button>`)
	assert.Contains(t, lastPage, `href="/admin/device?devices.offset=10">Previous</a>`)
	assert.Equal(t, last.Offset, 20)
}

func TestTableSortLinksCycleAndKeepWindowOut(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)

	unsorted := serve(h, http.MethodGet, "/admin/device?devices.offset=10").Body.String()
	assert.Contains(t, unsorted, `href="/admin/device?devices.sort=id"`) // sorting resets to the first page

	asc := serve(h, http.MethodGet, "/admin/device?devices.sort=id").Body.String()
	assert.Equal(t, *last, webui.Query{Limit: 10, Sort: "id"})
	assert.Contains(t, asc, `aria-sort="ascending"`)
	assert.Contains(t, asc, `href="/admin/device?devices.desc=true&amp;devices.sort=id"`)

	desc := serve(h, http.MethodGet, "/admin/device?devices.sort=id&devices.desc=true").Body.String()
	assert.Equal(t, last.Desc, true)
	assert.Contains(t, desc, `aria-sort="descending"`)
	assert.Contains(t, desc, `href="/admin/device?devices.sort=id"`)
}

func TestTableDropsUndeclaredSortKeys(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)
	serve(h, http.MethodGet, "/admin/device?devices.sort=password%3B+drop&devices.desc=true")
	assert.Equal(t, *last, webui.Query{Limit: 10})
}

func TestTableWithUnknownTotalPagesByFullPage(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(func(_ context.Context, q webui.Query) (webui.Rows[Device], error) {
		items := make([]Device, 10)
		for i := range items {
			items[i] = Device{Id: fmt.Sprintf("d%d", q.Offset+i)}
		}
		return webui.Rows[Device]{Items: items}, nil // Total left zero: unknown
	})
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, body, "1–10")
	assert.False(t, strings.Contains(body, "of 25"))
	assert.Contains(t, body, `href="/admin/device?devices.offset=10"`) // a full page may have a successor
}

func TestTableLoadFailureFailsThePanelNotThePage(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(func(context.Context, webui.Query) (webui.Rows[Device], error) {
		return webui.Rows[Device]{}, errors.New("db password is hunter2")
	})
	rec := serve(h, http.MethodGet, "/admin/device")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), "Could not load")
	assert.Contains(t, rec.Body.String(), `class="panel-status bad"`)
	assert.False(t, strings.Contains(rec.Body.String(), "hunter2")) // the cause is logged, never shown
}

func TestTableEmpty(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(func(context.Context, webui.Query) (webui.Rows[Device], error) { return webui.Rows[Device]{}, nil })
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, body, "Nothing to show")
	assert.Contains(t, body, "No results")
	assert.Contains(t, body, `colspan="4"`)
}

// twoTables is a page with two paging tables, one named and one left to default.
func twoTables() (http.Handler, *[2]webui.Query) {
	var seen [2]webui.Query
	table := func(i int, id string) webui.Table[Device] {
		return webui.Table[Device]{
			ID: id, Title: fmt.Sprintf("T%d", i), PageSize: 5,
			Load: func(_ context.Context, q webui.Query) (webui.Rows[Device], error) {
				seen[i] = q
				return webui.Rows[Device]{Items: []Device{{Id: "row"}}, Total: 50}, nil
			},
			Columns: []webui.Accessor[Device]{webui.Sortable[Device]{Accessor: formID, Key: "id"}},
		}
	}
	page := webui.Page[tableArgs]{Path: "/p", Body: webui.Split{table(0, "left"), table(1, "")}}
	return webui.App{Pages: webui.Pages{page}}.MustCompile(""), &seen
}

func TestTwoTablesKeepIndependentStateUnderTheirOwnIDs(t *testing.T) {
	t.Parallel()

	h, seen := twoTables()
	body := serve(h, http.MethodGet, "/p?left.offset=10&table.offset=15&table.sort=id&table.desc=true&q=x").Body.String()

	assert.Equal(t, seen[0], webui.Query{Offset: 10, Limit: 5})
	assert.Equal(t, seen[1], webui.Query{Offset: 15, Limit: 5, Sort: "id", Desc: true})

	// Each table's links change only its own state and carry the other's along.
	assert.Contains(t, body, `href="/p?left.offset=15&amp;q=x&amp;table.desc=true&amp;table.offset=15&amp;table.sort=id">Next`)
	assert.Contains(t, body, `href="/p?left.offset=10&amp;q=x&amp;table.desc=true&amp;table.offset=20&amp;table.sort=id">Next`)
	// Sorting one table returns only that table to its first page, and a column
	// already sorted descending sorts ascending again.
	assert.Contains(t, body, `href="/p?left.sort=id&amp;q=x&amp;table.desc=true&amp;table.offset=15&amp;table.sort=id">ID`)
	assert.Contains(t, body, `href="/p?left.offset=10&amp;q=x&amp;table.sort=id">ID`)
}

func TestViewStateForALeafThePageDoesNotHaveIsIgnored(t *testing.T) {
	t.Parallel()

	h, seen := twoTables()
	body := serve(h, http.MethodGet, "/p?ghost.offset=5&left.offset=abc&table.offset=-4").Body.String()
	assert.Equal(t, seen[0], webui.Query{Limit: 5})                  // unreadable: the first page
	assert.Equal(t, seen[1], webui.Query{Limit: 5})                  // negative: the first page
	assert.False(t, strings.Contains(body, `ghost`+"."+`offset=5"`)) // never carried into a link or form
}

func TestToolbarCarriesViewStateButResetsPagingAndKeepsSortOnClearAll(t *testing.T) {
	t.Parallel()

	h, _ := twoTables()
	body := serve(h, http.MethodGet, "/p?q=x&left.offset=10&left.sort=id&table.offset=5").Body.String()

	// Setting the filter keeps sort, and drops every offset: the old one may not exist.
	assert.Contains(t, body, `<input type="hidden" name="left.sort" value="id">`)
	assert.False(t, strings.Contains(body, `name="left.offset"`))
	assert.False(t, strings.Contains(body, `name="table.offset"`))

	// Clear all removes the filter and paging, but not how the page is sorted.
	assert.Contains(t, body, `<a class="tb-btn " href="/p?left.sort=id">Clear all</a>`)
	// Clear one filter is the same address with that argument gone.
	assert.Contains(t, body, `href="/p?left.sort=id"`)
}

func TestRefreshHoldsTheWholeAddressIncludingViewState(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device?devices.sort=id&devices.offset=10").Body.String()
	// The page's own address, view state included, is what a refresh link holds.
	assert.Contains(t, body, `href="/admin/device?devices.offset=10&amp;devices.sort=id" data-refresh`)
}

// A clickable row is one link stretched over the whole row by the stylesheet;
// without these rules only the link's own text is clickable.
func TestStylesheetStretchesTheRowLinkOverTheRow(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	css := serve(h, http.MethodGet, "/admin/_webui/app.css").Body.String()
	assert.Contains(t, css, "tbody tr.clickable {\n  cursor: pointer;\n  position: relative;")
	assert.Contains(t, css, ".rowlink::after {\n  content: '';\n  position: absolute;\n  inset: 0;")
}
