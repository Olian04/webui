package webui_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type searchResponse struct {
	Results []struct {
		Group, Title, Desc, Href string
	}
}

func search(t *testing.T, h http.Handler, query string) searchResponse {
	t.Helper()

	rec := serve(h, http.MethodGet, "/admin/_webui/search?q="+query)
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, rec.Header().Get("Content-Type"), "application/json; charset=utf-8")
	assert.Equal(t, rec.Header().Get("Cache-Control"), "no-store")
	var out searchResponse
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

type searchPageArgs struct{ Q string }

var (
	searchID   = webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}
	searchIP   = webui.String[Device]{Label: "IP", Load: func(d Device) string { return d.Ip }}
	searchSite = webui.Int[Device]{Label: "Occurrences", Load: func(d Device) int { return d.Count }}
)

func searchDevices() []Device {
	return []Device{
		{Id: "dev-eth 0", Ip: "10.0.0.1", Count: 4},
		{Id: "dev-eth 1", Ip: "10.0.0.2", Count: 5},
		{Id: "gateway", Ip: "192.168.1.1", Count: 6},
	}
}

// searchTable is a table that offers its rows to the search, from Rows or Load.
func searchTable(details webui.PageRef[detailsArgs]) webui.Table[Device] {
	return webui.Table[Device]{
		Title:   "Devices",
		Search:  true,
		Columns: []webui.Accessor[Device]{searchID, searchIP, searchSite},
		RowClick: webui.Link[Device, detailsArgs]{
			Page: details,
			Args: func(_ context.Context, d Device) detailsArgs { return detailsArgs{Id: d.Id} },
		},
	}
}

// searchApp has a list whose table is searchable and a detail page its rows lead to.
func searchApp(table func(webui.Table[Device]) webui.Table[Device], guard func(context.Context, searchPageArgs) error) http.Handler {
	var details webui.Page[detailsArgs]
	details.Path = "/device/{id}"
	details.Body = webui.Stack{}
	base := table(searchTable(details))
	list := webui.Page[searchPageArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, Guard: guard, Body: base}
	other := webui.Page[webui.NoArgs]{Path: "/other", Nav: webui.Nav{Label: "Other"}, Body: webui.Stack{}}
	return webui.App{Pages: webui.Pages{list, details, other}}.MustCompile("/admin")
}

func withRows(t webui.Table[Device]) webui.Table[Device] {
	t.Rows = func(context.Context) ([]Device, error) { return searchDevices(), nil }
	return t
}

func TestSearchFindsRowsByAnyColumnAndShowsFirstColumnAsTheTitle(t *testing.T) {
	t.Parallel()

	h := searchApp(withRows, nil)

	got := search(t, h, "eth%200") // matches the ID column
	assert.Equal(t, len(got.Results), 1)
	assert.Equal(t, got.Results[0].Group, "Devices") // the table's own name
	assert.Equal(t, got.Results[0].Title, "dev-eth 0")
	assert.Equal(t, got.Results[0].Desc, "10.0.0.1") // the numbers are searched, but not shown without their label
	assert.Equal(t, got.Results[0].Href, "/admin/device/dev-eth%200") // the mount prefix, escaped

	// Not only the first column: an address, and a number.
	assert.Equal(t, search(t, h, "192.168").Results[0].Title, "gateway")
	assert.Equal(t, search(t, h, "6").Results[0].Title, "gateway") // 6 occurrences (and no id has a 6)
	assert.Equal(t, len(search(t, h, "ETH").Results), 2)          // ignoring case
	assert.Equal(t, len(search(t, h, "nothing").Results), 0)
}

func TestSearchOfALoadTableIsHandedTheTextInQuerySearch(t *testing.T) {
	t.Parallel()

	var seen webui.Query
	h := searchApp(func(t webui.Table[Device]) webui.Table[Device] {
		t.Load = func(_ context.Context, q webui.Query) (webui.Window[Device], error) {
			seen = q
			if q.Search == "" {
				return webui.Window[Device]{Items: searchDevices(), Total: 3}, nil
			}
			return webui.Window[Device]{Items: searchDevices()[2:], Total: 1}, nil
		}
		return t
	}, nil)

	got := search(t, h, "gate")
	assert.Equal(t, seen.Search, "gate")
	assert.Equal(t, seen.Limit, 8) // a handful, not the table
	assert.Equal(t, len(got.Results), 1)

	// The table's own page never carries it.
	seen = webui.Query{Search: "stale"}
	serve(h, http.MethodGet, "/admin/device")
	assert.Equal(t, seen.Search, "")
}

func TestSearchIsEmptyForNoQueryAndWhenNoTableOffersIt(t *testing.T) {
	t.Parallel()

	called := false
	h := searchApp(func(t webui.Table[Device]) webui.Table[Device] {
		t.Rows = func(context.Context) ([]Device, error) { called = true; return nil, nil }
		return t
	}, nil)
	assert.Equal(t, len(search(t, h, "").Results), 0)
	assert.Equal(t, len(search(t, h, "%20%20").Results), 0)
	assert.False(t, called)

	notSearchable := searchApp(func(t webui.Table[Device]) webui.Table[Device] {
		t = withRows(t)
		t.Search = false
		return t
	}, nil)
	assert.Equal(t, len(search(t, notSearchable, "dev").Results), 0)
}

func TestSearchRunsThePageGuardWithZeroArgumentsFirst(t *testing.T) {
	t.Parallel()

	var guarded searchPageArgs
	called := false
	h := searchApp(func(t webui.Table[Device]) webui.Table[Device] {
		t.Rows = func(context.Context) ([]Device, error) { called = true; return searchDevices(), nil }
		return t
	}, func(_ context.Context, a searchPageArgs) error {
		guarded = a
		return errors.New("not for you")
	})
	assert.Equal(t, len(search(t, h, "dev").Results), 0)
	assert.False(t, called) // a refused page's table is never asked
	assert.Equal(t, guarded, searchPageArgs{})
}

func TestSearchCapsHitsPerTableAndSurvivesAFailingTable(t *testing.T) {
	t.Parallel()

	many := searchApp(func(t webui.Table[Device]) webui.Table[Device] {
		t.Rows = func(context.Context) ([]Device, error) {
			var out []Device
			for i := range 30 {
				out = append(out, Device{Id: fmt.Sprint("dev", i)})
			}
			return out, nil
		}
		return t
	}, nil)
	assert.Equal(t, len(search(t, many, "dev").Results), 8)

	failing := searchApp(func(t webui.Table[Device]) webui.Table[Device] {
		t.Rows = func(context.Context) ([]Device, error) { return nil, errors.New("backend down: hunter2") }
		return t
	}, nil)
	rec := serve(failing, http.MethodGet, "/admin/_webui/search?q=x")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.False(t, strings.Contains(rec.Body.String(), "hunter2"))
}

func TestASearchableTableOnTwoPagesIsOneResult(t *testing.T) {
	t.Parallel()

	var details webui.Page[detailsArgs]
	details.Path = "/device/{id}"
	details.Body = webui.Stack{}
	table := withRows(searchTable(details))
	a := webui.Page[webui.NoArgs]{Path: "/a", Body: table}
	b := webui.Page[webui.NoArgs]{Path: "/b", Body: webui.Stack{table}}
	h := webui.App{Pages: webui.Pages{a, b, details}}.MustCompile("/admin")
	assert.Equal(t, len(search(t, h, "gateway").Results), 1)
}

func TestTableSearchNeedsARowClickColumnsAndAPageWithoutPathArguments(t *testing.T) {
	t.Parallel()

	rows := func(context.Context) ([]Device, error) { return nil, nil }
	var details webui.Page[detailsArgs]
	details.Path = "/device/{id}"
	details.Body = webui.Stack{}
	click := webui.Link[Device, detailsArgs]{Page: details, Args: func(context.Context, Device) detailsArgs { return detailsArgs{} }}
	cols := []webui.Accessor[Device]{searchID}

	errorsOf := func(page webui.Page[detailsArgs]) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{page, details}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	none := webui.Page[detailsArgs]{Path: "/device/{id}/x", Body: webui.Table[Device]{Rows: rows, Search: true, Columns: cols, RowClick: click}}
	assert.Contains(t, errorsOf(none), "a Table on a page with path arguments cannot have Search")

	var noClick, noCols string
	for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: webui.Table[Device]{Rows: rows, Search: true, Columns: cols}}, details}}) {
		noClick += e.Error()
	}
	for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: webui.Table[Device]{Rows: rows, Search: true, RowClick: click}}, details}}) {
		noCols += e.Error()
	}
	assert.Contains(t, noClick, "a Table has Search but no RowClick")
	assert.Contains(t, noCols, "a Table has Search but no Columns")
}

func TestSearchBoxKnowsWhereToAskOnlyWhenATableOffersSearch(t *testing.T) {
	t.Parallel()

	with := searchApp(withRows, nil)
	body := serve(with, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, body, `data-search-url="/admin/_webui/search"`)
	assert.Contains(t, body, `role="combobox"`)
	assert.Contains(t, body, `role="listbox"`)

	without := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: webui.Stack{}}}}.MustCompile("/admin")
	assert.False(t, strings.Contains(serve(without, http.MethodGet, "/admin/a").Body.String(), "data-search-url"))
}

func TestInfoIconCarriesItsDescriptionInAPopover(t *testing.T) {
	t.Parallel()

	body := serve(formApp(okForm()), http.MethodGet, "/admin/device/dev1").Body.String()
	assert.Contains(t, body, `<span class="tip" tabindex="0" role="note" aria-label="One device.">`)
	assert.Contains(t, body, `<span class="tip-body" aria-hidden="true">One device.</span>`)
}

func TestPanelsHaveNoRefreshButtonOfTheirOwn(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	for _, absent := range []string{"refresh-leaf", "Refresh this panel", `class="panel-menu"`} {
		assert.False(t, strings.Contains(body, absent))
	}
	// The page's own Refresh is the one reload.
	assert.Equal(t, strings.Count(body, `fa-rotate-right`), 1)
	assert.Contains(t, body, `data-refresh`)
}

// The search box shows focus as a ring, which is how the Cmd+K shortcut shows
// it worked.
func TestSearchBoxHasAVisibleFocusRing(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	css := serve(h, http.MethodGet, "/admin/_webui/app.css").Body.String()
	assert.Contains(t, css, ".search:focus-within {\n  border-color: var(--blue);\n  box-shadow: 0 0 0 2px")
}

// guardedSearchApp finds three devices; the detail page's Guard decides which of
// them this visitor may open.
func guardedSearchApp(allowed func(id string) error) http.Handler {
	var details webui.Page[detailsArgs]
	details.Path = "/device/{id}"
	details.Guard = func(_ context.Context, a detailsArgs) error { return allowed(a.Id) }
	details.Body = webui.Stack{}
	table := searchTable(details)
	table.Rows = func(context.Context) ([]Device, error) {
		return []Device{{Id: "dev-a"}, {Id: "dev-secret"}, {Id: "dev-b"}}, nil
	}
	list := webui.Page[webui.NoArgs]{Path: "/device", Body: table}
	return webui.App{Pages: webui.Pages{list, details}}.MustCompile("/admin")
}

func TestSearchOffersOnlyWhatTheVisitorCouldOpen(t *testing.T) {
	t.Parallel()

	h := guardedSearchApp(func(id string) error {
		if id == "dev-secret" {
			return errors.New("requires the admin role")
		}
		return nil
	})
	var titles []string
	for _, r := range search(t, h, "dev").Results {
		titles = append(titles, r.Title)
	}
	assert.DeepEqual(t, titles, []string{"dev-a", "dev-b"}) // the one the destination refuses is not a hit

	// And nothing of it leaks: not in the body, not in the title, not in the href.
	body := serve(h, http.MethodGet, "/admin/_webui/search?q=dev").Body.String()
	assert.False(t, strings.Contains(body, "secret"))
	assert.False(t, strings.Contains(body, "admin role"))
}

func TestSearchChecksTheGuardWithTheResultsOwnArguments(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	h := guardedSearchApp(func(id string) error { seen[id] = true; return nil })
	search(t, h, "dev")
	assert.True(t, seen["dev-a"] && seen["dev-secret"] && seen["dev-b"]) // each result, by its own id
}

func TestSearchDropsAResultWhoseLinkCannotBeBuilt(t *testing.T) {
	t.Parallel()

	var details webui.Page[detailsArgs]
	details.Path = "/device/{id}"
	details.Body = webui.Stack{}
	table := searchTable(details)
	table.Rows = func(context.Context) ([]Device, error) {
		return []Device{{Id: ""}, {Id: "dev-ok"}}, nil // an empty path argument has no address
	}
	table.Columns = []webui.Accessor[Device]{searchIP, searchID}
	list := webui.Page[webui.NoArgs]{Path: "/device", Body: webui.Table[Device]{
		Search: true, Rows: table.Rows, Columns: []webui.Accessor[Device]{searchID, searchIP},
		RowClick: table.RowClick,
	}}
	h := webui.App{Pages: webui.Pages{list, details}}.MustCompile("/admin")
	got := search(t, h, "dev")
	assert.Equal(t, len(got.Results), 1)
	assert.Equal(t, got.Results[0].Title, "dev-ok")
}
