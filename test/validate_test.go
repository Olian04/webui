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

func okRows(context.Context, webui.Query) (webui.Window[Device], error) {
	return webui.Window[Device]{}, nil
}

func idCol() webui.Accessor[Device] {
	return webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}
}

// compileErrors compiles and returns the structured problems: the CompileError that
// the returned error wraps for each.
func compileErrors(t *testing.T, app webui.App) []webui.CompileError {
	t.Helper()

	_, err := app.Compile("/admin")
	var first webui.CompileError
	if !errors.As(err, &first) {
		t.Fatalf("Compile error = %v, want it to wrap a CompileError", err)
	}
	all, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("Compile error %T does not wrap its problems with Unwrap() []error", err)
	}
	var errs []webui.CompileError
	for _, e := range all.Unwrap() {
		var ce webui.CompileError
		if !errors.As(e, &ce) {
			t.Fatalf("Compile reported %v, which is not a CompileError", e)
		}
		errs = append(errs, ce)
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
			Actions: []webui.Action[Device]{{Label: "x", Run: func(context.Context, Device) (webui.Outcome, error) { return webui.Outcome{}, nil }}},
		}}, "declares actions but no Key"},
		{"two columns with labels that differ only in case", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{
			Load: okRows, Columns: []webui.Accessor[Device]{idCol(), webui.String[Device]{Label: "id", Load: func(Device) string { return "" }}},
		}}, `the labels "ID" and "id" name the same column in the address`},
		{"two columns with one label", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{
			Load: okRows, Columns: []webui.Accessor[Device]{idCol(), idCol()},
		}}, `the labels "ID" and "ID" name the same column in the address`},
		{"a column label with nothing to name it by", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{
			Load: okRows, Columns: []webui.Accessor[Device]{idCol(), webui.String[Device]{Label: "—", Load: func(Device) string { return "" }}},
		}}, `the label "—" has no letters or digits`},
		{"two tabs with labels that name one tab", webui.Page[okArgs]{Path: "/a", Body: webui.Tabs{Panels: []webui.Tab{
			{Label: "Raw events", Body: webui.Stack{}}, {Label: "raw  events", Body: webui.Stack{}},
		}}}, `names the same tab as another in the address`},
		{"negative page size", webui.Page[okArgs]{Path: "/a", Body: webui.Table[Device]{Load: okRows, PageSize: -1}}, "PageSize is negative"},
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

func TestPanelsOfOneTabsMayRepeatATable(t *testing.T) {
	t.Parallel()

	shared := webui.Table[Device]{Load: okRows, Columns: []webui.Accessor[Device]{idCol()}}
	app := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: webui.Tabs{Panels: []webui.Tab{
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

func TestValidateDuplicatePaths(t *testing.T) {
	t.Parallel()

	app := webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "A"}, Body: webui.Stack{}},
		webui.Page[webui.NoArgs]{Path: "/a", Body: webui.Stack{}},
	}}
	var all string
	for _, e := range compileErrors(t, app) {
		all += e.Error() + "\n"
	}
	assert.Contains(t, all, "two pages declare this path")
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
