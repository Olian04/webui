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
	"github.com/Olian04/webui/test/util/page"
)

type tableArgs struct {
	Q string
}

type detailsArgs struct{ Id string }

// tableApp serves 25 devices, ten to a page, and records the Query the loader
// was handed.
func tableApp(load func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded)) (http.Handler, *webui.PagedQuery) {
	var (
		mu   sync.Mutex
		last webui.PagedQuery
	)
	details := webui.Page[detailsArgs]{Path: "/device/{id}", View: webui.Stack{}}
	list := webui.Page[tableArgs]{
		Path: "/device",
		Nav:  webui.Nav{Label: "Devices"},
		View: webui.Table[Device]{
			Title: "Devices",
			Loader: webui.PagedLoader[Device]{Load: func(ctx context.Context, q webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
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
				return webui.PagedWindow[Device]{Items: all[q.Offset:end], Total: len(all)}, webui.Ready()
			}},
			PageSize: 10,
			RowClick: webui.Link[Device, detailsArgs]{
				Page: details,
				Args: func(_ context.Context, d Device) detailsArgs { return detailsArgs{Id: d.Id} },
			},
			Columns: []webui.Accessor[Device]{
				webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }},
				webui.Select[Device]{
					Label: "Status",
					Load: func(d Device) []string {
						if d.Count%2 == 0 {
							return []string{"healthy"}
						}
						return []string{"degraded"}
					},
					Options: []webui.Option{{Value: "healthy", Tone: webui.ToneOK}, {Value: "degraded", Tone: webui.ToneWarning}},
				},
				webui.Int[Device]{Label: "Occurrences", Load: func(d Device) int { return d.Count }},
				webui.Slider[Device]{Label: "Rate", Max: 4, StepSize: 0.01, Load: func(d Device) float64 { return float64(d.Count) / 3 }},
			},
		},
	}
	h := webui.App{Pages: webui.Pages{list, details}}.MustCompile("/admin")
	return h, &last
}

func TestTableRendersRowsLinksAndCells(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	p := pageOf(t, h, "/admin/device")
	table := page.One(t, "the Devices table", p.ByRole("table", "Devices"))

	rows := table.Rows()
	assert.Equal(t, len(rows), 10)
	assert.DeepEqual(t, rows[4], []string{"dev04", "healthy", "4", "1.33"}) // an Int as it is, a Select as its value, a Slider to its Precision
	assert.Equal(t, rows[3][1], "degraded")
	assert.True(t, hasLink(p, "dev00", "/admin/device/dev00"))
	leading := 0 // one link to a page for each row
	for _, l := range table.Links() {
		if strings.HasPrefix(l.Href, "/admin/device/dev") {
			leading++
		}
	}
	assert.Equal(t, leading, 10)
	panels := 0 // the panel's own heading, under the page's
	for _, h := range p.ByRole("heading", "Devices") {
		if h.Tag() == "h2" {
			panels++
		}
	}
	assert.Equal(t, panels, 1)
	assert.Equal(t, page.One(t, "the panel", p.Views()).Attr("data-view"), "p")
}

func TestTablePagerLinksAndRange(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)

	first := pageOf(t, h, "/admin/device?q=x")
	assert.Contains(t, first.Root().Text(), "1–10 of 25")
	assert.True(t, hasLink(first, "Next", "/admin/device?devices.offset=10&q=x")) // keeps the filter
	assert.DeepEqual(t, *last, webui.PagedQuery{Offset: 0, Limit: 10})
	assert.True(t, page.One(t, "the Next link", first.ByRole("link", "Next")).Has("data-in-place")) // script puts the panel in place

	mid := pageOf(t, h, "/admin/device?devices.offset=10")
	assert.Contains(t, mid.Root().Text(), "11–20 of 25")
	assert.True(t, hasLink(mid, "Previous", "/admin/device")) // the first page carries no offset

	lastPage := pageOf(t, h, "/admin/device?devices.offset=20")
	assert.Contains(t, lastPage.Root().Text(), "21–25 of 25")
	assert.True(t, page.One(t, "the Next button", lastPage.ByRole("button", "Next")).Has("disabled"))
	assert.True(t, hasLink(lastPage, "Previous", "/admin/device?devices.offset=10"))
	assert.Equal(t, last.Offset, 20)
}

func TestTableSortLinksCycleAndKeepWindowOut(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)

	unsorted := pageOf(t, h, "/admin/device?devices.offset=10")
	assert.True(t, hasLink(unsorted, "ID", "/admin/device?devices.sort=id")) // sorting resets to the first page

	asc := pageOf(t, h, "/admin/device?devices.sort=id")
	assert.DeepEqual(t, *last, webui.PagedQuery{Limit: 10, Sort: "ID"}) // Load knows the column by its Label
	assert.Equal(t, column(t, asc, "Devices", "ID").Attr("aria-sort"), "ascending")
	assert.True(t, hasLink(asc, "ID", "/admin/device?devices.desc=true&devices.sort=id"))

	desc := pageOf(t, h, "/admin/device?devices.sort=id&devices.desc=true")
	assert.Equal(t, last.Desc, true)
	assert.Equal(t, column(t, desc, "Devices", "ID").Attr("aria-sort"), "descending")
	assert.True(t, hasLink(desc, "ID", "/admin/device?devices.sort=id"))
}

func TestTableDropsUndeclaredSortKeys(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)
	serve(h, http.MethodGet, "/admin/device?devices.sort=password%3B+drop&devices.desc=true")
	assert.DeepEqual(t, *last, webui.PagedQuery{Limit: 10})
}

func TestTableWithUnknownTotalPagesByFullPage(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(func(_ context.Context, q webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
		items := make([]Device, 10)
		for i := range items {
			items[i] = Device{Id: fmt.Sprintf("d%d", q.Offset+i)}
		}
		return webui.PagedWindow[Device]{Items: items}, webui.Ready() // Total left zero: unknown
	})
	p := pageOf(t, h, "/admin/device")
	assert.Contains(t, p.Root().Text(), "1–10")
	assert.False(t, strings.Contains(p.Root().Text(), "of 25"))
	assert.True(t, hasLink(p, "Next", "/admin/device?devices.offset=10")) // a full page may have a successor
}

func TestTableLoadFailureFailsThePanelNotThePage(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
		return webui.PagedWindow[Device]{}, webui.Failed(errors.New("db password is hunter2"))
	})
	rec := serve(h, http.MethodGet, "/admin/device")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, page.Parse(t, rec.Body.String()).Root().Text(), "Could not load")
	assert.False(t, strings.Contains(rec.Body.String(), "hunter2")) // the cause is logged, never shown
}

func TestTableEmpty(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
		return webui.PagedWindow[Device]{}, webui.Ready()
	})
	text := pageOf(t, h, "/admin/device").Root().Text()
	assert.Contains(t, text, "Nothing to show")
	assert.Contains(t, text, "No results")
}

// twoTables is a page with two paging tables, named by their titles T0 and T1.
func twoTables() (http.Handler, *[2]webui.PagedQuery) {
	var seen [2]webui.PagedQuery
	table := func(i int) webui.Table[Device] {
		return webui.Table[Device]{
			Title: fmt.Sprintf("T%d", i), PageSize: 5,
			Loader: webui.PagedLoader[Device]{Load: func(_ context.Context, q webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
				seen[i] = q
				return webui.PagedWindow[Device]{Items: []Device{{Id: "row"}}, Total: 50}, webui.Ready()
			}},
			Columns: []webui.Accessor[Device]{webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}},
		}
	}
	page := webui.Page[tableArgs]{Path: "/p", View: webui.Split{table(0), table(1)}}
	return webui.App{Pages: webui.Pages{page}}.MustCompile(""), &seen
}

func TestTwoTablesKeepIndependentStateUnderTheirTitles(t *testing.T) {
	t.Parallel()

	h, seen := twoTables()
	p := pageOf(t, h, "/p?t0.offset=10&t1.offset=15&t1.sort=id&t1.desc=true&q=x")

	assert.DeepEqual(t, seen[0], webui.PagedQuery{Offset: 10, Limit: 5})
	assert.DeepEqual(t, seen[1], webui.PagedQuery{Offset: 15, Limit: 5, Sort: "ID", Desc: true})

	// Each table's links change only its own state and carry the other's along.
	assert.True(t, hasLink(p, "Next", "/p?q=x&t0.offset=15&t1.desc=true&t1.offset=15&t1.sort=id"))
	assert.True(t, hasLink(p, "Next", "/p?q=x&t0.offset=10&t1.desc=true&t1.offset=20&t1.sort=id"))
	// Sorting one table returns only that table to its first page, and a column
	// already sorted descending sorts ascending again.
	assert.True(t, hasLink(p, "ID", "/p?q=x&t0.sort=id&t1.desc=true&t1.offset=15&t1.sort=id"))
	assert.True(t, hasLink(p, "ID", "/p?q=x&t0.offset=10&t1.sort=id"))
}

func TestViewStateForAPanelThePageDoesNotHaveIsIgnored(t *testing.T) {
	t.Parallel()

	h, seen := twoTables()
	p := pageOf(t, h, "/p?ghost.offset=5&left.offset=abc&table.offset=-4")
	assert.DeepEqual(t, seen[0], webui.PagedQuery{Limit: 5}) // unreadable: the first page
	assert.DeepEqual(t, seen[1], webui.PagedQuery{Limit: 5}) // negative: the first page
	for _, l := range p.Links() {
		if l.Name != "Refresh" { // which reloads the address as it is
			assert.False(t, strings.Contains(l.Href, "ghost")) // never carried into a link
		}
	}
	for _, f := range p.Fields() {
		assert.False(t, strings.Contains(f.Name+f.Value, "ghost")) // or a form
	}
}

func TestRefreshHoldsTheWholeAddressIncludingViewState(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	p := pageOf(t, h, "/admin/device?devices.sort=id&devices.offset=10")
	// The page's own address, view state included, is what a refresh link holds.
	// Refresh reloads the address the browser is on, exactly as it has it.
	assert.True(t, hasLink(p, "Refresh", "/admin/device?devices.sort=id&devices.offset=10"))
}

func TestEveryColumnIsSortableAndLoadReceivesItsLabel(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)
	p := pageOf(t, h, "/admin/device")

	// Four columns, four sort links: nothing is declared to make a column sortable.
	unsorted := 0
	for _, c := range page.One(t, "the table", p.ByRole("table", "Devices")).ByRole("columnheader", "") {
		if c.Attr("aria-sort") == "none" {
			unsorted++
		}
	}
	assert.Equal(t, unsorted, 4)
	assert.True(t, hasLink(p, "ID", "/admin/device?devices.sort=id"))                   // ID declares Key "id"
	assert.True(t, hasLink(p, "Status", "/admin/device?devices.sort=status"))           // no Key: the Label
	assert.True(t, hasLink(p, "Occurrences", "/admin/device?devices.sort=occurrences")) // numbers too
	assert.True(t, hasLink(p, "Rate", "/admin/device?devices.sort=rate"))               // and a slider

	serve(h, http.MethodGet, "/admin/device?devices.sort=status&devices.desc=true")
	assert.DeepEqual(t, *last, webui.PagedQuery{Limit: 10, Sort: "Status", Desc: true})

	serve(h, http.MethodGet, "/admin/device?devices.sort=id")
	assert.Equal(t, last.Sort, "ID")

	// The address names a column by its label in lower case; the Label as written
	// is not a way in, and neither is a name no column has.
	serve(h, http.MethodGet, "/admin/device?devices.sort=ID")
	assert.Equal(t, last.Sort, "")
	serve(h, http.MethodGet, "/admin/device?devices.sort=nonsense")
	assert.Equal(t, last.Sort, "")
}

func TestEveryColumnHasAFilterAFixedSetOfOptionsIsAMultiSelect(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	p := pageOf(t, h, "/admin/device")

	// One filter per column, in the header.
	for _, name := range []string{"ID", "Status", "Occurrences", "Rate"} {
		assert.Equal(t, len(p.ByRole("button", "Filter "+name)), 1)
	}

	// The Select's options are the filter's: a multi-select of exactly those, in the order they were declared.
	var options []string
	for _, f := range fieldsNamed(p, "devices.filter.status") {
		assert.Equal(t, f.Type, "checkbox")
		options = append(options, f.Value)
	}
	assert.DeepEqual(t, options, []string{"healthy", "degraded"})

	// A text column takes text; the two numeric ones, Occurrences and Rate, take a
	// minimum and a maximum instead, never text.
	types := map[string]int{}
	for _, f := range p.Fields() {
		types[f.Type]++
	}
	assert.Equal(t, types["search"], 1)
	assert.Equal(t, len(p.ByAttr("placeholder", "Contains…")), 1)
	assert.Equal(t, types["number"], 4)
	for _, name := range []string{"devices.min.occurrences", "devices.max.occurrences", "devices.min.rate", "devices.max.rate"} {
		assert.Equal(t, len(fieldsNamed(p, name)), 1)
	}
	for _, name := range []string{"devices.filter.occurrences", "devices.filter.rate"} {
		assert.Equal(t, len(fieldsNamed(p, name)), 0)
	}

	// The form is a plain GET back to the page, so it works without script. It stands beside the table, not in it (the table may be a form of
	// its own, and a form cannot be in a form), and the controls of the filter say that it is theirs, by the form they name.
	forms := p.ByAttr("data-filter-form", "")
	assert.Equal(t, len(forms), 4)
	ids := map[string]bool{}
	for _, f := range forms {
		assert.Equal(t, f.Attr("method"), "get")
		assert.Equal(t, f.Attr("action"), "/admin/device")
		ids[f.Attr("id")] = true
	}
	lowest, _ := p.Field("devices.min.occurrences")
	assert.True(t, ids[lowest.Form])
	applying := map[string]bool{}
	for _, b := range p.ByRole("button", "Apply") {
		applying[b.Attr("form")] = true
	}
	assert.DeepEqual(t, applying, ids) // each filter has its own button, and it applies that filter
}

func TestFiltersAreNamedByLabelInTheAddressAndInQuery(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)

	// A column is named in the address by its label in lower case, and in Query by
	// the label as written.
	p := pageOf(t, h, "/admin/device")
	assert.Equal(t, len(fieldsNamed(p, "devices.filter.id")), 1)
	assert.True(t, len(fieldsNamed(p, "devices.filter.status")) > 0)
	assert.Equal(t, len(fieldsNamed(p, "devices.min.occurrences")), 1) // numeric: bounds
	assert.Equal(t, len(fieldsNamed(p, "devices.filter.ID")), 0)

	serve(h, http.MethodGet, "/admin/device?devices.filter.id=dev01&devices.filter.status=healthy")
	assert.DeepEqual(t, last.Filters, map[string][]string{"ID": {"dev01"}, "Status": {"healthy"}})

	serve(h, http.MethodGet, "/admin/device?devices.filter.ID=dev01") // the Label as written is no way in
	assert.True(t, last.Filters == nil)
}

func TestLoadReceivesOnlyFiltersThatMakeSense(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)

	serve(h, http.MethodGet, "/admin/device?devices.filter.status=healthy&devices.filter.status=degraded&devices.filter.status=healthy")
	assert.DeepEqual(t, last.Filters, map[string][]string{"Status": {"healthy", "degraded"}}) // in order, once each

	serve(h, http.MethodGet, "/admin/device?devices.filter.status=bogus&devices.filter.status=healthy")
	assert.DeepEqual(t, last.Filters, map[string][]string{"Status": {"healthy"}}) // not an option: dropped

	serve(h, http.MethodGet, "/admin/device?devices.filter.status=bogus")
	assert.True(t, last.Filters == nil)

	serve(h, http.MethodGet, "/admin/device?devices.filter.id=%20%20dev01%20&devices.filter.status=")
	assert.DeepEqual(t, last.Filters, map[string][]string{"ID": {"dev01"}}) // trimmed; empty is no filter

	serve(h, http.MethodGet, "/admin/device?devices.filter.id="+strings.Repeat("x", 500))
	assert.Equal(t, len(last.Filters["ID"][0]), 200) // typed by a person: bounded

	serve(h, http.MethodGet, "/admin/device?devices.filter.nope=1")
	assert.True(t, last.Filters == nil) // a column the table does not have
}

func TestAnActiveFilterShowsAndCanBeCleared(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	p := pageOf(t, h, "/admin/device?devices.offset=10&devices.sort=id&devices.filter.id=dev0&devices.filter.status=healthy")

	typed, _ := p.Field("devices.filter.id")
	assert.Equal(t, typed.Value, "dev0") // the text, as typed
	for _, f := range fieldsNamed(p, "devices.filter.status") {
		if f.Type == "checkbox" { // the options; the others are what the other columns' filters carry
			assert.Equal(t, f.Checked, f.Value == "healthy") // the option that is chosen, and no other
		}
	}

	// Applying one column's filter carries everything else and drops every offset;
	// the filter being replaced is not carried.
	assert.DeepEqual(t, carried(t, p, "devices.filter.id"), map[string][]string{
		"devices.sort":          {"id"},
		"devices.filter.status": {"healthy"},
	})

	// Clear removes that filter, and the offset, and keeps the rest.
	assert.True(t, hasLink(p, "Clear", "/admin/device?devices.filter.status=healthy&devices.sort=id"))
}

func TestSortAndPagerLinksKeepFiltersAsRepeatedParameters(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	p := pageOf(t, h, "/admin/device?devices.filter.status=healthy&devices.filter.status=degraded")
	assert.True(t, hasLink(p, "ID", "/admin/device?devices.filter.status=healthy&devices.filter.status=degraded&devices.sort=id"))
	assert.True(t, hasLink(p, "Next", "/admin/device?devices.filter.status=healthy&devices.filter.status=degraded&devices.offset=10"))
}

func TestNumericColumnsFilterByRangeNotText(t *testing.T) {
	t.Parallel()

	h, last := tableApp(nil)
	f := func(v float64) *float64 { return &v }

	// Either bound, both, or neither; the key is the Label, as these have no Key.
	serve(h, http.MethodGet, "/admin/device?devices.min.occurrences=5")
	assert.DeepEqual(t, last.Ranges, map[string]webui.PagedRange{"Occurrences": {Min: f(5)}})
	assert.True(t, last.Filters == nil)

	serve(h, http.MethodGet, "/admin/device?devices.max.occurrences=10.5&devices.min.rate=-2")
	assert.DeepEqual(t, last.Ranges, map[string]webui.PagedRange{"Occurrences": {Max: f(10.5)}, "Rate": {Min: f(-2)}})

	serve(h, http.MethodGet, "/admin/device?devices.min.occurrences=5&devices.max.occurrences=5")
	assert.DeepEqual(t, last.Ranges, map[string]webui.PagedRange{"Occurrences": {Min: f(5), Max: f(5)}}) // equal bounds: exactly 5

	// A bound that is not a finite number never arrives; neither does text for a number column.
	for _, bad := range []string{"abc", "NaN", "Inf", "-Inf", "1e999", ""} {
		serve(h, http.MethodGet, "/admin/device?devices.min.occurrences="+bad+"&devices.max.rate="+bad)
		assert.True(t, last.Ranges == nil)
	}
	serve(h, http.MethodGet, "/admin/device?devices.filter.occurrences=5&devices.filter.rate=5")
	assert.True(t, last.Filters == nil)
	assert.True(t, last.Ranges == nil)
}

func TestAnActiveRangeShowsItsBoundsAndClearsBoth(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	p := pageOf(t, h, "/admin/device?devices.min.occurrences=5&devices.max.occurrences=9.5&devices.sort=id&devices.offset=10")

	lowest, _ := p.Field("devices.min.occurrences")
	highest, _ := p.Field("devices.max.occurrences")
	assert.Equal(t, lowest.Value, "5")
	assert.Equal(t, highest.Value, "9.5")

	// Applying another column keeps these bounds; applying this one replaces them, and drops the offset.
	assert.DeepEqual(t, carried(t, p, "devices.filter.id"), map[string][]string{
		"devices.sort":            {"id"},
		"devices.min.occurrences": {"5"},
		"devices.max.occurrences": {"9.5"},
	})
	assert.DeepEqual(t, carried(t, p, "devices.min.occurrences"), map[string][]string{"devices.sort": {"id"}})

	// Clear drops both bounds and the offset, and keeps the sort.
	assert.True(t, hasLink(p, "Clear", "/admin/device?devices.sort=id"))
}

func TestATableWithNoPageSizePagesBy25(t *testing.T) {
	t.Parallel()

	var asked int
	page := webui.Page[webui.NoArgs]{Path: "/device", View: webui.Table[Device]{
		Loader: webui.PagedLoader[Device]{Load: func(_ context.Context, q webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
			asked = q.Limit
			var all []Device
			for i := range 60 {
				all = append(all, Device{Id: fmt.Sprintf("dev%02d", i)})
			}
			end := min(q.Offset+q.Limit, len(all))
			return webui.PagedWindow[Device]{Items: all[q.Offset:end], Total: len(all)}, webui.Ready()
		}},
		Columns: []webui.Accessor[Device]{formID},
	}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	body := serve(h, http.MethodGet, "/admin/device").Body.String()

	assert.Equal(t, asked, 25)
	assert.Contains(t, body, "1–25 of 60")
	assert.Contains(t, body, "dev24")
	assert.False(t, strings.Contains(body, "dev25")) // on the next page
}

// viewTable is a table that records the Query it was asked.
func viewTable(title string, seen *webui.PagedQuery) webui.Table[Device] {
	return webui.Table[Device]{
		Title: title,
		Loader: webui.PagedLoader[Device]{Load: func(_ context.Context, q webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
			*seen = q
			return webui.PagedWindow[Device]{Items: []Device{{Id: "row"}}, Total: 1}, webui.Ready()
		}},
		Columns: []webui.Accessor[Device]{webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}},
	}
}

func TestATablesStateIsNamedByItsTitleAndAnUntitledOneIsTable(t *testing.T) {
	t.Parallel()

	var a, b webui.PagedQuery
	page := webui.Page[webui.NoArgs]{Path: "/p", View: webui.Stack{viewTable("Open alerts", &a), viewTable("", &b)}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("")
	serve(h, http.MethodGet, "/p?open-alerts.sort=id&table.sort=id&table.desc=true")

	assert.DeepEqual(t, a, webui.PagedQuery{Limit: 25, Sort: "ID"})
	assert.DeepEqual(t, b, webui.PagedQuery{Limit: 25, Sort: "ID", Desc: true})
}

func TestTablesWithTheSameTitleOnScreenTogetherAreToldApartByANumber(t *testing.T) {
	t.Parallel()

	var first, second webui.PagedQuery
	page := webui.Page[webui.NoArgs]{Path: "/p", View: webui.Split{viewTable("Devices", &first), viewTable("Devices", &second)}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("")
	serve(h, http.MethodGet, "/p?devices.sort=id&devices-2.offset=3")

	assert.Equal(t, first.Sort, "ID")
	assert.Equal(t, first.Offset, 0)
	assert.Equal(t, second.Sort, "")
	assert.Equal(t, second.Offset, 3)
}

func TestTheTablesOfOneTabsShareANameAndAnotherTabsIsNumbered(t *testing.T) {
	t.Parallel()

	// The same table in both panels of a Tabs keeps its state in both, since they are
	// never shown together. A second Tabs is told apart by a number.
	var outside, inA, inB, inOther webui.PagedQuery
	page := webui.Page[webui.NoArgs]{Path: "/p", View: webui.Stack{
		viewTable("Events", &outside),
		webui.Tabs{
			{Label: "A", View: viewTable("Events", &inA)},
			{Label: "B", View: viewTable("Events", &inB)},
		},
		webui.Tabs{{Label: "Other", View: viewTable("Other", &inOther)}, {Label: "More", View: webui.Stack{}}},
	}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("")

	// Outside the Tabs it is "events", so the one in a panel is "events-2" in both panels.
	serve(h, http.MethodGet, "/p?events.sort=id&events-2.offset=7")
	assert.Equal(t, outside.Sort, "ID")
	assert.Equal(t, inA.Offset, 7)
	serve(h, http.MethodGet, "/p?tabs.tab=b&events-2.offset=7")
	assert.Equal(t, inB.Offset, 7)

	// The second Tabs keeps its selection under "tabs-2".
	more := pageOf(t, h, "/p?tabs-2.tab=more").ByRole("link", "More")
	assert.Equal(t, len(more), 1)
	assert.Equal(t, more[0].Attr("href"), "/p?tabs-2.tab=more")
	assert.Equal(t, more[0].Attr("aria-current"), "page") // the tab that is selected
}
