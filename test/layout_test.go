package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
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
			Load: func(context.Context, webui.Query) (webui.Window[Device], error) {
				loads[i].Add(1)
				return webui.Window[Device]{Items: []Device{{Id: title + "-row"}}, Total: 1}, nil
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
					Load:   func(context.Context) (Device, error) { loads[0].Add(1); return Device{Id: "x"}, nil },
					Fields: []webui.Accessor[Device]{formID},
				},
				table(1, "Events"),
			},
			webui.Tabs{Panels: []webui.Tab{
				{Label: "Overview", Body: table(2, "Overview")},
				{Label: "Raw", Body: table(3, "Raw")},
			}},
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
	assert.Contains(t, body, `class="split"`)
	assert.Contains(t, body, `class="stack"`)
}

// The panels of a Split are as tall as the tallest in their row, and rows are independent: the stylesheet says
// so with a grid that stretches its items, and a panel's body takes the room so its foot stays at the bottom.
func TestAStretchedSplitKeepsItsPanelsFootAtTheBottom(t *testing.T) {
	t.Parallel()

	h, _ := layoutApp(nil)
	assert.Contains(t, serve(h, http.MethodGet, "/admin/device/d1").Body.String(), `<div class="split">`)
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, ".split {\n  display: grid;\n  grid-template-columns: 1fr 1fr;\n  gap: 8px;\n  align-items: stretch;\n}")
	assert.Contains(t, css, ":is(.split, .stack, [data-tabbody]) > .panel > .panel-body {\n  flex: 1 1 auto;\n}")
}

// A column of panels in a Split, and the panel under a tab strip in one, take the height the row has: the tables of a
// column share it, a column without one gives it to its last panel, and the panel in a tab fills what is left of its cell.
func TestAColumnAndATabInASplitFillTheRow(t *testing.T) {
	t.Parallel()

	h, _ := layoutApp(nil)
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, ".stack > .panel.grows {\n  flex: 1 1 auto;\n}")
	assert.Contains(t, css, ".stack:not(:has(> .panel.grows)) > .panel:last-child {\n    flex: 1 1 auto;\n  }")
	assert.Contains(t, css, ".tabset {\n  min-width: 0;\n  display: flex;\n  flex-direction: column;\n}")
	assert.Contains(t, css, "[data-tabbody] > .panel,\n[data-tabbody] > .stack,\n[data-tabbody] > .split {\n  flex: 1 1 auto;\n}")
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
// its children in a grid, does not give the strip a cell of its own.
func TestTabsInASplitAreOneChildOfIt(t *testing.T) {
	t.Parallel()

	h := streamApp(webui.Split{
		webui.Tabs{Panels: []webui.Tab{{Label: "One", Body: quickTable("In tabs")}, {Label: "Two", Body: quickTable("Other")}}},
		quickTable("Beside"),
	})
	body := serve(h, http.MethodGet, "/admin/p").Body.String()
	assert.Contains(t, body, `<div class="split"><div class="tabset"><nav class="tabs"`)
	assert.Equal(t, strings.Count(body, `<div class="tabset">`), 1)
	// The strip and the panel it selects are inside the one element, which ends before the next child begins.
	end := strings.Index(body, `</div></div><div class="panel grows"`)
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
	assert.Contains(t, css, ".sb-opt + .sb-opt::before {\n  content: var(--sep);")
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
// two tables of the same columns, one above the other, would have them in different places. A text takes twice the share of a
// number, a badge, a bar or a moment, and a table that has a selection or row actions leaves those columns to their content.
func TestTheColumnsOfATableAreAsWideAsWhatTheyAreNotAsWhatIsInThem(t *testing.T) {
	t.Parallel()

	// The same columns with different rows, stacked: the same widths.
	h := streamApp(webui.Stack{quickTable("First"), quickTable("A second table with a longer title")})
	body := serve(h, http.MethodGet, "/admin/p").Body.String()
	tight := func(s string) string { return strings.ReplaceAll(s, "> <", "><") }
	colgroups := regexp.MustCompile(`<colgroup>.*?</colgroup>`).FindAllString(tight(body), -1)
	assert.Equal(t, len(colgroups), 2)
	assert.Equal(t, colgroups[0], colgroups[1])
	assert.Equal(t, colgroups[0], `<colgroup><col style="width:100.00%;"></colgroup>`)

	// A text and a number: two thirds and one third.
	mixed := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/m", Body: webui.Table[blob]{
		Title:   "Blobs",
		Rows:    func(context.Context) ([]blob, error) { return blobs(), nil },
		Key:     func(b blob) string { return b.Name },
		Columns: []webui.Accessor[blob]{blobName, blobSize},
	}}}}.MustCompile("/admin")
	assert.Contains(t, tight(serve(mixed, http.MethodGet, "/admin/m").Body.String()), `<colgroup><col style="width:66.67%;"><col style="width:33.33%;"></colgroup>`)
}
