package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// condition is the model a When decides from.
type condition struct{ Open bool }

var conditionOpen = webui.Bool[condition]{Load: func(c condition) bool { return c.Open }}

// gate is a condition that a test turns on and off, and counts how often it was asked.
type gate struct {
	open  atomic.Bool
	loads atomic.Int32
}

func (g *gate) load(context.Context) (condition, webui.Loaded) {
	g.loads.Add(1)
	return condition{Open: g.open.Load()}, webui.Ready()
}

func (g *gate) when(body webui.View) webui.When[condition] {
	return webui.When[condition]{Load: g.load, Show: conditionOpen, View: body}
}

// stockTable is a table with an export, to put under a When.
func whenTable(title string) webui.Table[flags] {
	return webui.Table[flags]{
		Title: title,
		Loader: webui.RowsLoader[flags]{Load: func(context.Context) ([]flags, webui.Loaded) {
			return []flags{{Name: "a", Compress: true}}, webui.Ready()
		}},
		Columns: []webui.Accessor[flags]{flagName, flagCompress},
		Export:  true,
	}
}

func askView(h http.Handler, target, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("X-Webui-View", id)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAWhenShowsItsBodyOnlyWhileTheConditionHolds(t *testing.T) {
	t.Parallel()

	var g gate
	h := editorApp(webui.Stack{columnPanel("Before"), g.when(columnPanel("Inside")), columnPanel("After")})

	g.open.Store(true)
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, ">Inside<")

	g.open.Store(false)
	rec := serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	body = rec.Body.String()
	assert.False(t, strings.Contains(body, ">Inside<"))
	assert.Contains(t, body, ">Before<")
	assert.Contains(t, body, ">After<")
	// It leaves nothing of its own in the page: the panels either side are as they are, at the positions they were declared at.
	assert.Contains(t, body, `data-view="p.0"`)
	assert.Contains(t, body, `data-view="p.2"`)
}

// A body is where it was declared whether it is shown or not, so its panel is named the same.
func TestThePanelUnderAWhenIsNamedTheSameWhetherOrNotItIsShown(t *testing.T) {
	t.Parallel()

	var g gate
	g.open.Store(true)
	h := editorApp(webui.Stack{columnPanel("Before"), g.when(columnPanel("Inside"))})
	assert.Contains(t, serve(h, http.MethodGet, "/admin/e").Body.String(), `data-view="p.1.0"`)

	// And it is the one that is asked for, as any panel is.
	assert.Equal(t, askView(h, "/admin/e", "p.1.0").Code, http.StatusOK)
	assert.Equal(t, askView(h, "/admin/e", "p.1").Code, http.StatusNotFound) // the When is a layout, and not a panel
}

func TestTheConditionIsAskedAgainOnEveryRequest(t *testing.T) {
	t.Parallel()

	var g gate
	h := editorApp(g.when(columnPanel("Inside")))
	serve(h, http.MethodGet, "/admin/e")
	serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, int(g.loads.Load()), 2)

	// Each time it is what Load read then.
	g.open.Store(true)
	assert.Contains(t, serve(h, http.MethodGet, "/admin/e").Body.String(), ">Inside<")
	g.open.Store(false)
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/e").Body.String(), ">Inside<"))
}

// Show is a Bool like any other, and an accessor is written once: what a form or a table uses it with does not change what a When reads.
func TestAShowThatIsAFullyDeclaredBoolReadsTheSame(t *testing.T) {
	t.Parallel()

	var g gate
	g.open.Store(true)
	full := webui.Bool[condition]{Label: "Open", Help: "Whether it is open.", NoSort: true, NoFilter: true,
		Load: func(c condition) bool { return c.Open }, Store: func(c *condition, v bool) { c.Open = v }}
	h := editorApp(webui.When[condition]{Load: g.load, Show: full, View: columnPanel("Inside")})
	assert.Contains(t, serve(h, http.MethodGet, "/admin/e").Body.String(), ">Inside<")
	g.open.Store(false)
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/e").Body.String(), ">Inside<"))

	// "Show when false" is a Bool that says the opposite.
	g.open.Store(false)
	not := webui.Bool[condition]{Load: func(c condition) bool { return !c.Open }}
	h = editorApp(webui.When[condition]{Load: g.load, Show: not, View: columnPanel("When closed")})
	assert.Contains(t, serve(h, http.MethodGet, "/admin/e").Body.String(), ">When closed<")
}

// What a When hides is out of reach: a refresh, a post and an export are answered as they are for a panel that is not there.
func TestWhatIsHiddenByAWhenCannotBeRefreshedPostedToOrExported(t *testing.T) {
	t.Parallel()

	var g gate
	var saved atomic.Pointer[flags]
	form := webui.Form[flags]{
		Load:   func(context.Context) (flags, webui.Loaded) { return flags{}, webui.Ready() },
		Fields: []webui.Accessor[flags]{flagName},
		Submit: webui.Action[flags]{Run: func(_ context.Context, f flags) (webui.Outcome, error) {
			saved.Store(&f)
			return webui.Success("Saved"), nil
		}},
	}
	h := editorApp(webui.Stack{g.when(webui.Stack{whenTable("Table"), form})})
	table, formID := "p.0.0.0", "p.0.0.1"

	g.open.Store(true)
	assert.Equal(t, askView(h, "/admin/e", table).Code, http.StatusOK)
	assert.Contains(t, serve(h, http.MethodGet, "/admin/e?table.export=csv").Body.String(), "Name,Compress")
	rec := post(h, "/admin/e", url.Values{"_view": {formID}, "f0": {"x"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, saved.Load().Name, "x")

	g.open.Store(false)
	saved.Store(nil)
	assert.Equal(t, askView(h, "/admin/e", table).Code, http.StatusNotFound)
	assert.Equal(t, askView(h, "/admin/e", formID).Code, http.StatusNotFound)
	// An export of a table that is not there is a parameter for one that is not, which is ignored: the page, and not the file.
	exp := serve(h, http.MethodGet, "/admin/e?table.export=csv")
	assert.Equal(t, exp.Code, http.StatusOK)
	assert.False(t, strings.Contains(exp.Body.String(), "Name,Compress"))
	// A post is refused as one to a panel that does not exist is, and nothing is saved.
	hidden := post(h, "/admin/e", url.Values{"_view": {formID}, "f0": {"x"}})
	unknown := post(h, "/admin/e", url.Values{"_view": {"p.9"}, "f0": {"x"}})
	assert.Equal(t, hidden.Code, unknown.Code)
	assert.Equal(t, hidden.Code, http.StatusSeeOther) // back to the page with the message that it was not understood, as for any panel the page does not have
	assert.Equal(t, hidden.Header().Get("Location"), unknown.Header().Get("Location"))
	assert.True(t, saved.Load() == nil)
}

func TestAWhenInAWhenIsAnd(t *testing.T) {
	t.Parallel()

	var outer, inner gate
	h := editorApp(outer.when(inner.when(columnPanel("Both"))))
	for _, tc := range []struct{ outer, inner, shown bool }{{false, false, false}, {true, false, false}, {false, true, false}, {true, true, true}} {
		outer.open.Store(tc.outer)
		inner.open.Store(tc.inner)
		assert.Equal(t, strings.Contains(serve(h, http.MethodGet, "/admin/e").Body.String(), ">Both<"), tc.shown)
	}

	// The inner panel is out of reach while either is closed.
	outer.open.Store(true)
	inner.open.Store(false)
	assert.Equal(t, askView(h, "/admin/e", "p.0.0").Code, http.StatusNotFound)
	inner.open.Store(true)
	assert.Equal(t, askView(h, "/admin/e", "p.0.0").Code, http.StatusOK)
}

// A condition that cannot be decided does not hold: the panel says so, the page stands, and what was under it is out of reach.
func TestAFailingLoadSaysSoAndHidesTheBody(t *testing.T) {
	t.Parallel()

	var shownLoads atomic.Int32
	failing := webui.When[condition]{
		Load: func(context.Context) (condition, webui.Loaded) {
			return condition{}, webui.Failed(errors.New("store secret-bucket is gone"))
		},
		Show: conditionOpen,
		View: webui.Markdown[handbook]{Title: "Inside", Load: func(context.Context) (handbook, webui.Loaded) {
			shownLoads.Add(1)
			return handbook{Text: "x"}, webui.Ready()
		}, Content: handbookText},
	}
	h := editorApp(webui.Stack{columnPanel("Before"), failing, columnPanel("After")})

	rec := serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.Contains(t, body, "Could not load")
	assert.False(t, strings.Contains(body, "secret-bucket")) // the cause is logged, and not shown
	assert.Contains(t, body, ">Before<")
	assert.Contains(t, body, ">After<")
	assert.Equal(t, int(shownLoads.Load()), 0) // the body was never built
	assert.Equal(t, askView(h, "/admin/e", "p.1.0").Code, http.StatusNotFound)
}

func TestAPanicInALoadOrAShowIsRecoveredLikeAnyPanels(t *testing.T) {
	t.Parallel()

	panicking := webui.When[condition]{
		Load: func(context.Context) (condition, webui.Loaded) { panic("the condition fell over") },
		Show: conditionOpen, View: columnPanel("Inside"),
	}
	rec := serve(editorApp(webui.Stack{panicking}), http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusInternalServerError)

	var g gate
	g.open.Store(true)
	bad := webui.When[condition]{Load: g.load, Show: webui.Bool[condition]{Load: func(condition) bool { panic("show fell over") }}, View: columnPanel("Inside")}
	assert.Equal(t, serve(editorApp(webui.Stack{bad}), http.MethodGet, "/admin/e").Code, http.StatusInternalServerError)
}

// The condition is decided first, and the body is built after it: the body's load is in front of nothing, and behind the When's.
func TestTheBodyLoadsAfterTheConditionAndSiblingsStillLoadTogether(t *testing.T) {
	t.Parallel()

	var (
		mu     sync.Mutex
		events []string
	)
	note := func(s string) {
		mu.Lock()
		events = append(events, s)
		mu.Unlock()
	}
	when := webui.When[condition]{
		Load: func(context.Context) (condition, webui.Loaded) {
			time.Sleep(20 * time.Millisecond)
			note("when")
			return condition{Open: true}, webui.Ready()
		},
		Show: conditionOpen,
		View: webui.Table[flags]{Title: "Inside", Columns: []webui.Accessor[flags]{flagName}, Loader: webui.RowsLoader[flags]{Load: func(context.Context) ([]flags, webui.Loaded) { note("body"); return nil, webui.Ready() }}},
	}
	sibling := webui.Table[flags]{Title: "Sibling", Columns: []webui.Accessor[flags]{flagName}, Loader: webui.RowsLoader[flags]{Load: func(context.Context) ([]flags, webui.Loaded) { note("sibling"); return nil, webui.Ready() }}}
	serve(editorApp(webui.Stack{when, sibling}), http.MethodGet, "/admin/e")

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, len(events), 3)
	assert.True(t, indexOf(events, "when") < indexOf(events, "body"))
	assert.True(t, indexOf(events, "sibling") < indexOf(events, "when")) // the sibling did not wait for the condition
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// In a Columns, a column that has nothing is not drawn, and the row wraps as if it were not declared.
func TestAColumnWhoseWhenDoesNotHoldIsEmptyAndNotDrawn(t *testing.T) {
	t.Parallel()

	var g gate
	h := editorApp(webui.Columns{{Weight: 1, View: g.when(columnPanel("Inside"))}, {Weight: 2, View: columnPanel("Next")}})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<div class="col" style="--w:1;"></div>`) // empty, which the stylesheet does not draw
	assert.Contains(t, body, ">Next<")

	css := assetSource(t, "app.css")
	assert.Contains(t, css, ".col:empty {\n  display: none;\n}")
}

// A tab with nothing visible in it is hidden; the others stay, and the first of them is the default.
func TestATabWithNoVisiblePanelIsHidden(t *testing.T) {
	t.Parallel()

	var g gate
	h := editorApp(webui.Tabs{
		{Label: "Maybe", View: g.when(columnPanel("Inside"))},
		{Label: "Always", View: columnPanel("Outside")},
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.False(t, strings.Contains(body, ">Maybe<"))
	assert.False(t, strings.Contains(body, ">Inside<"))
	assert.Contains(t, body, ">Always<")
	assert.Contains(t, body, ">Outside<")

	g.open.Store(true)
	body = serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, ">Maybe<")
	assert.Contains(t, body, ">Inside<")
}

// Only the first declared tab goes without a parameter, so a link to another tab keeps meaning it when the first appears.
func TestTheLinkOfATabKeepsItsParameterWhileTheFirstIsHidden(t *testing.T) {
	t.Parallel()

	var g gate
	h := editorApp(webui.Tabs{
		{Label: "Maybe", View: g.when(columnPanel("Inside"))},
		{Label: "Always", View: columnPanel("Outside")},
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `class="tab active" href="/admin/e?tabs.tab=always"`)

	g.open.Store(true)
	body = serve(h, http.MethodGet, "/admin/e?tabs.tab=always").Body.String()
	assert.Contains(t, body, ">Outside<")
	assert.False(t, strings.Contains(body, ">Inside<"))
}

// The condition of a When in a tab is asked once per request, for whether the tab is shown and for drawing it.
func TestAWhenInATabIsAskedOncePerRequest(t *testing.T) {
	t.Parallel()

	var g gate
	g.open.Store(true)
	h := editorApp(webui.Tabs{{Label: "Maybe", View: g.when(columnPanel("Inside"))}})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, ">Inside<")
	assert.Equal(t, int(g.loads.Load()), 1)
}

func TestTabsWithNoVisibleTabDrawNothing(t *testing.T) {
	t.Parallel()

	var g gate
	h := editorApp(webui.Tabs{{Label: "Maybe", View: g.when(columnPanel("Inside"))}})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.False(t, strings.Contains(body, ">Maybe<"))
	assert.False(t, strings.Contains(body, ">Inside<"))
}

// A layout has nowhere to be refreshed from, so a refresh hint of the Load is accepted and draws nothing.
func TestARefreshHintFromTheLoadIsAcceptedAndDrawsNothing(t *testing.T) {
	t.Parallel()

	when := webui.When[condition]{
		Load: func(context.Context) (condition, webui.Loaded) {
			return condition{Open: true}, webui.Ready().RefreshAfter(5 * time.Second)
		},
		Show: conditionOpen, View: columnPanel("Inside"),
	}
	rec := serve(editorApp(webui.Stack{when}), http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), ">Inside<")
	assert.False(t, strings.Contains(rec.Body.String(), "data-refresh-after"))
}

func TestAWhenThatCannotBeDrawnIsACompileError(t *testing.T) {
	t.Parallel()

	var g gate
	for name, tc := range map[string]struct {
		body webui.View
		want string
	}{
		"no load":                      {webui.When[condition]{Show: conditionOpen, View: columnPanel("A")}, "a When has no Load"},
		"no show load":                 {webui.When[condition]{Load: g.load, View: columnPanel("A")}, "When.Show has no Load"},
		"a Bool with no load":          {webui.When[condition]{Load: g.load, Show: webui.Bool[condition]{Label: "Open"}, View: columnPanel("A")}, "When.Show has no Load"},
		"an OptionalBool with no load": {webui.When[condition]{Load: g.load, Show: webui.OptionalBool[condition]{Label: "Open"}, View: columnPanel("A")}, "When.Show has no Load"},
		"no body":                      {webui.When[condition]{Load: g.load, Show: conditionOpen}, "When.View is nil"},
		"a bad body":                   {webui.When[condition]{Load: g.load, Show: conditionOpen, View: webui.Markdown[handbook]{Title: "No load"}}, "Markdown"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, editorErrors(t, tc.body), tc.want)
		})
	}
}

// The condition's Load is a load like any, counted as one of the kind "when", and a failure of it is counted as one.
func TestTheConditionsLoadIsCountedAsAWhen(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	var g gate
	g.open.Store(true)
	h := webui.App{Observer: observe(reg), Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/w", View: g.when(columnPanel("Inside"))}}}.MustCompile("/admin")
	serve(h, http.MethodGet, "/admin/w")
	assert.Equal(t, gathered(t, reg, "webui_load_duration_seconds")["kind=when,page=/w"], 1.0)
	assert.Equal(t, gathered(t, reg, "webui_load_errors_total")["kind=when,page=/w"], 0.0)

	bad := prometheus.NewRegistry()
	failing := webui.When[condition]{Load: func(context.Context) (condition, webui.Loaded) { return condition{}, webui.Failed(errors.New("down")) }, Show: conditionOpen, View: columnPanel("Inside")}
	h = webui.App{Observer: observe(bad), Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/w", View: failing}}}.MustCompile("/admin")
	serve(h, http.MethodGet, "/admin/w")
	assert.Equal(t, gathered(t, bad, "webui_load_errors_total")["kind=when,page=/w"], 1.0)
}

// A Show may be either kind of yes or no. The tri-state one is shown when it is yes, and a value that is not set is no.
func TestAShowThatIsAnOptionalBoolIsShownOnlyWhenItIsYes(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		value *bool
	)
	set := func(v *bool) {
		mu.Lock()
		value = v
		mu.Unlock()
	}
	when := webui.When[condition]{
		Load: func(context.Context) (condition, webui.Loaded) { return condition{}, webui.Ready() },
		Show: webui.OptionalBool[condition]{Label: "Maybe", Load: func(condition) *bool {
			mu.Lock()
			defer mu.Unlock()
			return value
		}},
		View: columnPanel("Inside"),
	}
	h := editorApp(webui.Stack{when})
	shown := func() bool { return strings.Contains(serve(h, http.MethodGet, "/admin/e").Body.String(), ">Inside<") }

	yes, no := true, false
	set(&yes)
	assert.True(t, shown())
	set(&no)
	assert.False(t, shown())
	set(nil) // not set is no
	assert.False(t, shown())
	assert.Equal(t, askView(h, "/admin/e", "p.0.0").Code, http.StatusNotFound) // and what is under it is out of reach, as for any condition that does not hold
}

// Both accessors are a BoolLike, which is what a place that takes a yes or no of either kind asks for.
func TestBothKindsOfBoolAreABoolLike(t *testing.T) {
	t.Parallel()

	var _ webui.BoolLike[condition] = webui.Bool[condition]{}
	var _ webui.BoolLike[condition] = webui.OptionalBool[condition]{}
}
