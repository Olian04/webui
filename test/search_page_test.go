package webui_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// timedPages are list pages of things, each with a detail page whose Search takes the given time to answer, with the one result it is
// named for, so that results of different pages are different and each page's time is its own.
func timedPages(wait func(context.Context), names ...string) webui.Pages {
	var pages webui.Pages
	for _, name := range names {
		list := webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs]("/" + name), Nav: webui.Nav{Label: strings.ToUpper(name[:1]) + name[1:]}, Body: webui.Stack{}}
		one := webui.Page[detailsArgs]{
			Path: webui.PageID[detailsArgs]("/" + name + "/{id}"), Body: webui.Stack{},
			Search: func(ctx context.Context, _ string, _ int) ([]webui.Hit[detailsArgs], error) {
				wait(ctx)
				return []webui.Hit[detailsArgs]{{Args: detailsArgs{Id: name}, Title: name}}, nil
			},
		}
		pages = append(pages, list, one)
	}
	return pages
}

// Every page is searched at once, so the answer takes as long as the slowest page and not as long as all of them.
func TestThePagesAreSearchedAtOnceAndNotOneAfterTheOther(t *testing.T) {
	t.Parallel()

	h := webui.App{Pages: timedPages(func(ctx context.Context) {
		select {
		case <-time.After(400 * time.Millisecond):
		case <-ctx.Done():
		}
	}, "alpha", "bravo", "charlie", "delta", "echo")}.MustCompile("/admin")

	started := time.Now()
	got := search(t, h, "a")
	took := time.Since(started)
	assert.Equal(t, len(got.Results), 5)
	assert.True(t, took < 1200*time.Millisecond) // one after the other, five of 400ms would be 2s
}

// A page that is not answered within the time is left out and the rest answer, however it was written: a Search that does not look at its
// context is not waited for. The answer is whole: it is not sent before, so that nothing in it moves.
func TestAPageThatDoesNotAnswerInTimeIsLeftOutAndTheOthersAnswer(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	var mu sync.Mutex
	asked := 0
	pages := timedPages(func(context.Context) {
		mu.Lock()
		asked++
		first := asked == 1
		mu.Unlock()
		if first {
			<-release // the first page asked ignores its context, and never answers
		}
	}, "alpha", "bravo", "charlie")
	h := webui.App{Pages: pages}.MustCompile("/admin")

	started := time.Now()
	got := search(t, h, "a")
	took := time.Since(started)
	assert.True(t, took >= 1500*time.Millisecond && took < 4*time.Second) // the time for a page, and not for ever
	assert.Equal(t, len(got.Results), 2)                                  // the one that never answered is not in it
}

// What was typed and what was found are text, never markup.
func TestSearchResultsAreEscaped(t *testing.T) {
	t.Parallel()

	page := webui.Page[detailsArgs]{Path: "/thing/{id}", Body: webui.Stack{}, Search: func(context.Context, string, int) ([]webui.Hit[detailsArgs], error) {
		return []webui.Hit[detailsArgs]{{Args: detailsArgs{Id: "a"}, Title: `<img src=x onerror=alert(1)>`, Desc: `<b>bold</b>`}}, nil
	}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	for _, target := range []string{"/admin/_webui/search?fragment=1&q=%3Cscript%3E", "/admin/_webui/search?q=%3Cscript%3E"} {
		body := serve(h, http.MethodGet, target).Body.String()
		assert.False(t, strings.Contains(body, "<img src=x"))
		assert.False(t, strings.Contains(body, "<b>bold"))
		assert.False(t, strings.Contains(body, "<script>"))
		assert.Contains(t, body, "&lt;img src=x onerror=alert(1)&gt;")
	}
}

// With no script the box is a form with a button, and its answer is a page of results, in the shell of the app.
func TestWithoutScriptTheSearchIsAFormWithAButtonAndAnswersWithAPage(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, body, `<form class="search" method="get" action="/admin/_webui/search" role="search">`)
	assert.Contains(t, body, `name="q" value=""`)
	assert.Contains(t, body, `<button class="btn btn-secondary btn-sm search-submit" type="submit">Search</button>`)

	page := serve(h, http.MethodGet, "/admin/_webui/search?q=eth")
	assert.Equal(t, page.Code, http.StatusOK)
	assert.Equal(t, page.Header().Get("Content-Type"), "text/html; charset=utf-8")
	assert.Equal(t, page.Header().Get("Cache-Control"), "no-store")
	text := page.Body.String()
	assert.Contains(t, text, "<title>Search — ") // a page: the shell, its sidebar and its breadcrumb
	assert.Contains(t, text, `<nav class="side-scroll"`)
	assert.Contains(t, text, `<span class="cur" aria-current="page">Search</span>`)
	assert.Contains(t, text, `<h2 class="panel-title">Results for “eth”</h2>`)
	assert.Contains(t, text, `name="q" value="eth"`) // the box holds what was searched for
	got := parseResults(text)
	assert.Equal(t, len(got.Results), 2)
	assert.Equal(t, got.Results[0].Group, "Devices")
	assert.Equal(t, got.Results[0].Href, "/admin/device/dev-eth%200")
	assert.False(t, strings.Contains(text, `role="option"`)) // links on a page, not the options of a list

	// Nothing typed, and nothing found, say so.
	assert.Contains(t, serve(h, http.MethodGet, "/admin/_webui/search?q=").Body.String(), "Type what to search for.")
	assert.Contains(t, serve(h, http.MethodGet, "/admin/_webui/search?q=zzzz").Body.String(), "Nothing matches.")
}

// With script the same results are a list for the box, which script puts in it as they are: the options of its listbox.
func TestTheBoxIsGivenTheResultsAsHTMLToPutInIt(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)
	list := serve(h, http.MethodGet, "/admin/_webui/search?fragment=1&q=eth").Body.String()
	assert.Contains(t, list, `<div class="pal-label">Devices</div>`)
	assert.Contains(t, list, `<a class="pal-item" role="option" href="/admin/device/dev-eth%200">`)
	assert.False(t, strings.Contains(list, "<html"))

	js := serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Body.String()
	for _, want := range []string{"'?fragment=1&q='", "box.innerHTML = palette.html", "text/html", "palette.state !== 'loading'"} {
		assert.Contains(t, js, want)
	}
	assert.False(t, strings.Contains(js, "res.json()")) // it is not JSON any more
}
