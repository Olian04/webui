package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type layoutArgs struct {
	Id string
	Q  string
}

// layoutApp is the README's nested body: a Split of a form and a table, then
// Tabs. It counts loads so a test can prove which leaves were read.
func layoutApp(guard func(context.Context, layoutArgs) error) (http.Handler, *[4]atomic.Int32) {
	var loads [4]atomic.Int32
	table := func(i int, title string) webui.Table[Device] {
		return webui.Table[Device]{
			Title: title,
			Load: func(context.Context, webui.Query) (webui.Window[Device], webui.Loaded) {
				loads[i].Add(1)
				return webui.Window[Device]{Items: []Device{{Id: title + "-row"}}, Total: 1}, webui.Ready()
			},
			Columns: []webui.Accessor[Device]{formID},
		}
	}
	page := webui.Page[layoutArgs]{
		Path:  "/device/{id}",
		Guard: guard,
		Body: webui.Stack{
			webui.Split{
				webui.Form[Device]{
					Title:  "Form",
					Load:   func(context.Context) (Device, webui.Loaded) { loads[0].Add(1); return Device{Id: "x"}, webui.Ready() },
					Fields: []webui.Accessor[Device]{formID},
				},
				table(1, "Events"),
			},
			webui.Tabs{
				{Label: "Overview", Body: table(2, "Overview")},
				{Label: "Raw", Body: table(3, "Raw")},
			},
		},
	}
	return webui.App{Pages: webui.Pages{page}}.MustCompile("/admin"), &loads
}

func TestTabsLoadOnlyTheSelectedPanel(t *testing.T) {
	t.Parallel()

	h, loads := layoutApp(nil)

	body := serve(h, http.MethodGet, "/admin/device/d1").Body.String()
	assert.Contains(t, body, "Overview-row")
	assert.False(t, strings.Contains(body, "Raw-row"))
	assert.Equal(t, loads[2].Load(), int32(1))
	assert.Equal(t, loads[3].Load(), int32(0)) // the hidden tab was never read

	// The strip is links; the first needs no argument, the others carry theirs.
	assert.Contains(t, body, `<a class="tab active" href="/admin/device/d1" aria-current="page">Overview</a>`)
	assert.Contains(t, body, `<a class="tab" href="/admin/device/d1?tabs.tab=raw">Raw</a>`)

	raw := serve(h, http.MethodGet, "/admin/device/d1?tabs.tab=raw").Body.String()
	assert.Contains(t, raw, "Raw-row")
	assert.False(t, strings.Contains(raw, "Overview-row"))
	assert.Equal(t, loads[3].Load(), int32(1))
	assert.Contains(t, raw, `<a class="tab active" href="/admin/device/d1?tabs.tab=raw" aria-current="page">Raw</a>`)

	// An unknown tab lands on the first rather than on nothing.
	stale := serve(h, http.MethodGet, "/admin/device/d1?tabs.tab=Gone").Body.String()
	assert.Contains(t, stale, "Overview-row")
}

func TestLayoutsGiveLeavesPositionalIDs(t *testing.T) {
	t.Parallel()

	h, _ := layoutApp(nil)
	body := serve(h, http.MethodGet, "/admin/device/d1").Body.String()
	for _, id := range []string{`data-leaf="p.0.0"`, `data-leaf="p.0.1"`, `data-leaf="p.1.0"`} {
		assert.Contains(t, body, id)
	}
	assert.Contains(t, body, `class="columns"`)
	assert.Contains(t, body, `class="stack"`)
}

// A Split is Columns of equal weights: it has no markup or rule of its own, so its panels are as tall as the tallest in their row,
// and rows are independent, by the rule of Columns, and a panel's body takes the room so its foot stays at the bottom.
func TestASplitIsColumnsAndKeepsItsPanelsFootAtTheBottom(t *testing.T) {
	t.Parallel()

	h, _ := layoutApp(nil)
	body := serve(h, http.MethodGet, "/admin/device/d1").Body.String()
	assert.Contains(t, body, `<div class="columns"><div class="col" style="--w:1;">`)
	assert.False(t, strings.Contains(body, `class="split"`))
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.False(t, strings.Contains(css, ".split"))
	assert.Contains(t, css, ":is(.stack, .col, [data-tabbody]) > .panel > .panel-body {\n  flex: 1 1 auto;\n}")
}

// A column of panels in a Split, and the panel under a tab strip in one, take the height the row has: the tables of a
// column share it, a column without one gives it to its last panel, and the panel in a tab fills what is left of its cell.
func TestAColumnAndATabInAColumnsFillTheRow(t *testing.T) {
	t.Parallel()

	h, _ := layoutApp(nil)
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, ".stack > .panel.grows {\n  flex: 1 1 auto;\n}")
	assert.Contains(t, css, ".stack:not(:has(> .panel.grows)) > .panel:last-child {\n    flex: 1 1 auto;\n  }")
	assert.Contains(t, css, ".tabset {\n  min-width: 0;\n  display: flex;\n  flex-direction: column;\n}")
	assert.Contains(t, css, "[data-tabbody] > .panel,\n[data-tabbody] > .stack,\n[data-tabbody] > .columns {\n  flex: 1 1 auto;\n}")
	assert.Contains(t, css, ".tabset[aria-busy='true'] > [data-tabbody] {\n  opacity: 0.55;")
}

func TestLeafRequestReturnsOnlyThatPanel(t *testing.T) {
	t.Parallel()

	h, loads := layoutApp(nil)
	req := httptest.NewRequest(http.MethodGet, "/admin/device/d1", nil)
	req.Header.Set("X-Webui-Leaf", "p.0.1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.True(t, strings.HasPrefix(body, `<div class="panel grows" data-leaf="p.0.1">`))
	assert.False(t, strings.Contains(body, "<html"))
	assert.Contains(t, body, "Events-row")
	assert.Contains(t, rec.Header().Get("Vary"), "X-Webui-Leaf")
	assert.Equal(t, rec.Header().Get("Cache-Control"), "no-store")
	// Only that leaf was read; the rest of the screen holds still.
	assert.Equal(t, loads[0].Load(), int32(0))
	assert.Equal(t, loads[1].Load(), int32(1))
	assert.Equal(t, loads[2].Load(), int32(0))

	// A full page says it varies on the header too, so no cache serves one for the other.
	assert.Contains(t, serve(h, http.MethodGet, "/admin/device/d1").Header().Get("Vary"), "X-Webui-Leaf")
}

func TestLeafRequestStillRunsThePageGuardAndRejectsUnknownIDs(t *testing.T) {
	t.Parallel()

	var denied bool
	h, loads := layoutApp(func(context.Context, layoutArgs) error {
		if denied {
			return errors.New("not for you")
		}
		return nil
	})

	ask := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/admin/device/d1", nil)
		req.Header.Set("X-Webui-Leaf", id)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	assert.Equal(t, ask("p.9").Code, http.StatusNotFound)
	assert.Equal(t, ask("p.0").Code, http.StatusNotFound) // a layout is not a leaf
	assert.Equal(t, ask("../../etc").Code, http.StatusNotFound)

	denied = true
	rec := ask("p.0.1") // the leaf URL is the page URL: the same Guard runs first
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Contains(t, rec.Body.String(), "not for you")
	assert.Equal(t, loads[1].Load(), int32(0))
}

func TestOnlyATablePanelGrowsToTheHeightThereIsAndAFormKeepsItsOwn(t *testing.T) {
	t.Parallel()

	h, _ := layoutApp(nil)
	body := serve(h, http.MethodGet, "/admin/device/d1").Body.String()
	// The table's panel is the one that scrolls inside itself; the form's is not.
	assert.Contains(t, body, `<div class="panel grows" data-leaf="p.0.1">`)
	assert.Contains(t, body, `<div class="panel" data-leaf="p.0.0">`)
}

// Tabs is one child of the Split it is in: the strip and the selected panel are one element, so a Split, which puts
// its children in columns, does not give the strip a column of its own.
func TestTabsInASplitAreOneChildOfIt(t *testing.T) {
	t.Parallel()

	h := streamApp(webui.Split{
		webui.Tabs{{Label: "One", Body: quickTable("In tabs")}, {Label: "Two", Body: quickTable("Other")}},
		quickTable("Beside"),
	})
	body := serve(h, http.MethodGet, "/admin/p").Body.String()
	assert.Contains(t, body, `<div class="columns"><div class="col" style="--w:1;"><div class="tabset" data-tabs="p.0"><nav class="tabs"`)
	assert.Equal(t, strings.Count(body, `<div class="tabset" `), 1)
	// The strip and the panel it selects are inside the one element, which ends before the next child begins.
	end := strings.Index(body, `</div></div><div class="col" style="--w:1;"><div class="panel grows"`)
	assert.True(t, end > strings.Index(body, "In tabs-row"))
	assert.True(t, strings.Index(body, "Beside-row") > end)
}

// The sidebar as a rail has no room for a section's caption: it is a short, thick dash between the groups, in both places the
// stylesheet makes a rail, with the caption as the tooltip of the element. It is not a line the width of the rail, which is
// the edge under the brand, and not the middle dot (--sep), which separates the figures of the status bar.
func TestTheRailSeparatesItsGroupsWithAShortDashAndTheStatusBarItsFiguresWithADot(t *testing.T) {
	t.Parallel()

	h, _ := layoutApp(nil)
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	dash := "content: '';\n  display: block;\n  width: 32px;\n  height: 3px;\n  margin: 9px auto 0;\n  border-radius: 2px;\n  background: var(--border-2);"
	assert.Contains(t, css, "html[data-sidebar='collapsed'] .nav-section::before {\n  "+dash)
	assert.Contains(t, css, "  .nav-section::before {\n    "+strings.ReplaceAll(dash, "\n  ", "\n    "))
	assert.Contains(t, css, "html[data-sidebar='collapsed'] .side-scroll > .nav-section:first-child {\n  display: none;\n}")
	assert.False(t, strings.Contains(css, "linear-gradient(var(--border-2), var(--border-2))"))
	assert.Contains(t, css, "--sep: '\\00b7';")
	assert.Contains(t, css, ".sb-timing + .sb-opt::before {\n  content: var(--sep);")
}

// A diff names its two sides above it. In one column, which it is without script and when its panel is narrow, they are
// not sides but a change, and the header says "Deployed → Saved"; only a diff that is side by side has a name over each
// side. The script says which it is on the panel's body (data-diff).
func TestADiffNamesItsSidesOnlyWhenItIsSideBySide(t *testing.T) {
	t.Parallel()

	h, _ := layoutApp(nil)
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, ".code-sides span + span::before {\n  content: '\\2192';")
	assert.Contains(t, css, ".code-body[data-diff='split'] .code-sides span {\n  flex: 1;\n}")
	assert.Contains(t, css, ".code-body[data-diff='split'] .code-sides span + span::before {\n  content: none;\n}")
}

// The columns of a table are as wide as what they are, and not as what is in them: a table sizes its columns by its rows, so
// two tables of the same columns, one above the other, would have them in different places. Each column is given the room
// it needs from its declaration, and the table is shared in those proportions.
func TestTheColumnsOfATableAreAsWideAsWhatTheyAreNotAsWhatIsInThem(t *testing.T) {
	t.Parallel()

	widths := func(body string) [][]float64 {
		tight := strings.ReplaceAll(body, "> <", "><")
		var out [][]float64
		for _, group := range regexp.MustCompile(`<colgroup>.*?</colgroup>`).FindAllString(tight, -1) {
			var ws []float64
			for _, m := range regexp.MustCompile(`width:([0-9.]+)%`).FindAllStringSubmatch(group, -1) {
				f, err := strconv.ParseFloat(m[1], 64)
				assert.NoError(t, err)
				ws = append(ws, f)
			}
			out = append(out, ws)
		}
		return out
	}

	// The same columns with different rows, stacked: the same widths.
	same := widths(serve(streamApp(webui.Stack{quickTable("First"), quickTable("A second table with a longer title")}), http.MethodGet, "/admin/p").Body.String())
	assert.Equal(t, len(same), 2)
	assert.DeepEqual(t, same[0], same[1])
	assert.DeepEqual(t, same[0], []float64{100})

	// A column of badges that can hold several, with the room for two of them, is wider than one of a single value, which is
	// wider than the header it has; a number is not given more than it needs; and the shares are the whole of the table.
	tags := webui.Select[blob]{Label: "Tags", Load: func(blob) []string { return nil }, Multi: true,
		Options: []webui.Option{{Value: "edge"}, {Value: "gpu"}, {Value: "legacy"}, {Value: "staging"}}}
	state := webui.Select[blob]{Label: "State", Load: func(blob) []string { return nil },
		Options: []webui.Option{{Value: "healthy"}, {Value: "quiet"}, {Value: "degraded"}}}
	mixed := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/m", Body: webui.Table[blob]{
		Title:   "Blobs",
		Rows:    func(context.Context) ([]blob, webui.Loaded) { return blobs(), webui.Ready() },
		Key:     func(b blob) string { return b.Name },
		Columns: []webui.Accessor[blob]{blobName, state, tags, blobSize},
	}}}}.MustCompile("/admin")
	got := widths(serve(mixed, http.MethodGet, "/admin/m").Body.String())[0]
	assert.Equal(t, len(got), 4)
	name, st, multi, size := got[0], got[1], got[2], got[3]
	assert.True(t, multi > st)   // a column of several badges is wider than a column of one
	assert.True(t, multi > name) // and than a text
	assert.True(t, size < name)  // a number is not given more than a text
	assert.True(t, st > size)    // and a badge is given more than a number
	assert.True(t, name+st+multi+size > 99.9 && name+st+multi+size < 100.1)
}

func serveLeaf(h http.Handler, target, leaf string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("X-Webui-Leaf", leaf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// A tab is followed, with script, by asking for the Tabs alone: its strip and the tab that is selected. Nothing else of the
// page is read, which is the point: a tab is not a new page, and the panels around it have nothing new to say.
func TestATabsIsAskedForAloneAndOnlyTheSelectedTabIsRead(t *testing.T) {
	t.Parallel()

	h, loads := layoutApp(nil)
	rec := serveLeaf(h, "/admin/device/d1?tabs.tab=raw", "p.1")
	body := rec.Body.String()

	assert.Equal(t, rec.Code, http.StatusOK)
	assert.True(t, strings.HasPrefix(body, `<div class="tabset" data-tabs="p.1">`))
	assert.Contains(t, body, "Raw-row")
	assert.Contains(t, body, `<a class="tab active" href="/admin/device/d1?tabs.tab=raw" aria-current="page">Raw</a>`)
	assert.False(t, strings.Contains(body, "Overview-row"))
	assert.False(t, strings.Contains(body, `class="sidebar"`)) // not the page
	for i, want := range []int32{0, 0, 0, 1} {                 // the form, the events, and the other tab were not read
		assert.Equal(t, loads[i].Load(), want)
	}

	// The page carries what script needs to ask: the id, and the argument that selects a tab.
	page := serve(h, http.MethodGet, "/admin/device/d1").Body.String()
	assert.Contains(t, page, `<div class="tabset" data-tabs="p.1">`)

	// It is the page's own request: the Guard runs, and an id that is not a Tabs or a panel is a 404.
	refused, _ := layoutApp(func(context.Context, layoutArgs) error { return errors.New("no") })
	assert.Equal(t, serveLeaf(refused, "/admin/device/d1", "p.1").Code, http.StatusForbidden)
	assert.Equal(t, serveLeaf(h, "/admin/device/d1", "p.9").Code, http.StatusNotFound)
}

// A Tabs has as many tabs as it is given, and a strip that has more than fit scrolls sideways with its tabs at their own size:
// they do not shrink or wrap, and the rule under the strip does not depend on a border that a scrolling box would clip.
func TestAStripOfManyTabsScrollsSidewaysAndKeepsTheSizeOfItsTabs(t *testing.T) {
	t.Parallel()

	panels := make([]webui.Tab, 20)
	for i := range panels {
		panels[i] = webui.Tab{Label: "A tab with a label number " + strconv.Itoa(i), Body: quickTable("Panel " + strconv.Itoa(i))}
	}
	h := streamApp(webui.Tabs(panels))
	body := serve(h, http.MethodGet, "/admin/p?tabs.tab=a-tab-with-a-label-number-19").Body.String()
	assert.Equal(t, strings.Count(body, `<a class="tab`), 20)
	assert.Contains(t, body, `<a class="tab active" href="/admin/p?tabs.tab=a-tab-with-a-label-number-19" aria-current="page">`)
	assert.Contains(t, body, "Panel 19-row")
	assert.False(t, strings.Contains(body, "Panel 0-row"))

	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, "box-shadow: inset 0 -1px 0 var(--border);")
	assert.Contains(t, css, "overflow-x: auto;\n  overflow-y: hidden;")
	assert.Contains(t, css, ".tab {\n  flex: none;")
	assert.Contains(t, css, "white-space: nowrap;\n  color: var(--text-2);")
}

// A followed link is an ordinary navigation. The page does not opt in to a cross-document view transition, whose first frames
// are white whatever the page says its background is: neither the opt-in nor a name for a region of the shell is in the stylesheet.
func TestThePageDoesNotOptInToACrossDocumentViewTransition(t *testing.T) {
	t.Parallel()

	h := streamApp(webui.Stack{quickTable("First")})
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.False(t, strings.Contains(css, "@view-transition"))
	assert.False(t, strings.Contains(css, "view-transition-name"))
	assert.False(t, strings.Contains(css, "::view-transition"))
}

// When the HTML was built is shown where script is off, and with script it is not: the page says "stale page" when it is
// wrong for the request, and nothing else a reader would use it for. The stylesheet hides it under the class that script
// sets before first paint, so it never shows and goes.
func TestWhenThePageWasBuiltIsShownOnlyWhereScriptIsOffAndTheBadgeSaysWhenItIsStale(t *testing.T) {
	t.Parallel()

	h := streamApp(webui.Stack{quickTable("First")})
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, "html.js .status-meta [data-built] {\n  display: none;\n}")

	page := serve(h, http.MethodGet, "/admin/p").Body.String()
	assert.Contains(t, page, `<time class="sb-opt" data-built datetime="`) // the server says it, for the page without script
	assert.Contains(t, page, `data-stale hidden`)                          // and the badge is there for script to show

	js := serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Body.String()
	assert.Contains(t, js, "'stale page'")
	assert.False(t, strings.Contains(js, "cached?"))
	assert.False(t, strings.Contains(js, "showBuilt")) // nothing writes the time or its age into the bar
}

// The render time and the delivery time are not two measures of one thing: the page is loaded when the request has been
// answered and shown, which the server's time to build it is part of. The bar says each by name, with the separator between
// them, and the script works out the delivery as the rest. Without script only the render time is known.
func TestTheStatusBarSaysWhatTheRenderTimeAndTheDeliveryTimeAre(t *testing.T) {
	t.Parallel()

	h := streamApp(webui.Stack{quickTable("First")})
	page := serve(h, http.MethodGet, "/admin/p").Body.String()
	assert.Contains(t, page, `<span class="sb-timing" data-timing>`)
	assert.True(t, regexp.MustCompile(`<span class="sb-opt" data-build title="[^"]*">\d+(\.\d)? ms</span>`).MatchString(page))
	assert.Contains(t, page, `<span class="sb-opt" data-loaded hidden></span>`) // for script to fill in
	assert.False(t, strings.Contains(page, "sb-split"))                         // no bar: the numbers say it

	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, ".sb-timing [data-build]::before {\n  content: 'render ';")
	assert.Contains(t, css, ".sb-timing [data-loaded]::before {\n  content: var(--sep) ' delivery ';")
	assert.False(t, strings.Contains(css, "sb-split"))

	js := serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Body.String()
	assert.Contains(t, js, "Math.max(ms - render, 0)") // the delivery is what is left of the whole
	assert.False(t, strings.Contains(js, "data-split"))
}

// Two strips of tabs on one page (side by side, or one in the tab of another) are two navigations, and a reader who lists the landmarks tells them
// apart by their names, which are "Views" and what the first tabs are called.
func TestEachStripOfTabsHasItsOwnName(t *testing.T) {
	t.Parallel()

	h := streamApp(webui.Split{
		webui.Tabs{{Label: "One", Body: quickTable("A")}, {Label: "Two", Body: quickTable("B")}},
		webui.Tabs{{Label: "Red", Body: quickTable("C")}, {Label: "Green", Body: quickTable("D")}, {Label: "Blue", Body: quickTable("E")}, {Label: "Grey", Body: quickTable("F")}},
	})
	body := serve(h, http.MethodGet, "/admin/p").Body.String()
	assert.Contains(t, body, `<nav class="tabs" aria-label="Views: One, Two">`)
	assert.Contains(t, body, `<nav class="tabs" aria-label="Views: Red, Green, Blue, …">`)
}
