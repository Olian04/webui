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

// searchApp has a list that offers Search and a detail page it links to.
func searchApp(searchFn func(context.Context, string) ([]webui.SearchResult, error), guard func(context.Context, searchPageArgs) error) http.Handler {
	details := webui.Page[detailsArgs]{Path: "/device/{id}", Body: webui.Stack{}}
	list := webui.Page[searchPageArgs]{
		Path: "/device", Nav: webui.Nav{Label: "Devices"}, Guard: guard, Body: webui.Stack{},
		Search: searchFn,
	}
	other := webui.Page[webui.NoArgs]{Path: "/other", Nav: webui.Nav{Label: "Other"}, Body: webui.Stack{}}
	return webui.App{Pages: webui.Pages{list, details, other}}.MustCompile("/admin")
}

func TestSearchAsksEveryPageThatOffersItAndResolvesLinks(t *testing.T) {
	t.Parallel()

	var details webui.Page[detailsArgs]
	details.Path = "/device/{id}"
	h := searchApp(func(ctx context.Context, q string) ([]webui.SearchResult, error) {
		return []webui.SearchResult{{
			Title: "dev-" + q, Desc: "10.0.0.1 · Stockholm",
			Target: webui.Open(ctx, details, detailsArgs{Id: "dev-" + q}),
		}}, nil
	}, nil)

	got := search(t, h, "eth%200")
	assert.Equal(t, len(got.Results), 1)
	assert.Equal(t, got.Results[0].Group, "Devices")
	assert.Equal(t, got.Results[0].Title, "dev-eth 0")
	assert.Equal(t, got.Results[0].Desc, "10.0.0.1 · Stockholm")
	assert.Equal(t, got.Results[0].Href, "/admin/device/dev-eth%200") // the mount prefix, escaped
}

func TestSearchIsEmptyForNoQueryAndWhenNoPageOffersIt(t *testing.T) {
	t.Parallel()

	called := false
	h := searchApp(func(context.Context, string) ([]webui.SearchResult, error) { called = true; return nil, nil }, nil)
	assert.Equal(t, len(search(t, h, "").Results), 0)
	assert.Equal(t, len(search(t, h, "%20%20").Results), 0)
	assert.False(t, called)

	none := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: webui.Stack{}}}}.MustCompile("/admin")
	assert.Equal(t, len(search(t, none, "x").Results), 0)
}

func TestSearchRunsThePageGuardWithZeroArgumentsFirst(t *testing.T) {
	t.Parallel()

	var guarded searchPageArgs
	called := false
	h := searchApp(func(context.Context, string) ([]webui.SearchResult, error) {
		called = true
		return []webui.SearchResult{{Title: "secret", Target: webui.Target{URL: "/admin/device"}}}, nil
	}, func(_ context.Context, a searchPageArgs) error {
		guarded = a
		return errors.New("not for you")
	})
	assert.Equal(t, len(search(t, h, "x").Results), 0)
	assert.False(t, called) // a refused page is never asked
	assert.Equal(t, guarded, searchPageArgs{})
}

func TestSearchDropsBadResultsAndKeepsTheGoodOnes(t *testing.T) {
	t.Parallel()

	h := searchApp(func(context.Context, string) ([]webui.SearchResult, error) {
		return []webui.SearchResult{
			{Title: "broken link", Target: webui.Target{Err: errors.New("not mounted")}},
			{Title: "elsewhere", Target: webui.Target{URL: "https://evil.example/"}},
			{Title: "protocol-relative", Target: webui.Target{URL: "//evil.example/"}},
			{Title: "script", Target: webui.Target{URL: "javascript:alert(1)"}},
			{Title: "fine", Target: webui.Target{URL: "/admin/device"}},
		}, nil
	}, nil)
	got := search(t, h, "x")
	assert.Equal(t, len(got.Results), 1)
	assert.Equal(t, got.Results[0].Title, "fine")
}

func TestSearchCapsHitsPerPageAndSurvivesAFailingPage(t *testing.T) {
	t.Parallel()

	many := searchApp(func(context.Context, string) ([]webui.SearchResult, error) {
		var out []webui.SearchResult
		for i := range 30 {
			out = append(out, webui.SearchResult{Title: fmt.Sprint(i), Target: webui.Target{URL: "/admin/device"}})
		}
		return out, nil
	}, nil)
	assert.Equal(t, len(search(t, many, "x").Results), 8)

	failing := searchApp(func(context.Context, string) ([]webui.SearchResult, error) {
		return nil, errors.New("backend down: hunter2")
	}, nil)
	rec := serve(failing, http.MethodGet, "/admin/_webui/search?q=x")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.False(t, strings.Contains(rec.Body.String(), "hunter2"))
}

func TestPageSearchRequiresAPageWithoutPathArguments(t *testing.T) {
	t.Parallel()

	app := webui.App{Pages: webui.Pages{webui.Page[detailsArgs]{
		Path: "/device/{id}", Body: webui.Stack{},
		Search: func(context.Context, string) ([]webui.SearchResult, error) { return nil, nil },
	}}}
	errs := compileErrors(t, app)
	assert.Equal(t, len(errs), 1)
	assert.Contains(t, errs[0].Detail, "cannot offer Search")
}

func TestSearchBoxKnowsWhereToAskOnlyWhenAPageOffersSearch(t *testing.T) {
	t.Parallel()

	with := searchApp(func(context.Context, string) ([]webui.SearchResult, error) { return nil, nil }, nil)
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
