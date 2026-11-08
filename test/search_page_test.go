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
	"github.com/Olian04/webui/test/util/page"
)

// timedPages are list pages of things, each with a detail page whose Search takes the given time to answer, with the one result it is
// named for, so that results of different pages are different and each page's time is its own.
func timedPages(wait func(context.Context), names ...string) webui.Pages {
	var pages webui.Pages
	for _, name := range names {
		list := webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs]("/" + name), Nav: webui.Nav{Label: strings.ToUpper(name[:1]) + name[1:]}, View: webui.Stack{}}
		one := webui.Page[detailsArgs]{
			Path: webui.PageID[detailsArgs]("/" + name + "/{id}"), View: webui.Stack{},
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

	declared := webui.Page[detailsArgs]{Path: "/thing/{id}", View: webui.Stack{}, Search: func(context.Context, string, int) ([]webui.Hit[detailsArgs], error) {
		return []webui.Hit[detailsArgs]{{Args: detailsArgs{Id: "a"}, Title: `<img src=x onerror=alert(1)>`, Desc: `<b>bold</b>`}}, nil
	}}
	h := webui.App{Pages: webui.Pages{declared}}.MustCompile("/admin")
	for i, asked := range []http.Handler{searchBox(h), h} { // the box's list, which is a fragment, and the page of results, which has the one script that sets the theme
		body := serve(asked, http.MethodGet, "/admin/_webui/search?q=%3Cscript%3E").Body.String()
		assert.False(t, strings.Contains(body, "<img src=x"))
		assert.False(t, strings.Contains(body, "<b>bold"))
		assert.Equal(t, strings.Count(body, "<script>"), i) // and none that the data brought
		assert.Contains(t, body, "&lt;img src=x onerror=alert(1)&gt;")
	}
}

// With no script the box is a form with a button, and its answer is a page of results, in the shell of the app.
func TestWithoutScriptTheSearchIsAFormWithAButtonAndAnswersWithAPage(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)
	box := page.One(t, "the search form", pageOf(t, h, "/admin/device").ByRole("search", ""))
	q, _ := box.Field("q")
	assert.Equal(t, q.Value, "")
	assert.Equal(t, len(box.ByRole("button", "Search")), 1)
	assert.Equal(t, box.Attr("method"), "get")
	assert.Equal(t, box.Attr("action"), "/admin/_webui/search")

	answer := serve(h, http.MethodGet, "/admin/_webui/search?q=eth")
	assert.Equal(t, answer.Code, http.StatusOK)
	assert.Equal(t, answer.Header().Get("Content-Type"), "text/html; charset=utf-8")
	assert.Equal(t, answer.Header().Get("Cache-Control"), "no-store")
	text := answer.Body.String()
	assert.False(t, page.IsFragment(text)) // a page: the shell, its sidebar and its breadcrumb
	p := page.Parse(t, text)
	assert.True(t, strings.HasPrefix(p.Title(), "Search — "))
	assert.Equal(t, named(sidebarNav(t, p).Links(), "Devices"), 1) // the sidebar of the app is there
	got := trail(t, p)
	assert.Equal(t, got[len(got)-1], "Search")
	assert.Equal(t, len(p.ByRole("heading", "Results for “eth”")), 1)
	typed, _ := p.Field("q")
	assert.Equal(t, typed.Value, "eth") // the box holds what was searched for
	main := page.One(t, "main", p.ByRole("main", ""))
	assert.Equal(t, len(main.Links()), 2)
	assert.Equal(t, main.Links()[0].Href, "/admin/device/dev-eth%200")
	assert.Contains(t, main.Text(), "Devices")      // under the name of the page that found them
	assert.Equal(t, len(p.ByRole("option", "")), 0) // links on a page, not the options of a list

	// Nothing typed, and nothing found, say so.
	assert.Contains(t, serve(h, http.MethodGet, "/admin/_webui/search?q=").Body.String(), "Type what to search for.")
	assert.Contains(t, serve(h, http.MethodGet, "/admin/_webui/search?q=zzzz").Body.String(), "Nothing matches.")
}

// With script the same results are a list for the box, which script puts in it as they are: the options of its listbox.
func TestTheBoxIsGivenTheResultsAsHTMLToPutInIt(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)
	list := serve(searchBox(h), http.MethodGet, "/admin/_webui/search?q=eth").Body.String()
	// A listbox holds options, or groups of them: the label names its group and is not read again.
	p := page.Parse(t, list)
	group := page.One(t, "the Devices group", p.ByRole("group", "Devices"))
	assert.False(t, strings.HasPrefix(group.Text(), "Devices")) // the label is aria-hidden: the group is named, and the name is not part of what it says
	assert.True(t, strings.HasPrefix(group.AllText(), "Devices"))
	assert.Equal(t, group.ByRole("option", "")[0].Attr("href"), "/admin/device/dev-eth%200")
	assert.True(t, page.IsFragment(list))
}
