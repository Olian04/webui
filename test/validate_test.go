package webui_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type okArgs struct {
	Id string
}

func okRows(context.Context, webui.Query) (webui.Rows[Device], error) {
	return webui.Rows[Device]{}, nil
}

func idCol() webui.Accessor[Device] {
	return webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}
}

// compileErrors compiles and returns the structured problems.
func compileErrors(t *testing.T, app webui.App) webui.CompileErrors {
	t.Helper()

	_, err := app.Compile("/admin")
	var errs webui.CompileErrors
	if !errors.As(err, &errs) {
		t.Fatalf("Compile error = %v, want CompileErrors", err)
	}
	return errs
}

func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	t.Parallel()

	type bad struct {
		When []string
		A    string `webui:"x"`
		B    string `webui:"x"`
	}
	app := webui.App{Pages: webui.Pages{
		webui.Page[bad]{Path: "/event/{id}", Body: webui.Table[Device]{Load: okRows, Columns: []webui.Accessor[Device]{idCol()}}},
	}}
	errs := compileErrors(t, app)
	assert.Equal(t, len(errs), 3)
	assert.Equal(t, errs[0].Error(), `page "/event/{id}" (bad): field When has unsupported type []string
  Fix: Arguments must be string, bool, int, int64 or float64 — a URL carries one value per name.`)
	assert.Contains(t, errs[1].Detail, `two fields both map to the argument "x"`)
	assert.Equal(t, errs[2].Detail, "the path declares {id} but bad has no field for it")
}

func TestValidateLinkToUnmountedPage(t *testing.T) {
	t.Parallel()

	orphan := webui.Page[DetailsArgs]{Path: "/never-mounted/{id}", Body: webui.Stack{}}
	app := webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/broken", Body: webui.Stack{
			webui.Table[Device]{
				Load:    okRows,
				Columns: []webui.Accessor[Device]{idCol()},
				RowClick: webui.Link[Device, DetailsArgs]{
					Page: orphan,
					Args: func(context.Context, Device) DetailsArgs { return DetailsArgs{} },
				},
			},
		}},
	}}
	errs := compileErrors(t, app)
	assert.Equal(t, len(errs), 1)
	assert.Equal(t, errs[0].Error(), `page "/broken" (NoArgs): a link targets "/never-mounted/{id}", which is not mounted in this app
  Fix: Add that page to App.Pages, or point the link at a page that is already there.`)
}

func TestValidateDeclarationChecks(t *testing.T) {
	t.Parallel()

	badPattern := webui.String[Device]{
		Label: "IP", Load: func(d Device) string { return d.Ip },
		Rules: webui.StringRules{Pattern: &webui.PatternRule{Expr: `(?=x)`}},
	}
	dupLabel := []webui.Accessor[Device]{idCol(), idCol()}
	group := webui.Group[Device]{idCol()}

	tests := []struct {
		name string
		page webui.Page[okArgs]
		want string
	}{
		{"nil body", webui.Page[okArgs]{Path: "/a"}, "Body is nil"},
		{"relative path", webui.Page[okArgs]{Path: "a", Body: webui.Stack{}}, `Path must start with "/"`},
		{"reserved path", webui.Page[okArgs]{Path: "/_webui/x", Body: webui.Stack{}}, "reserved prefix"},
		{"form without load", webui.Page[okArgs]{Path: "/a", Body: webui.Form[Device]{}}, "a Form has no Load"},
		{"RE2", webui.Page[okArgs]{Path: "/a", Body: webui.Form[Device]{
			Load: func(context.Context) (Device, error) { return Device{}, nil }, Fields: []webui.Accessor[Device]{badPattern},
		}}, "RE2"},
		{"duplicate form labels", webui.Page[okArgs]{Path: "/a", Body: webui.Form[Device]{
			Load: func(context.Context) (Device, error) { return Device{}, nil }, Fields: dupLabel,
		}}, `label "ID" is used twice`},
		{"group in columns", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{
			Load: okRows, Columns: []webui.Accessor[Device]{group},
		}}, "Group in Table.Columns"},
		{"actions need key", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{
			Load: okRows, Columns: []webui.Accessor[Device]{idCol()},
			Actions: []webui.Action[Device]{{Label: "x", Run: func(context.Context, Device) (webui.Effect, error) { return webui.Effect{}, nil }}},
		}}, "declares actions but no Key"},
		{"stateful table needs a unique ID", webui.Page[okArgs]{Path: "/a", Body: webui.Stack{
			webui.Table[Device]{Load: okRows, PageSize: 10},
			webui.Table[Device]{Load: okRows, PageSize: 10},
		}}, `the ID "table" is used twice on this page`},
		{"a clash tells you to name one", webui.Page[okArgs]{Path: "/a", Body: webui.Stack{
			webui.Table[Device]{Load: okRows, PageSize: 10},
			webui.Table[Device]{Load: okRows, PageSize: 10},
		}}, `Set ID on this Table`},
		{"explicit duplicate", webui.Page[okArgs]{Path: "/a", Body: webui.Stack{
			webui.Table[Device]{ID: "devices", Load: okRows, PageSize: 10},
			webui.Table[Device]{ID: "devices", Load: okRows, PageSize: 10},
		}}, "Give each table and tabs its own ID"},
		{"every table keeps its sort, so every table claims an ID", webui.Page[okArgs]{Path: "/a", Body: webui.Stack{
			webui.Table[Device]{Load: okRows},
			webui.Table[Device]{Load: okRows},
		}}, `the ID "table" is used twice`},
		{"two columns sorting by one key", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{
			Load: okRows, Columns: []webui.Accessor[Device]{idCol(), webui.String[Device]{Label: "Other", Key: "ID", Load: func(Device) string { return "" }}},
		}}, `two columns sort by "ID"`},
		{"two columns with one label", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{
			Load: okRows, Columns: []webui.Accessor[Device]{idCol(), idCol()},
		}}, `two columns sort by "ID"`},
		{"same ID in one panel clashes", webui.Page[okArgs]{Path: "/a", Body: webui.Tabs{ID: "v", Panels: []webui.Tab{
			{Label: "A", Body: webui.Stack{
				webui.Table[Device]{ID: "t", Load: okRows},
				webui.Table[Device]{ID: "t", Load: okRows},
			}},
		}}}, `the ID "t" is used twice`},
		{"same ID in a panel and outside the tabs clashes", webui.Page[okArgs]{Path: "/a", Body: webui.Stack{
			webui.Table[Device]{ID: "t", Load: okRows},
			webui.Tabs{ID: "v", Panels: []webui.Tab{{Label: "A", Body: webui.Table[Device]{ID: "t", Load: okRows}}}},
		}}, `the ID "t" is used twice`},
		{"same ID in two different Tabs clashes", webui.Page[okArgs]{Path: "/a", Body: webui.Stack{
			webui.Tabs{ID: "v1", Panels: []webui.Tab{{Label: "A", Body: webui.Table[Device]{ID: "t", Load: okRows}}}},
			webui.Tabs{ID: "v2", Panels: []webui.Tab{{Label: "A", Body: webui.Table[Device]{ID: "t", Load: okRows}}}},
		}}, `the ID "t" is used twice`},
		{"ID must be a plain word", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{
			ID: "Dev.ices", Load: okRows, PageSize: 10,
		}}, "is not a lower-case word"},
		{"negative page size", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{Load: okRows, PageSize: -1}}, "PageSize is negative"},
		{"tabs and a table may not share an ID", webui.Page[okArgs]{Path: "/a", Body: webui.Stack{
			webui.Tabs{ID: "view", Panels: []webui.Tab{{Label: "A", Body: webui.Stack{}}}},
			webui.Table[Device]{ID: "view", Load: okRows, PageSize: 10},
		}}, `the ID "view" is used twice`},
		{"slider needs a range", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{
			Load: okRows, Columns: []webui.Accessor[Device]{webui.Slider[Device]{Label: "Rate", Load: func(Device) float64 { return 0 }}},
		}}, "has no range"},
		{"slider range must be ascending", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{
			Load: okRows, Columns: []webui.Accessor[Device]{webui.Slider[Device]{Label: "Rate", Min: 5, Max: 5, Load: func(Device) float64 { return 0 }}},
		}}, "has no range"},
		{"badge needs a load", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{
			Load: okRows, Columns: []webui.Accessor[Device]{webui.Badge[Device]{Label: "Status"}},
		}}, "has no Load"},
		{"tabs need panels", webui.Page[okArgs]{Path: "/a", Body: webui.Tabs{}}, "Tabs has no Panels"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := compileErrors(t, webui.App{Pages: webui.Pages{tt.page}})
			var all string
			for _, e := range errs {
				all += e.Error() + "\n"
			}
			assert.Contains(t, all, tt.want)
		})
	}
}

func TestIDsOnlyNeedToBeUniquePerPage(t *testing.T) {
	t.Parallel()

	table := webui.Table[Device]{Load: okRows, PageSize: 10, Columns: []webui.Accessor[Device]{idCol()}}
	app := webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/a", Body: table},
		webui.Page[webui.NoArgs]{Path: "/b", Body: table}, // the same default ID, on another page
	}}
	_, err := app.Compile("/admin")
	assert.NoError(t, err)
}

func TestPanelsOfOneTabsMayShareAnID(t *testing.T) {
	t.Parallel()

	shared := webui.Table[Device]{Load: okRows, Columns: []webui.Accessor[Device]{idCol()}} // both are "table"
	app := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: webui.Tabs{ID: "view", Panels: []webui.Tab{
		{Label: "Overview", Body: webui.Split{webui.Form[Device]{Load: func(context.Context) (Device, error) { return Device{}, nil }}, shared}},
		{Label: "Raw", Body: shared},
	}}}}}
	_, err := app.Compile("/admin")
	assert.NoError(t, err)
}

func TestArgumentNamesMayNotContainTheViewSeparator(t *testing.T) {
	t.Parallel()

	type dotted struct {
		Odd string `webui:"devices.offset"`
	}
	app := webui.App{Pages: webui.Pages{webui.Page[dotted]{Path: "/a", Body: webui.Stack{}}}}
	errs := compileErrors(t, app)
	assert.Equal(t, len(errs), 1)
	assert.Contains(t, errs[0].Detail, `contains "."`)
}

func TestValidateDuplicatePathsAndShadow(t *testing.T) {
	t.Parallel()

	devices := webui.Nav{Label: "Devices"}
	app := webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/a", Nav: devices, Body: webui.Stack{}},
		webui.Page[webui.NoArgs]{Path: "/a", Body: webui.Stack{}},
		webui.Page[webui.NoArgs]{Path: "/b", Nav: webui.Nav{Shadow: &webui.Nav{Label: "Ghost"}}, Body: webui.Stack{}},
	}}
	var all string
	for _, e := range compileErrors(t, app) {
		all += e.Error() + "\n"
	}
	assert.Contains(t, all, "two pages declare this path")
	assert.Contains(t, all, "Nav.Shadow points at a Nav that no page owns")
}

func TestCompileRejectsBadPrefix(t *testing.T) {
	t.Parallel()

	for _, prefix := range []string{"admin", "/admin/", "/a{b}"} {
		_, err := webui.App{}.Compile(prefix)
		assert.Error(t, err)
	}
	for _, prefix := range []string{"", "/admin"} {
		_, err := webui.App{}.Compile(prefix)
		assert.NoError(t, err)
	}
}

func TestValidateThemeColours(t *testing.T) {
	t.Parallel()

	app := webui.App{Theme: webui.Theme{
		Accent:   "red; } body { display:none",
		OK:       "green", // a name is not a hex colour
		Warning:  "#ff9830",
		Critical: "#12345", // five digits is no colour
	}}
	var all string
	for _, e := range compileErrors(t, app) {
		all += e.Error() + "\n"
	}
	assert.Contains(t, all, `Theme.Accent is "red; } body { display:none", which is not a hex colour`)
	assert.Contains(t, all, `Theme.OK is "green"`)
	assert.Contains(t, all, `Theme.Critical is "#12345"`)
	assert.False(t, strings.Contains(all, "Theme.Warning"))

	for _, ok := range []webui.Color{"#fff", "#ffff", "#3d71d9", "#3D71D9CC"} {
		_, err := webui.App{Theme: webui.Theme{Accent: ok}}.Compile("")
		assert.NoError(t, err)
	}
}
