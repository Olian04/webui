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
		{Weight: 25, View: columnPanel("Left")},
		{Weight: 50, View: columnPanel("Middle")},
		{Weight: 25, View: columnPanel("Right")},
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

	body := serve(editorApp(webui.Columns{{Weight: 1, View: columnPanel("A")}, {Weight: 3, View: columnPanel("B")}}), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `style="--w:1;"`)
	assert.Contains(t, body, `style="--w:3;"`)

	body = serve(editorApp(webui.Columns{{View: columnPanel("A")}, {View: columnPanel("B")}, {Weight: 2, View: columnPanel("C")}}), http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, strings.Count(body, `style="--w:1;"`), 2) // a Column with no Weight is as wide as any other
	assert.Contains(t, body, `style="--w:2;"`)
	assert.False(t, strings.Contains(body, `--w:0`))
}

func TestOneColumnIsStillAColumnThatFillsTheRow(t *testing.T) {
	t.Parallel()

	body := serve(editorApp(webui.Columns{{View: columnPanel("Only")}}), http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, strings.Count(body, `class="col"`), 1)
}

func TestColumnsGiveTheirPanelsPositionalIDsLikeASplitDoes(t *testing.T) {
	t.Parallel()

	body := serve(editorApp(webui.Stack{
		webui.Columns{{Weight: 1, View: columnPanel("A")}, {Weight: 2, View: columnPanel("B")}},
		webui.Split{columnPanel("C"), columnPanel("D")},
	}), http.MethodGet, "/admin/e").Body.String()
	for _, id := range []string{`data-view="p.0.0"`, `data-view="p.0.1"`, `data-view="p.1.0"`, `data-view="p.1.1"`} {
		assert.Contains(t, body, id)
	}
}

func TestColumnsNestInAnyLayoutAndTakeAnyBody(t *testing.T) {
	t.Parallel()

	body := serve(editorApp(webui.Columns{
		{Weight: 1, View: webui.Stack{columnPanel("Top"), columnPanel("Bottom")}},
		{Weight: 3, View: webui.Columns{{View: columnPanel("Inner left")}, {Weight: 2, View: columnPanel("Inner right")}}},
		{Weight: 1, View: webui.Tabs{{Label: "One", View: columnPanel("Tabbed")}}},
		{Weight: 1, View: webui.Split{columnPanel("S1"), columnPanel("S2")}},
	}), http.MethodGet, "/admin/e").Body.String()
	for _, want := range []string{"Top", "Bottom", "Inner left", "Inner right", "Tabbed", "S1", "S2"} {
		assert.Contains(t, body, ">"+want+"<")
	}
	assert.Equal(t, strings.Count(body, `<div class="columns">`), 3) // the outer, the inner, and the Split, which is one
	assert.Contains(t, body, `class="tabset"`)
}

// A layout in a column of a Split is a cell of it like any panel.
func TestColumnsInASplitAndASplitInAColumn(t *testing.T) {
	t.Parallel()

	body := serve(editorApp(webui.Split{
		webui.Columns{{View: columnPanel("A")}, {Weight: 3, View: columnPanel("B")}},
		webui.Columns{{View: webui.Split{columnPanel("C"), columnPanel("D")}}},
	}), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<div class="columns"><div class="col" style="--w:1;"><div class="columns">`)
	assert.Contains(t, body, `<div class="col" style="--w:1;"><div class="columns"><div class="col" style="--w:1;"><div class="panel`)
}

func TestTheColumnsOfALoadingPageStreamInOrder(t *testing.T) {
	t.Parallel()

	rec := serve(editorApp(webui.Columns{{Weight: 1, View: columnPanel("First")}, {Weight: 1, View: columnPanel("Second")}}), http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.True(t, strings.Index(body, ">First<") < strings.Index(body, ">Second<"))
}

func TestAColumnsThatCannotBeDrawnIsACompileError(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		body webui.View
		want string
	}{
		"none":     {webui.Columns{}, "Columns has no Column"},
		"nil body": {webui.Columns{{Weight: 1}, {View: columnPanel("B")}}, "Columns[0].View is nil"},
		"a bad body inside": {webui.Columns{{View: webui.Markdown[handbook]{Title: "No load"}}},
			"Markdown"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, editorErrors(t, tc.body), tc.want)
		})
	}
}

// A Split is Columns of equal weights: one column of weight 1 for each body, in the order given, wrapped by the same rule.
func TestASplitIsOneColumnOfEqualWeightForEachBody(t *testing.T) {
	t.Parallel()

	body := serve(editorApp(webui.Split{columnPanel("A"), columnPanel("B"), columnPanel("C")}), http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, strings.Count(body, `<div class="col" style="--w:1;">`), 3)
	assert.Equal(t, strings.Count(body, `<div class="columns">`), 1)
	assert.True(t, strings.Index(body, ">A<") < strings.Index(body, ">B<") && strings.Index(body, ">B<") < strings.Index(body, ">C<"))
}
