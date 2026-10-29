package webui_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// columnPanel is a panel of one line, to put in a column.
func columnPanel(title string) webui.Markdown[handbook] {
	return webui.Markdown[handbook]{
		Title:   title,
		Load:    func(context.Context) (handbook, webui.Loaded) { return handbook{Text: title}, webui.Ready() },
		Content: handbookText,
	}
}

func TestEachColumnCarriesItsWeightAndHoldsOneBody(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Columns{
		{Weight: 25, Body: columnPanel("Left")},
		{Weight: 50, Body: columnPanel("Middle")},
		{Weight: 25, Body: columnPanel("Right")},
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()

	assert.Contains(t, body, `<div class="columns">`)
	assert.Equal(t, strings.Count(body, `<div class="col" style="--w:`), 3)
	first := strings.Index(body, `<div class="col" style="--w:25;">`)
	second := strings.Index(body, `<div class="col" style="--w:50;">`)
	last := strings.LastIndex(body, `<div class="col" style="--w:25;">`)
	assert.True(t, first >= 0 && first < second && second < last)
	// In the order they are declared, each panel inside the column that has its weight.
	assert.True(t, strings.Index(body, ">Left<") < strings.Index(body, ">Middle<") && strings.Index(body, ">Middle<") < strings.Index(body, ">Right<"))
	assert.Contains(t, body, `<div class="col" style="--w:50;"><div class="panel`)
}

// A weight is relative, so 1 and 3 is 25/75; and zero is 1, which is the zero value of the field.
func TestAWeightIsRelativeAndZeroIsOne(t *testing.T) {
	t.Parallel()

	body := serve(editorApp(webui.Columns{{Weight: 1, Body: columnPanel("A")}, {Weight: 3, Body: columnPanel("B")}}), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `style="--w:1;"`)
	assert.Contains(t, body, `style="--w:3;"`)

	body = serve(editorApp(webui.Columns{{Body: columnPanel("A")}, {Body: columnPanel("B")}, {Weight: 2, Body: columnPanel("C")}}), http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, strings.Count(body, `style="--w:1;"`), 2) // a Column with no Weight is as wide as any other
	assert.Contains(t, body, `style="--w:2;"`)
	assert.False(t, strings.Contains(body, `--w:0`))
}

func TestOneColumnIsStillAColumnThatFillsTheRow(t *testing.T) {
	t.Parallel()

	body := serve(editorApp(webui.Columns{{Body: columnPanel("Only")}}), http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, strings.Count(body, `class="col"`), 1)
}

// The stylesheet is where the sizes are decided: a column grows by its weight from nothing, is never narrower than the floor, and a row that
// cannot hold its columns at that width wraps its last ones, each row sharing by weight in turn. No media query is needed, and none forces a
// single column, since a row too narrow for two columns holds one.
func TestTheStylesheetWrapsColumnsThatWouldBeTooNarrow(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Columns{{Body: columnPanel("A")}})
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, ".columns {\n  --column-min: 280px;\n  display: flex;\n  flex-wrap: wrap;\n  gap: 8px;\n  align-items: stretch;\n}")
	assert.Contains(t, css, ".col {\n  flex: var(--w, 1) 1 0;\n  min-width: min(var(--column-min), 100%);\n  display: flex;\n  flex-direction: column;\n}")
	assert.Contains(t, css, ".col > .panel,\n.col > :is(.stack, .split, .columns, .tabset) {\n  flex: 1 1 auto;\n}")
	// A panel in a column keeps its foot at the bottom of its box, as one in a Split does.
	assert.Contains(t, css, ":is(.split, .stack, .col, [data-tabbody]) > .panel > .panel-body {\n  flex: 1 1 auto;\n}")
	// And no breakpoint collapses the columns: the 1100px rule is a Split's alone.
	assert.Contains(t, css, "@media (max-width: 1100px) {\n  .split {\n    grid-template-columns: 1fr;\n  }\n}")
}

func TestColumnsGiveTheirLeavesPositionalIDsLikeASplitDoes(t *testing.T) {
	t.Parallel()

	body := serve(editorApp(webui.Stack{
		webui.Columns{{Weight: 1, Body: columnPanel("A")}, {Weight: 2, Body: columnPanel("B")}},
		webui.Split{columnPanel("C"), columnPanel("D")},
	}), http.MethodGet, "/admin/e").Body.String()
	for _, id := range []string{`data-leaf="p.0.0"`, `data-leaf="p.0.1"`, `data-leaf="p.1.0"`, `data-leaf="p.1.1"`} {
		assert.Contains(t, body, id)
	}
}

func TestColumnsNestInAnyLayoutAndTakeAnyBody(t *testing.T) {
	t.Parallel()

	body := serve(editorApp(webui.Columns{
		{Weight: 1, Body: webui.Stack{columnPanel("Top"), columnPanel("Bottom")}},
		{Weight: 3, Body: webui.Columns{{Body: columnPanel("Inner left")}, {Weight: 2, Body: columnPanel("Inner right")}}},
		{Weight: 1, Body: webui.Tabs{Panels: []webui.Tab{{Label: "One", Body: columnPanel("Tabbed")}}}},
		{Weight: 1, Body: webui.Split{columnPanel("S1"), columnPanel("S2")}},
	}), http.MethodGet, "/admin/e").Body.String()
	for _, want := range []string{"Top", "Bottom", "Inner left", "Inner right", "Tabbed", "S1", "S2"} {
		assert.Contains(t, body, ">"+want+"<")
	}
	assert.Equal(t, strings.Count(body, `<div class="columns">`), 2)
	assert.Contains(t, body, `class="tabset"`)
}

// A layout in a column of a Split is a cell of it like any panel.
func TestColumnsInASplitAndASplitInAColumn(t *testing.T) {
	t.Parallel()

	body := serve(editorApp(webui.Split{
		webui.Columns{{Body: columnPanel("A")}, {Weight: 3, Body: columnPanel("B")}},
		webui.Columns{{Body: webui.Split{columnPanel("C"), columnPanel("D")}}},
	}), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<div class="split"><div class="columns">`)
	assert.Contains(t, body, `<div class="col" style="--w:1;"><div class="split">`)
}

func TestTheColumnsOfALoadingPageStreamInOrder(t *testing.T) {
	t.Parallel()

	rec := serve(editorApp(webui.Columns{{Weight: 1, Body: columnPanel("First")}, {Weight: 1, Body: columnPanel("Second")}}), http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.True(t, strings.Index(body, ">First<") < strings.Index(body, ">Second<"))
}

func TestAColumnsThatCannotBeDrawnIsACompileError(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		body webui.PageBody
		want string
	}{
		"none":     {webui.Columns{}, "Columns has no Column"},
		"nil body": {webui.Columns{{Weight: 1}, {Body: columnPanel("B")}}, "Columns[0].Body is nil"},
		"a bad body inside": {webui.Columns{{Body: webui.Markdown[handbook]{Title: "No load"}}},
			"Markdown"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, editorErrors(t, tc.body), tc.want)
		})
	}
}
