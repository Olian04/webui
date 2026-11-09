package webui_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// columnPanel is a panel of one line, to put in a column.
func columnPanel(title string) webui.Markdown[handbook] {
	return webui.Markdown[handbook]{
		Title:   title,
		Load:    func(context.Context) (handbook, webui.Loaded) { return handbook{Text: title}, webui.Ready() },
		Content: handbookText,
	}
}

func TestEachColumnHoldsOneBodyInTheOrderTheyAreDeclared(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Columns{
		{Weight: 25, View: columnPanel("Left")},
		{Weight: 50, View: columnPanel("Middle")},
		{Weight: 25, View: columnPanel("Right")},
	})
	// How wide each is, from its weight, is said in the browser suite.
	assert.DeepEqual(t, panels(pageOf(t, h, "/admin/e")), [][2]string{{"p.0", "Left"}, {"p.1", "Middle"}, {"p.2", "Right"}})
}

func TestOneColumnIsStillAColumn(t *testing.T) {
	t.Parallel()

	assert.DeepEqual(t, panels(pageOf(t, editorApp(webui.Columns{{View: columnPanel("Only")}}), "/admin/e")), [][2]string{{"p.0", "Only"}})
}

func TestColumnsGiveTheirPanelsPositionalIDsLikeASplitDoes(t *testing.T) {
	t.Parallel()

	p := pageOf(t, editorApp(webui.Stack{
		webui.Columns{{Weight: 1, View: columnPanel("A")}, {Weight: 2, View: columnPanel("B")}},
		webui.Split{columnPanel("C"), columnPanel("D")},
	}), "/admin/e")
	assert.DeepEqual(t, panels(p), [][2]string{{"p.0.0", "A"}, {"p.0.1", "B"}, {"p.1.0", "C"}, {"p.1.1", "D"}})
}

func TestColumnsNestInAnyLayoutAndTakeAnyBody(t *testing.T) {
	t.Parallel()

	p := pageOf(t, editorApp(webui.Columns{
		{Weight: 1, View: webui.Stack{columnPanel("Top"), columnPanel("Bottom")}},
		{Weight: 3, View: webui.Columns{{View: columnPanel("Inner left")}, {Weight: 2, View: columnPanel("Inner right")}}},
		{Weight: 1, View: webui.Tabs{{Label: "One", View: columnPanel("Tabbed")}}},
		{Weight: 1, View: webui.Split{columnPanel("S1"), columnPanel("S2")}},
	}), "/admin/e")
	var titles []string
	for _, panel := range panels(p) {
		titles = append(titles, panel[1])
	}
	assert.DeepEqual(t, titles, []string{"Top", "Bottom", "Inner left", "Inner right", "Tabbed", "S1", "S2"})
	assert.Equal(t, len(p.ByRole("navigation", "Views: One")), 1)
}

// A layout in a column of a Split is a cell of it like any panel.
func TestColumnsInASplitAndASplitInAColumn(t *testing.T) {
	t.Parallel()

	p := pageOf(t, editorApp(webui.Split{
		webui.Columns{{View: columnPanel("A")}, {Weight: 3, View: columnPanel("B")}},
		webui.Columns{{View: webui.Split{columnPanel("C"), columnPanel("D")}}},
	}), "/admin/e")
	assert.DeepEqual(t, panels(p), [][2]string{{"p.0.0", "A"}, {"p.0.1", "B"}, {"p.1.0.0", "C"}, {"p.1.0.1", "D"}})
}

func TestTheColumnsOfALoadingPageStreamInOrder(t *testing.T) {
	t.Parallel()

	rec := serve(editorApp(webui.Columns{{Weight: 1, View: columnPanel("First")}, {Weight: 1, View: columnPanel("Second")}}), http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.DeepEqual(t, panels(page.Parse(t, rec.Body.String())), [][2]string{{"p.0", "First"}, {"p.1", "Second"}})
}

func TestAColumnsThatCannotBeDrawnIsACompileError(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		body  webui.View
		names []string
	}{
		"none":              {webui.Columns{}, []string{"Columns", "Column"}},
		"nil body":          {webui.Columns{{Weight: 1}, {View: columnPanel("B")}}, []string{"Columns[0].View"}},
		"a bad body inside": {webui.Columns{{View: webui.Markdown[handbook]{Title: "No load"}}}, []string{"Markdown"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			refuses(t, editorErrors(t, tc.body), tc.names...)
		})
	}
}

// A Split is Columns of equal weights: one column of weight 1 for each body, in the order given (their widths are measured in the browser suite).
func TestASplitIsOneColumnForEachBody(t *testing.T) {
	t.Parallel()

	p := pageOf(t, editorApp(webui.Split{columnPanel("A"), columnPanel("B"), columnPanel("C")}), "/admin/e")
	assert.DeepEqual(t, panels(p), [][2]string{{"p.0", "A"}, {"p.1", "B"}, {"p.2", "C"}})
}
