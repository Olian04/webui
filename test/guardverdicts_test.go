package webui_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// A page's Guard is asked for what a request to an address would be: by the request for the page, by the sidebar for each entry, by a breadcrumb, by Cancel and Back,
// and by every link that leads there, which is drawn only for a visitor who may follow it. These tests are what is asked of that: the Guard of a page is run once for an address
// in a request, whoever asks, and whoever asks is told the same.

type verdictArgs struct{ ID int }

// verdictRows is n rows, to be linked from.
func verdictRows(n int) []parcel {
	rows := make([]parcel, n)
	for i := range rows {
		rows[i] = parcel{Name: fmt.Sprintf("row-%02d", i)}
	}
	return rows
}

// guardCount is how many times a Guard ran for each value of ID.
type guardCount struct {
	mu sync.Mutex
	by map[int]int
}

func (c *guardCount) add(id int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.by == nil {
		c.by = map[int]int{}
	}
	c.by[id]++
}

func (c *guardCount) get(id int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.by[id]
}

func (c *guardCount) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.by {
		n += v
	}
	return n
}

func linkRows(n int, dest webui.PageRef[verdictArgs], idOf func(i int) int) webui.Table[parcel] {
	rows := verdictRows(n)
	index := map[string]int{}
	for i, r := range rows {
		index[r.Name] = i
	}
	return webui.Table[parcel]{
		Loader:  webui.RowsLoader[parcel]{Load: func(context.Context) ([]parcel, webui.Loaded) { return rows, webui.Ready() }},
		Key:     func(p parcel) string { return p.Name },
		Columns: []webui.Accessor[parcel]{parcelName},
		RowClick: webui.Link[parcel, verdictArgs]{Page: dest, Args: func(_ context.Context, p parcel) verdictArgs {
			return verdictArgs{ID: idOf(index[p.Name])}
		}},
	}
}

// Twelve rows that lead to three addresses, and a button that leads to one of them, are four Guards run and not thirteen.
func TestAGuardRunsOncePerAddressInARequestHoweverManyLinksLeadThere(t *testing.T) {
	t.Parallel()

	var guard guardCount
	var item webui.Page[verdictArgs]
	item.Path = "/item/{id}"
	item.Body = webui.Stack{}
	item.Guard = func(_ context.Context, a verdictArgs) error { guard.add(a.ID); return nil }

	bar := webui.Toolbar{Left: []webui.ToolbarItem{webui.Link[webui.NoArgs, verdictArgs]{
		Label: "First", Page: &item, Args: func(context.Context, webui.NoArgs) verdictArgs { return verdictArgs{ID: 1} },
	}}}
	list := webui.Page[webui.NoArgs]{Path: "/list", Nav: webui.Nav{Label: "List"}, Body: webui.Stack{bar, linkRows(12, &item, func(i int) int { return 1 + i%3 })}}
	h := webui.App{Pages: webui.Pages{list, item}}.MustCompile("/admin")

	body := serve(h, http.MethodGet, "/admin/list").Body.String()
	assert.Equal(t, strings.Count(body, `href="/admin/item/1"`), 5) // the button, and four rows: each is still drawn
	assert.Equal(t, strings.Count(body, `href="/admin/item/2"`), 4)
	assert.Equal(t, strings.Count(body, `href="/admin/item/3"`), 4)
	for id := 1; id <= 3; id++ {
		assert.Equal(t, guard.get(id), 1)
	}
	assert.Equal(t, guard.total(), 3)
}

// The request for a page has run its Guard, and the sidebar's entry for the page, a link back to it and the entries of the other pages are not each another run of it.
func TestThePageGuardIsNotAskedAgainByTheSidebarOrByALinkBackToIt(t *testing.T) {
	t.Parallel()

	var a, b atomic.Int32
	self := &webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "A"}}
	other := &webui.Page[webui.NoArgs]{Path: "/b", Nav: webui.Nav{Label: "B"}, Body: webui.Stack{}}
	self.Guard = func(context.Context, webui.NoArgs) error { a.Add(1); return nil }
	other.Guard = func(context.Context, webui.NoArgs) error { b.Add(1); return nil }
	to := func(p webui.PageRef[webui.NoArgs]) webui.ToolbarItem {
		return webui.Link[webui.NoArgs, webui.NoArgs]{Label: "Go", Page: p, Args: func(context.Context, webui.NoArgs) webui.NoArgs { return webui.NoArgs{} }}
	}
	self.Body = webui.Stack{webui.Toolbar{Left: []webui.ToolbarItem{to(self), to(other), to(other)}}}
	h := webui.App{Pages: webui.Pages{*self, *other}}.MustCompile("/admin")

	body := serve(h, http.MethodGet, "/admin/a").Body.String()
	assert.Equal(t, strings.Count(body, `href="/admin/b"`), 3) // the entry, and two buttons
	assert.Equal(t, a.Load(), int32(1))                        // the request, and no one after it
	assert.Equal(t, b.Load(), int32(1))                        // the sidebar, the buttons, and nothing else
}

// A Guard whose answer changes in the middle of a request gives a page that agrees with itself: the sidebar's entry for a page, a button that leads there and a link in a row
// are all drawn, or none is, and not an entry for a page whose link is gone. Whoever asks first decides.
func TestAGuardThatChangesItsMindGivesAPageThatAgreesWithItself(t *testing.T) {
	t.Parallel()

	for range 25 {
		var calls atomic.Int32
		var bare webui.Page[webui.NoArgs]
		bare.Path = "/b"
		bare.Nav = webui.Nav{Label: "B"}
		bare.Body = webui.Stack{}
		bare.Guard = func(context.Context, webui.NoArgs) error {
			if calls.Add(1) == 1 {
				return nil
			}
			return errors.New("not any more")
		}
		to := webui.Link[webui.NoArgs, webui.NoArgs]{Label: "Go", Page: &bare, Args: func(context.Context, webui.NoArgs) webui.NoArgs { return webui.NoArgs{} }}
		rows := verdictRows(6)
		table := webui.Table[parcel]{
			Loader:   webui.RowsLoader[parcel]{Load: func(context.Context) ([]parcel, webui.Loaded) { return rows, webui.Ready() }},
			Key:      func(p parcel) string { return p.Name },
			Columns:  []webui.Accessor[parcel]{parcelName},
			RowClick: webui.Link[parcel, webui.NoArgs]{Page: &bare, Args: func(context.Context, parcel) webui.NoArgs { return webui.NoArgs{} }},
		}
		a := webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "A"}, Body: webui.Stack{webui.Toolbar{Left: []webui.ToolbarItem{to, to}}, table}}
		h := webui.App{Pages: webui.Pages{a, bare}}.MustCompile("/admin")

		body := serve(h, http.MethodGet, "/admin/a").Body.String()
		got := strings.Count(body, `href="/admin/b"`)
		if got != 0 && got != 1+2+6 { // the entry, the two buttons and the six rows
			t.Fatalf("the Guard said yes once and no after: the page draws %d of the 9 ways to /b, which is neither none nor all\n%s", got, body)
		}
		assert.Equal(t, calls.Load(), int32(1))
	}
}

// A Guard that panics is the application's bug: asked once, it leaves out every way to its page, and the page that is asked for is answered as it was.
func TestAGuardThatPanicsIsAskedOnceAndEveryLinkToItsPageIsLeftOut(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	var dest webui.Page[verdictArgs]
	dest.Path = "/d/{id}"
	dest.Body = webui.Stack{}
	dest.Guard = func(context.Context, verdictArgs) error {
		calls.Add(1)
		panic("the guard is broken")
	}
	list := webui.Page[webui.NoArgs]{Path: "/list", Body: webui.Stack{linkRows(8, &dest, func(int) int { return 7 })}}
	h := webui.App{Pages: webui.Pages{list, dest}}.MustCompile("/admin")

	rec := serve(h, http.MethodGet, "/admin/list")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.False(t, strings.Contains(rec.Body.String(), `href="/admin/d/7"`))
	assert.Equal(t, calls.Load(), int32(1))

	calls.Store(0)
	assert.Equal(t, serve(h, http.MethodGet, "/admin/d/7").Code, http.StatusInternalServerError) // asked for itself, it is the server error it was
	assert.Equal(t, calls.Load(), int32(1))
}

// A Guard that is slow is asked of every page in the sidebar, and the sidebar waits for the slowest of them and not for the sum.
func TestTheGuardsOfTheSidebarAreAskedAtTheSameTime(t *testing.T) {
	t.Parallel()

	const pages = 6
	var inflight, most atomic.Int32
	arrived := make(chan struct{})
	var once sync.Once
	guard := func(context.Context, webui.NoArgs) error {
		n := inflight.Add(1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		if n == pages {
			once.Do(func() { close(arrived) })
		}
		select {
		case <-arrived: // all of them are in their Guards at once
		case <-time.After(3 * time.Second):
		}
		inflight.Add(-1)
		return nil
	}
	app := webui.Pages{webui.Page[webui.NoArgs]{Path: "/home", Nav: webui.Nav{Label: "Home"}, Body: webui.Stack{}}} // the page that is asked for has no Guard: its own would run first, and alone
	for i := range pages {
		app = append(app, webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs](fmt.Sprintf("/p%d", i)), Nav: webui.Nav{Label: fmt.Sprintf("P%d", i)}, Body: webui.Stack{}, Guard: guard})
	}
	h := webui.App{Pages: app}.MustCompile("/admin")

	started := time.Now()
	body := serve(h, http.MethodGet, "/admin/home").Body.String()
	for i := range pages {
		assert.Contains(t, body, fmt.Sprintf(`href="/admin/p%d"`, i)) // the entries are all there
	}
	assert.True(t, time.Since(started) < 2*time.Second)
	assert.Equal(t, most.Load(), int32(pages))
}
