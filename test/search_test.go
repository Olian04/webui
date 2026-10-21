package webui_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type searchResponse struct {
	Results []struct {
		Group, Title, Desc, Href string
		Score                    float64
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

// deviceHit is a device as a result of the page that shows one.
func deviceHit(d Device) webui.Hit[detailsArgs] {
	return webui.Hit[detailsArgs]{Args: detailsArgs{Id: d.Id}, Title: d.Id, Desc: d.Ip}
}

// searchApp has a list page, and the detail page that finds the devices: its Search, and the Guard
// that decides which of them a visitor may open.
func searchApp(search func(context.Context, string, int) ([]webui.Hit[detailsArgs], error), guard func(context.Context, detailsArgs) error) http.Handler {
	details := webui.Page[detailsArgs]{Path: "/device/{id}", Guard: guard, Search: search, Body: webui.Stack{}}
	list := webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, Body: webui.Stack{}}
	other := webui.Page[webui.NoArgs]{Path: "/other", Nav: webui.Nav{Label: "Other"}, Body: webui.Stack{}}
	return webui.App{Pages: webui.Pages{list, details, other}}.MustCompile("/admin")
}

// rowsSearch is a page's Search made from the devices in memory.
func rowsSearch() func(context.Context, string, int) ([]webui.Hit[detailsArgs], error) {
	return webui.SearchRows(
		func(context.Context) ([]Device, error) { return searchDevices(), nil },
		deviceHit, searchID, searchIP, searchSite,
	)
}

func TestSearchRowsFindsRowsByAnyFieldIgnoringCaseAndLeadsToThePage(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)

	got := search(t, h, "eth%200") // matches the ID
	assert.Equal(t, len(got.Results), 1)
	assert.Equal(t, got.Results[0].Title, "dev-eth 0")
	assert.Equal(t, got.Results[0].Desc, "10.0.0.1")
	assert.Equal(t, got.Results[0].Href, "/admin/device/dev-eth%200") // the page with the hit's arguments: the mount prefix, escaped

	// Not only the first field: an address, and a number, which is searched as it is shown.
	assert.Equal(t, search(t, h, "192.168").Results[0].Title, "gateway")
	assert.Equal(t, search(t, h, "6").Results[0].Title, "gateway") // 6 occurrences (and no id has a 6)
	assert.Equal(t, len(search(t, h, "ETH").Results), 2)           // ignoring case
	assert.Equal(t, len(search(t, h, "nothing").Results), 0)
}

func TestAPagesResultsAreListedUnderItsNameInTheNavigationOrTheEntryItBorrowsFrom(t *testing.T) {
	t.Parallel()

	// The detail page has no entry of its own, so it lights Devices: its results are Devices.
	assert.Equal(t, search(t, searchApp(rowsSearch(), nil), "gate").Results[0].Group, "Devices")

	// A page with an entry of its own is its own group.
	labelled := webui.Page[detailsArgs]{Path: "/thing/{id}", Search: rowsSearch(), Body: webui.Stack{}}
	h := webui.App{Pages: webui.Pages{labelled}}.MustCompile("/admin")
	assert.Equal(t, search(t, h, "gate").Results[0].Group, "/thing/{id}") // none at all: the path

	own := webui.Page[webui.NoArgs]{Path: "/devices", Nav: webui.Nav{Label: "All devices"}, Search: func(context.Context, string, int) ([]webui.Hit[webui.NoArgs], error) {
		return []webui.Hit[webui.NoArgs]{{Title: "The list"}}, nil
	}, Body: webui.Stack{}}
	h = webui.App{Pages: webui.Pages{own}}.MustCompile("/admin")
	assert.Equal(t, search(t, h, "x").Results[0].Group, "All devices")
	assert.Equal(t, search(t, h, "x").Results[0].Href, "/admin/devices")
}

func TestASearchIsHandedTheTextAndALimitAndTheAppsOwnContext(t *testing.T) {
	t.Parallel()

	var query string
	var limit int
	h := searchApp(func(ctx context.Context, q string, n int) ([]webui.Hit[detailsArgs], error) {
		query, limit = q, n
		// The context is the app's, as anywhere in a page: its arguments are what the page has when
		// nothing is selected.
		return []webui.Hit[detailsArgs]{{Args: detailsArgs{Id: "x"}, Title: webui.ArgsOf[detailsArgs](ctx).Id + "|ok"}}, nil
	}, nil)

	got := search(t, h, "gate")
	assert.Equal(t, query, "gate")
	assert.Equal(t, limit, 8) // a handful, not the table
	assert.Equal(t, got.Results[0].Title, "|ok")
}

func TestSearchIsEmptyForNoQueryAndForAPageThatHasNoSearch(t *testing.T) {
	t.Parallel()

	called := false
	h := searchApp(func(context.Context, string, int) ([]webui.Hit[detailsArgs], error) { called = true; return nil, nil }, nil)
	assert.Equal(t, len(search(t, h, "").Results), 0)
	assert.Equal(t, len(search(t, h, "%20%20").Results), 0)
	assert.False(t, called)

	none := searchApp(nil, nil)
	assert.Equal(t, len(search(t, none, "dev").Results), 0)
}

func TestSearchCapsTheHitsOfAPageAndSurvivesAFailingOrPanickingPage(t *testing.T) {
	t.Parallel()

	many := func(context.Context, string, int) ([]webui.Hit[detailsArgs], error) {
		var hits []webui.Hit[detailsArgs]
		for i := range 20 {
			hits = append(hits, webui.Hit[detailsArgs]{Args: detailsArgs{Id: fmt.Sprintf("d%d", i)}, Title: "t"})
		}
		return hits, nil
	}
	assert.Equal(t, len(search(t, searchApp(many, nil), "d").Results), 8)

	broken := webui.Page[webui.NoArgs]{Path: "/broken", Nav: webui.Nav{Label: "Broken"}, Body: webui.Stack{}, Search: func(context.Context, string, int) ([]webui.Hit[webui.NoArgs], error) {
		return nil, errors.New("db secret-host is down")
	}}
	panics := webui.Page[webui.NoArgs]{Path: "/panics", Nav: webui.Nav{Label: "Panics"}, Body: webui.Stack{}, Search: func(context.Context, string, int) ([]webui.Hit[webui.NoArgs], error) {
		panic("boom")
	}}
	ok := webui.Page[detailsArgs]{Path: "/device/{id}", Body: webui.Stack{}, Search: rowsSearch()}
	h := webui.App{Pages: webui.Pages{broken, panics, ok}}.MustCompile("/admin")
	got := search(t, h, "gate")
	assert.Equal(t, len(got.Results), 1) // the others answer
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/_webui/search?q=gate").Body.String(), "secret-host"))
}

func TestAThingFoundByTwoPagesIsOneResult(t *testing.T) {
	t.Parallel()

	a := webui.Page[detailsArgs]{Path: "/device/{id}", Body: webui.Stack{}, Search: rowsSearch()}
	again := webui.Page[detailsArgs]{Path: "/device/{id}/log", Body: webui.Stack{}, Search: rowsSearch()}
	h := webui.App{Pages: webui.Pages{a, again}}.MustCompile("/admin")
	// Two pages, two addresses: two results. The same address twice is one.
	assert.Equal(t, len(search(t, h, "gateway").Results), 2)

	dup := func(context.Context, string, int) ([]webui.Hit[detailsArgs], error) {
		return []webui.Hit[detailsArgs]{{Args: detailsArgs{Id: "same"}, Title: "one"}, {Args: detailsArgs{Id: "same"}, Title: "two"}}, nil
	}
	got := search(t, searchApp(dup, nil), "x")
	assert.Equal(t, len(got.Results), 1)
	assert.Equal(t, got.Results[0].Title, "one")
}

func TestAHitWithNoTitleIsNamedByItsAddress(t *testing.T) {
	t.Parallel()

	h := searchApp(func(context.Context, string, int) ([]webui.Hit[detailsArgs], error) {
		return []webui.Hit[detailsArgs]{{Args: detailsArgs{Id: "a"}}}, nil
	}, nil)
	assert.Equal(t, search(t, h, "a").Results[0].Title, "/admin/device/a")
}

func TestSearchBoxKnowsWhereToAskOnlyWhenAPageOffersSearch(t *testing.T) {
	t.Parallel()

	with := searchApp(rowsSearch(), nil)
	body := serve(with, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, body, `data-search-url="/admin/_webui/search"`)
	assert.Contains(t, body, `role="combobox"`)
	assert.Contains(t, body, `role="listbox"`)

	without := searchApp(nil, nil)
	assert.False(t, strings.Contains(serve(without, http.MethodGet, "/admin/device").Body.String(), "data-search-url"))
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
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, ".search:focus-within {\n  border-color: var(--blue);\n  box-shadow: 0 0 0 2px")
}

// guardedSearchApp finds three devices; the detail page's Guard decides which of
// them this visitor may open.
func guardedSearchApp(allowed func(id string) error) http.Handler {
	return searchApp(func(context.Context, string, int) ([]webui.Hit[detailsArgs], error) {
		var hits []webui.Hit[detailsArgs]
		for _, id := range []string{"dev-a", "dev-secret", "dev-b"} {
			hits = append(hits, webui.Hit[detailsArgs]{Args: detailsArgs{Id: id}, Title: id})
		}
		return hits, nil
	}, func(_ context.Context, a detailsArgs) error { return allowed(a.Id) })
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

func TestSearchDoesNotAskThePagesGuardWithNoArgumentsToLetItFindThings(t *testing.T) {
	t.Parallel()

	// A Guard that refuses an empty id, as one that looks the thing up would, does not stop the page
	// from being searched: each result is checked with its own id.
	h := guardedSearchApp(func(id string) error {
		if id == "" {
			return errors.New("no such device")
		}
		return nil
	})
	assert.Equal(t, len(search(t, h, "dev").Results), 3)
}

func TestSearchDropsAResultWhoseLinkCannotBeBuilt(t *testing.T) {
	t.Parallel()

	h := searchApp(func(context.Context, string, int) ([]webui.Hit[detailsArgs], error) {
		return []webui.Hit[detailsArgs]{{Args: detailsArgs{Id: ""}, Title: "empty"}, {Args: detailsArgs{Id: "dev-ok"}, Title: "ok"}}, nil // an empty path argument has no address
	}, nil)
	got := search(t, h, "dev")
	assert.Equal(t, len(got.Results), 1)
	assert.Equal(t, got.Results[0].Title, "ok")
}

func TestAPageWithPathArgumentsFindsThingsAcrossAllOfThem(t *testing.T) {
	t.Parallel()

	type objectArgs struct{ Bucket, Name string }
	objects := map[string][]string{"photos": {"cat.png", "dog.png"}, "docs": {"cat-notes.txt"}}
	object := webui.Page[objectArgs]{
		Path: "/bucket/{bucket}/{name}",
		Search: func(_ context.Context, q string, _ int) ([]webui.Hit[objectArgs], error) {
			var hits []webui.Hit[objectArgs]
			for _, bucket := range []string{"docs", "photos"} {
				for _, name := range objects[bucket] {
					if strings.Contains(name, q) {
						hits = append(hits, webui.Hit[objectArgs]{Args: objectArgs{Bucket: bucket, Name: name}, Title: name, Desc: bucket})
					}
				}
			}
			return hits, nil
		},
		Body: webui.Stack{},
	}
	h := webui.App{Pages: webui.Pages{object}}.MustCompile("/admin")
	got := search(t, h, "cat")
	assert.Equal(t, len(got.Results), 2) // wherever the visitor is, in any bucket
	assert.Equal(t, got.Results[0].Href, "/admin/bucket/docs/cat-notes.txt")
	assert.Equal(t, got.Results[1].Href, "/admin/bucket/photos/cat.png")
}

func TestSearchRowsNeedsRowsAHitAndAField(t *testing.T) {
	t.Parallel()

	for name, s := range map[string]func(context.Context, string, int) ([]webui.Hit[detailsArgs], error){
		"no rows":  webui.SearchRows[Device, detailsArgs](nil, deviceHit, searchID),
		"no hit":   webui.SearchRows[Device, detailsArgs](func(context.Context) ([]Device, error) { return nil, nil }, nil, searchID),
		"no field": webui.SearchRows(func(context.Context) ([]Device, error) { return nil, nil }, deviceHit),
	} {
		if _, err := s(context.Background(), "x", 8); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	failing := webui.SearchRows(func(context.Context) ([]Device, error) { return nil, errors.New("down") }, deviceHit, searchID)
	_, err := failing(context.Background(), "x", 8)
	assert.Error(t, err)
}

func TestSearchOfAPageTheVisitorMayNotOpenAtAllIsNotCalledMoreThanOncePerSearch(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	h := searchApp(func(context.Context, string, int) ([]webui.Hit[detailsArgs], error) {
		calls.Add(1)
		return nil, nil
	}, nil)
	search(t, h, "x")
	assert.Equal(t, int(calls.Load()), 1)
}

var _ = httptest.NewRecorder
