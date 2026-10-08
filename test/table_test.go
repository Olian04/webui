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
				webui.String[Device]{Label: "ID", Key: "id", Load: func(d Device) string { return d.Id }},
				webui.Badge[Device]{
					Label: "Status",
					Load: func(d Device) string {
						if d.Count%2 == 0 {
							return "healthy"
						}
						return "degraded"
					},
					Kinds: map[string]webui.Tone{"healthy": webui.ToneOK, "degraded": webui.ToneWarning},
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
	assert.DeepEqual(t, *last, webui.Query{Offset: 0, Limit: 10})

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
	assert.DeepEqual(t, *last, webui.Query{Limit: 10, Sort: "id"})
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
	assert.DeepEqual(t, *last, webui.Query{Limit: 10})
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
	assert.Contains(t, rec.Body.String(), `panel-status bad"`)
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
			Columns: []webui.Accessor[Device]{webui.String[Device]{Label: "ID", Key: "id", Load: func(d Device) string { return d.Id }}},
		}
	}
	page := webui.Page[tableArgs]{Path: "/p", Body: webui.Split{table(0, "left"), table(1, "")}}
	return webui.App{Pages: webui.Pages{page}}.MustCompile(""), &seen
}

func TestTwoTablesKeepIndependentStateUnderTheirOwnIDs(t *testing.T) {
	t.Parallel()

	h, seen := twoTables()
	body := serve(h, http.MethodGet, "/p?left.offset=10&table.offset=15&table.sort=id&table.desc=true&q=x").Body.String()

	assert.DeepEqual(t, seen[0], webui.Query{Offset: 10, Limit: 5})
	assert.DeepEqual(t, seen[1], webui.Query{Offset: 15, Limit: 5, Sort: "id", Desc: true})

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
	assert.DeepEqual(t, seen[0], webui.Query{Limit: 5})              // unreadable: the first page
	assert.DeepEqual(t, seen[1], webui.Query{Limit: 5})              // negative: the first page
	assert.False(t, strings.Contains(body, `ghost`+"."+`offset=5"`)) // never carried into a link or form
}

func TestRefreshHoldsTheWholeAddressIncludingViewState(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device?devices.sort=id&devices.offset=10").Body.String()
	// The page's own address, view state included, is what a refresh link holds.
	// Refresh reloads the address the browser is on, exactly as it has it.
	assert.Contains(t, body, `href="/admin/device?devices.sort=id&amp;devices.offset=10" title="Refresh"`)
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

func TestEveryColumnIsSortableAndLoadReceivesItsKeyOrLabel(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()

	// Four columns, four sort links: nothing is declared to make a column sortable.
	assert.Equal(t, strings.Count(body, `aria-sort="none"`), 4)
	assert.Contains(t, body, `href="/admin/device?devices.sort=id"`)          // ID declares Key "id"
	assert.Contains(t, body, `href="/admin/device?devices.sort=Status"`)      // no Key: the Label
	assert.Contains(t, body, `href="/admin/device?devices.sort=Occurrences"`) // numbers too
	assert.Contains(t, body, `href="/admin/device?devices.sort=Rate"`)        // and a slider

	serve(h, http.MethodGet, "/admin/device?devices.sort=Status&devices.desc=true")
	assert.DeepEqual(t, *last, webui.Query{Limit: 10, Sort: "Status", Desc: true})

	serve(h, http.MethodGet, "/admin/device?devices.sort=id")
	assert.Equal(t, last.Sort, "id")

	// A Key replaces the Label: the Label is no longer a way in.
	serve(h, http.MethodGet, "/admin/device?devices.sort=ID")
	assert.Equal(t, last.Sort, "")
}

// With a bulk action the form hides the selection bar until a row is checked.
// :has() lets the form know without script; the bar stays where it is
// unsupported, and the server refuses an empty selection either way.
func TestSelectionBarIsHiddenUntilARowIsSelected(t *testing.T) {
	t.Parallel()

	css := serve(newShop().h, http.MethodGet, "/admin/_webui/app.css").Body.String()
	assert.Contains(t, css, "@supports selector(:has(*)) {\n  form:not(:has(input[name='_sel']:checked)) .actionbar {\n    display: none;")
	assert.Contains(t, css, "html:not(.js) .actionbar [data-selcount]") // no live count without script
}

func TestEveryColumnHasAFilterAFixedSetOfOptionsIsAMultiSelect(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()

	// One filter per column, in the header, none active.
	assert.Equal(t, strings.Count(body, `<details class="filter">`), 4)
	assert.Equal(t, strings.Count(body, `<details class="filter on">`), 0)
	assert.Contains(t, body, `aria-label="Filter Status"`)

	// The Badge's Kinds are the options: a multi-select of exactly those, in order.
	status := body[strings.Index(body, `<div class="filter-title">Status</div>`):]
	status = status[:strings.Index(status, `</form>`)]
	assert.Equal(t, strings.Count(status, `type="checkbox"`), 2)
	assert.True(t, strings.Index(status, `value="degraded"`) < strings.Index(status, `value="healthy"`))
	assert.False(t, strings.Contains(status, `type="search"`))

	// A text column takes text; the two numeric ones, Occurrences and Rate, take a
	// minimum and a maximum instead, never text.
	assert.Equal(t, strings.Count(body, `type="search"`), 1)
	assert.Contains(t, body, `placeholder="Contains…"`)
	assert.Equal(t, strings.Count(body, `type="number" step="any"`), 4)
	assert.Contains(t, body, `name="devices.min.Occurrences"`)
	assert.Contains(t, body, `name="devices.max.Occurrences"`)
	assert.Contains(t, body, `name="devices.min.Rate"`)
	assert.False(t, strings.Contains(body, `name="devices.filter.Occurrences"`))
	assert.False(t, strings.Contains(body, `name="devices.filter.Rate"`))

	// The form is a plain GET back to the page, so it works without script.
	assert.Contains(t, body, `<form class="filter-pop" method="get" action="/admin/device" data-filter-form>`)
}

func TestFiltersAreKeyedByTheAccessorsKeyElseItsLabel(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)

	// ID declares Key "id"; Status and Occurrences have none, so their Label is the key.
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, body, `name="devices.filter.id"`)
	assert.Contains(t, body, `name="devices.filter.Status"`)
	assert.Contains(t, body, `name="devices.min.Occurrences"`)          // numeric: bounds, keyed by Label
	assert.False(t, strings.Contains(body, `name="devices.filter.ID"`)) // the Label is no way in once there is a Key

	serve(h, http.MethodGet, "/admin/device?devices.filter.id=dev01&devices.filter.Status=healthy")
	assert.DeepEqual(t, last.Filters, map[string][]string{"id": {"dev01"}, "Status": {"healthy"}})

	serve(h, http.MethodGet, "/admin/device?devices.filter.ID=dev01")
	assert.True(t, last.Filters == nil)
}

func TestLoadReceivesOnlyFiltersThatMakeSense(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)

	serve(h, http.MethodGet, "/admin/device?devices.filter.Status=healthy&devices.filter.Status=degraded&devices.filter.Status=healthy")
	assert.DeepEqual(t, last.Filters, map[string][]string{"Status": {"healthy", "degraded"}}) // in order, once each

	serve(h, http.MethodGet, "/admin/device?devices.filter.Status=bogus&devices.filter.Status=healthy")
	assert.DeepEqual(t, last.Filters, map[string][]string{"Status": {"healthy"}}) // not an option: dropped

	serve(h, http.MethodGet, "/admin/device?devices.filter.Status=bogus")
	assert.True(t, last.Filters == nil)

	serve(h, http.MethodGet, "/admin/device?devices.filter.id=%20%20dev01%20&devices.filter.Status=")
	assert.DeepEqual(t, last.Filters, map[string][]string{"id": {"dev01"}}) // trimmed; empty is no filter

	serve(h, http.MethodGet, "/admin/device?devices.filter.id="+strings.Repeat("x", 500))
	assert.Equal(t, len(last.Filters["id"][0]), 200) // typed by a person: bounded

	serve(h, http.MethodGet, "/admin/device?devices.filter.nope=1")
	assert.True(t, last.Filters == nil) // a column the table does not have
}

func TestAnActiveFilterShowsAndCanBeCleared(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device?devices.offset=10&devices.sort=id&devices.filter.id=dev0&devices.filter.Status=healthy").Body.String()

	assert.Equal(t, strings.Count(body, `<details class="filter on">`), 2)
	assert.Contains(t, body, `value="dev0"`) // the text, as typed
	assert.Contains(t, body, `type="checkbox" name="devices.filter.Status" value="healthy" checked>`)
	assert.False(t, strings.Contains(body, `value="degraded" checked`))

	// Applying one column's filter carries everything else and drops every offset;
	// the filter being replaced is not carried.
	id := body[strings.Index(body, `aria-label="Filter ID"><i class="fa-solid fa-filter"`):]
	id = id[:strings.Index(id, `<div class="filter-title">ID</div>`)]
	assert.Contains(t, id, `name="devices.sort" value="id"`)
	assert.Contains(t, id, `name="devices.filter.Status" value="healthy"`)
	assert.False(t, strings.Contains(id, `devices.offset`))
	assert.False(t, strings.Contains(id, `name="devices.filter.id"`))

	// Clear removes that filter, and the offset, and keeps the rest.
	assert.Contains(t, body, `href="/admin/device?devices.filter.Status=healthy&amp;devices.sort=id">`)
}

func TestSortAndPagerLinksKeepFiltersAsRepeatedParameters(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device?devices.filter.Status=healthy&devices.filter.Status=degraded").Body.String()
	assert.Contains(t, body, `href="/admin/device?devices.filter.Status=healthy&amp;devices.filter.Status=degraded&amp;devices.sort=id"`)
	assert.Contains(t, body, `href="/admin/device?devices.filter.Status=healthy&amp;devices.filter.Status=degraded&amp;devices.offset=10"`)
}

func TestFilterIconAppearsOnHoverAndStaysWhenActive(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	css := serve(h, http.MethodGet, "/admin/_webui/app.css").Body.String()
	assert.Contains(t, css, ".filter-btn {\n  list-style: none;")
	assert.Contains(t, css, "  opacity: 0;\n  transition: opacity 0.12s;\n}\n.filter-btn::-webkit-details-marker")
	assert.Contains(t, css, "th:hover .filter-btn,\nth:focus-within .filter-btn,\n.filter[open] .filter-btn,\n.filter.on .filter-btn {\n  opacity: 1;")
}

func TestNumericColumnsFilterByRangeNotText(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)
	f := func(v float64) *float64 { return &v }

	// Either bound, both, or neither; the key is the Label, as these have no Key.
	serve(h, http.MethodGet, "/admin/device?devices.min.Occurrences=5")
	assert.DeepEqual(t, last.Ranges, map[string]webui.Range{"Occurrences": {Min: f(5)}})
	assert.True(t, last.Filters == nil)

	serve(h, http.MethodGet, "/admin/device?devices.max.Occurrences=10.5&devices.min.Rate=-2")
	assert.DeepEqual(t, last.Ranges, map[string]webui.Range{"Occurrences": {Max: f(10.5)}, "Rate": {Min: f(-2)}})

	serve(h, http.MethodGet, "/admin/device?devices.min.Occurrences=5&devices.max.Occurrences=5")
	assert.DeepEqual(t, last.Ranges, map[string]webui.Range{"Occurrences": {Min: f(5), Max: f(5)}}) // equal bounds: exactly 5

	// A bound that is not a finite number never arrives; neither does text for a number column.
	for _, bad := range []string{"abc", "NaN", "Inf", "-Inf", "1e999", ""} {
		serve(h, http.MethodGet, "/admin/device?devices.min.Occurrences="+bad+"&devices.max.Rate="+bad)
		assert.True(t, last.Ranges == nil)
	}
	serve(h, http.MethodGet, "/admin/device?devices.filter.Occurrences=5&devices.filter.Rate=5")
	assert.True(t, last.Filters == nil)
	assert.True(t, last.Ranges == nil)
}

func TestAnActiveRangeShowsItsBoundsAndClearsBoth(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device?devices.min.Occurrences=5&devices.max.Occurrences=9.5&devices.sort=id&devices.offset=10").Body.String()

	assert.Equal(t, strings.Count(body, `<details class="filter on">`), 1)
	assert.Contains(t, body, `name="devices.min.Occurrences" value="5"`)
	assert.Contains(t, body, `name="devices.max.Occurrences" value="9.5"`)

	// Applying another column keeps these bounds; applying this one replaces them.
	idForm := body[strings.Index(body, `aria-label="Filter ID"><i class="fa-solid fa-filter"`):]
	idForm = idForm[:strings.Index(idForm, `<div class="filter-title">ID</div>`)]
	assert.Contains(t, idForm, `name="devices.min.Occurrences" value="5"`)
	assert.Contains(t, idForm, `name="devices.max.Occurrences" value="9.5"`)

	own := body[strings.Index(body, `aria-label="Filter Occurrences"><i class="fa-solid fa-filter"`):]
	own = own[:strings.Index(own, `<div class="filter-title">Occurrences</div>`)]
	assert.False(t, strings.Contains(own, `devices.min.Occurrences`))
	assert.False(t, strings.Contains(own, `devices.max.Occurrences`))
	assert.False(t, strings.Contains(own, `devices.offset`))
	assert.Contains(t, own, `name="devices.sort" value="id"`)

	// Clear drops both bounds and the offset, and keeps the sort.
	assert.Contains(t, body, `href="/admin/device?devices.sort=id">Clear`)
}
