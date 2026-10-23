package webui_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// titles are what the results of a search are called, in the order they are given.
func titles(r searchResponse) []string {
	var out []string
	for _, h := range r.Results {
		out = append(out, h.Title)
	}
	return out
}

// resultsPages are a list page that is named label, and the page for one of its things, whose Search finds the given titles, each leading to
// the page with its own argument, which is what makes them different results. They are listed under the list's name.
func resultsPages(path, label string, titles ...string) webui.Pages {
	list := webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs](path), Nav: webui.Nav{Label: label}, Body: webui.Stack{}}
	one := webui.Page[detailsArgs]{
		Path: webui.PageID[detailsArgs](path + "/{id}"), Body: webui.Stack{},
		Search: func(context.Context, string, int) ([]webui.Hit[detailsArgs], error) {
			var hits []webui.Hit[detailsArgs]
			for _, title := range titles {
				hits = append(hits, webui.Hit[detailsArgs]{Args: detailsArgs{Id: title}, Title: title})
			}
			return hits, nil
		},
	}
	return webui.Pages{list, one}
}

func TestTheResultsOfASearchAreOrderedByHowWellTheyMatch(t *testing.T) {
	t.Parallel()

	h := webui.App{Pages: resultsPages("/things", "Things", "My devices", "Device", "Subdevice", "Devices", "device list", "Odevice x")}.MustCompile("/admin")

	got := search(t, h, "device")
	// exact, then a word that starts it, then a start, then an end, then a word's start, then inside a word.
	assert.DeepEqual(t, titles(got), []string{"Device", "device list", "Devices", "Subdevice", "My devices", "Odevice x"})
}

func TestTheGroupsAreOrderedByTheirBestResultAndKeptTogether(t *testing.T) {
	t.Parallel()

	// Alerts is first in the app and its best result is the weakest; Sites has the exact match, and Devices has a start.
	var pages webui.Pages
	pages = append(pages, resultsPages("/alerts", "Alerts", "my stockholm alert", "alert for stockholm")...)
	pages = append(pages, resultsPages("/sites", "Sites", "My stockholm", "Stockholm")...)
	pages = append(pages, resultsPages("/devices", "Devices", "stockholm-gw")...)
	h := webui.App{Pages: pages}.MustCompile("/admin")

	got := search(t, h, "stockholm")
	var order []string
	for _, r := range got.Results {
		order = append(order, r.Group+"/"+r.Title)
	}
	assert.DeepEqual(t, order, []string{
		"Sites/Stockholm", "Sites/My stockholm", // the exact one first, and its group with it
		"Devices/stockholm-gw",                                    // a start of the title
		"Alerts/alert for stockholm", "Alerts/my stockholm alert", // a word that ends the title, above a word in the middle
	})
}

// A source that stops at a limit used to give the first matches it found; the rows are ranked first, so the best are what is kept.
func TestSearchRowsKeepsTheBestMatchesAndNotTheFirstOnes(t *testing.T) {
	t.Parallel()

	rows := []Device{{Id: "a-gate-b"}, {Id: "my gate"}, {Id: "gate"}, {Id: "gateway"}, {Id: "agate"}}
	search := webui.SearchRows(
		func(context.Context) ([]Device, error) { return rows, nil },
		func(d Device) webui.Hit[webui.NoArgs] { return webui.Hit[webui.NoArgs]{Title: d.Id} },
		searchID,
	)
	got, err := search(context.Background(), "gate", 3)
	assert.NoError(t, err)
	var ids []string
	for _, h := range got {
		ids = append(ids, h.Title)
	}
	assert.DeepEqual(t, ids, []string{"gate", "gateway", "my gate"}) // exact, a start, a word that ends it: not the first three found, which began with "a-gate-b"
	all, _ := search(context.Background(), "gate", 0)
	assert.Equal(t, len(all), 5) // no limit is all of them, ranked
	assert.Equal(t, all[0].Title, "gate")
}

func TestThereIsNoSearchBoxWhenNoPageHasASearch(t *testing.T) {
	t.Parallel()

	without := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "A"}, Body: webui.Stack{}}}}.MustCompile("/admin")
	body := serve(without, http.MethodGet, "/admin/a").Body.String()
	for _, absent := range []string{"data-palette-input", "data-palette", `class="search"`, "data-search-url", `id="palette"`} {
		assert.False(t, strings.Contains(body, absent))
	}

	with := searchApp(rowsSearch(), nil)
	body = serve(with, http.MethodGet, "/admin/device").Body.String()
	for _, present := range []string{"data-palette-input", `id="palette"`, `data-search-url="/admin/_webui/search`} {
		assert.Contains(t, body, present)
	}
}

// The search finds what the pages that have a Search find, and does not list the pages of the sidebar: the script has no results of its own.
func TestTheSearchScriptHasNoResultsOfItsOwn(t *testing.T) {
	t.Parallel()

	h := webui.App{}.MustCompile("/admin")
	js := serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Body.String()
	assert.False(t, strings.Contains(js, "group: 'Pages'"))
	assert.False(t, strings.Contains(js, "function pages()"))
	assert.False(t, strings.Contains(js, "Searching…")) // the page's own words for it are in a template, which the server wrote
	assert.Contains(t, js, "nothing typed, nothing to find")
	page := serve(searchApp(rowsSearch(), nil), http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, page, `<template data-palette-pending><div class="pal-empty">Searching…</div></template>`)
}
