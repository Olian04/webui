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

func okRows(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
	return webui.PagedWindow[Device]{}, webui.Ready()
}

func idCol() webui.Accessor[Device] {
	return webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}
}

// refuses is whether the problems that an app was refused with (each as Error() says it, one after another) include one that is about everything named: the
// page, the field or argument, the label, as the declaration spells them, and that says what to do about it. What the problem is called in words is the
// library's to change, and is not pinned here; where it is, in the declaration, is.
func refuses(t *testing.T, problems string, names ...string) {
	t.Helper()

	var each []string // a problem is a line, and the line of its fix, which is indented
	for _, line := range strings.Split(problems, "\n") {
		switch {
		case strings.HasPrefix(line, "  "):
			if len(each) > 0 {
				each[len(each)-1] += "\n" + line
			}
		case line != "":
			each = append(each, line)
		}
	}
	for _, one := range each {
		_, fix, _ := strings.Cut(one, "\n")
		ok := true
		for _, name := range names {
			ok = ok && strings.Contains(one, name) // in the problem, or in what to do about it
		}
		if ok {
			if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(fix), "Fix:")) == "" {
				t.Errorf("the problem about %q says nothing to do about it:\n%s", names, one)
			}
			return
		}
	}
	t.Errorf("no problem is about %q, in:\n%s", names, problems)
}

// compileErrors compiles and returns the structured problems: the CompileError that
// the returned error wraps for each.
func compileErrors(t *testing.T, app webui.App) []webui.CompileError {
	t.Helper()

	_, err := app.Compile("/admin")
	if _, ok := errors.AsType[webui.CompileError](err); !ok {
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
		When map[string]int
		A    string `webui:"x"`
		B    string `webui:"x"`
	}
	app := webui.App{Pages: webui.Pages{
		webui.Page[bad]{Path: "/event/{id}", View: webui.Table[Device]{Loader: webui.PagedLoader[Device]{Load: okRows}, Columns: []webui.Accessor[Device]{idCol()}}},
	}}
	errs := compileErrors(t, app)
	assert.Equal(t, len(errs), 3)
	for _, e := range errs {
		assert.Equal(t, e.Page, "/event/{id}") // where
		assert.Equal(t, e.Args, "bad")
	}
	refuses(t, errs[0].Error(), "When", "map[string]int")
	refuses(t, errs[1].Error(), `"x"`)
	refuses(t, errs[2].Error(), "{id}", "bad")
}

func TestValidateLinkToUnmountedPage(t *testing.T) {
	t.Parallel()

	orphan := webui.Page[DetailsArgs]{Path: "/never-mounted/{id}", View: webui.Stack{}}
	app := webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/broken", View: webui.Stack{
			webui.Table[Device]{
				Loader:  webui.PagedLoader[Device]{Load: okRows},
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
		name  string
		page  webui.Page[okArgs]
		names []string
	}{
		{"nil body", webui.Page[okArgs]{Path: "/a"}, []string{"View"}},
		{"relative path", webui.Page[okArgs]{Path: "a", View: webui.Stack{}}, []string{"Path", "\"/\""}},
		{"reserved path", webui.Page[okArgs]{Path: "/_webui/x", View: webui.Stack{}}, []string{"_webui"}},
		{"form without load", webui.Page[okArgs]{Path: "/a", View: webui.Form[Device]{}}, []string{"Form", "Load"}},
		{"RE2", webui.Page[okArgs]{Path: "/a", View: webui.Form[Device]{
			Load: func(context.Context) (Device, webui.Loaded) { return Device{}, webui.Ready() }, Fields: []webui.Accessor[Device]{badPattern},
		}}, []string{"RE"}},
		{"duplicate form labels", webui.Page[okArgs]{Path: "/a", View: webui.Form[Device]{
			Load: func(context.Context) (Device, webui.Loaded) { return Device{}, webui.Ready() }, Fields: dupLabel,
		}}, []string{"\"ID\""}},
		{"group in columns", webui.Page[okArgs]{Path: "/a", View: webui.Table[Device]{
			Loader: webui.PagedLoader[Device]{Load: okRows}, Columns: []webui.Accessor[Device]{group},
		}}, []string{"Group", "Table.Columns"}},
		{"actions need key", webui.Page[okArgs]{Path: "/a", View: webui.Table[Device]{
			Loader: webui.PagedLoader[Device]{Load: okRows}, Columns: []webui.Accessor[Device]{idCol()},
			Actions: []webui.Control[Device]{webui.Action[Device]{Label: "x", Run: func(context.Context, Device) (webui.Outcome, error) { return webui.Outcome{}, nil }}},
		}}, []string{"Key"}},
		{"two columns with labels that differ only in case", webui.Page[okArgs]{Path: "/a", View: webui.Table[Device]{
			Loader: webui.PagedLoader[Device]{Load: okRows}, Columns: []webui.Accessor[Device]{idCol(), webui.String[Device]{Label: "id", Load: func(Device) string { return "" }}},
		}}, []string{"\"ID\"", "\"id\""}},
		{"two columns with one label", webui.Page[okArgs]{Path: "/a", View: webui.Table[Device]{
			Loader: webui.PagedLoader[Device]{Load: okRows}, Columns: []webui.Accessor[Device]{idCol(), idCol()},
		}}, []string{"\"ID\""}},
		{"a column label with nothing to name it by", webui.Page[okArgs]{Path: "/a", View: webui.Table[Device]{
			Loader: webui.PagedLoader[Device]{Load: okRows}, Columns: []webui.Accessor[Device]{idCol(), webui.String[Device]{Label: "—", Load: func(Device) string { return "" }}},
		}}, []string{"\"—\""}},
		{"two tabs with labels that name one tab", webui.Page[okArgs]{Path: "/a", View: webui.Tabs{
			{Label: "Raw events", View: webui.Stack{}}, {Label: "raw  events", View: webui.Stack{}},
		}}, []string{"Tabs[1]", `"raw  events"`}},
		{"negative page size", webui.Page[okArgs]{Path: "/a", View: webui.Table[Device]{Loader: webui.PagedLoader[Device]{Load: okRows}, PageSize: -1}}, []string{"PageSize"}},
		{"slider needs a range", webui.Page[okArgs]{Path: "/a", View: webui.Table[Device]{
			Loader: webui.PagedLoader[Device]{Load: okRows}, Columns: []webui.Accessor[Device]{webui.Slider[Device]{Label: "Rate", Load: func(Device) float64 { return 0 }}},
		}}, []string{"range"}},
		{"slider range must be ascending", webui.Page[okArgs]{Path: "/a", View: webui.Table[Device]{
			Loader: webui.PagedLoader[Device]{Load: okRows}, Columns: []webui.Accessor[Device]{webui.Slider[Device]{Label: "Rate", Min: 5, Max: 5, Load: func(Device) float64 { return 0 }}},
		}}, []string{"range"}},
		{"select needs a load", webui.Page[okArgs]{Path: "/a", View: webui.Table[Device]{
			Loader: webui.PagedLoader[Device]{Load: okRows}, Columns: []webui.Accessor[Device]{webui.Select[Device]{Label: "Status"}},
		}}, []string{"Load"}},
		{"tabs need panels", webui.Page[okArgs]{Path: "/a", View: webui.Tabs{}}, []string{"Tabs", "Tab"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := compileErrors(t, webui.App{Pages: webui.Pages{tt.page}})
			var all string
			for _, e := range errs {
				all += e.Error() + "\n"
			}
			refuses(t, all, tt.names...)
		})
	}
}

func TestIDsOnlyNeedToBeUniquePerPage(t *testing.T) {
	t.Parallel()

	table := webui.Table[Device]{Loader: webui.PagedLoader[Device]{Load: okRows}, PageSize: 10, Columns: []webui.Accessor[Device]{idCol()}}
	app := webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/a", View: table},
		webui.Page[webui.NoArgs]{Path: "/b", View: table}, // the same default ID, on another page
	}}
	_, err := app.Compile("/admin")
	assert.NoError(t, err)
}

func TestPanelsOfOneTabsMayRepeatATable(t *testing.T) {
	t.Parallel()

	shared := webui.Table[Device]{Loader: webui.PagedLoader[Device]{Load: okRows}, Columns: []webui.Accessor[Device]{idCol()}}
	app := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", View: webui.Tabs{
		{Label: "Overview", View: webui.Split{webui.Form[Device]{Load: func(context.Context) (Device, webui.Loaded) { return Device{}, webui.Ready() }}, shared}},
		{Label: "Raw", View: shared},
	}}}}
	_, err := app.Compile("/admin")
	assert.NoError(t, err)
}

func TestArgumentNamesMayNotContainTheViewSeparator(t *testing.T) {
	t.Parallel()

	type dotted struct {
		Odd string `webui:"devices.offset"`
	}
	app := webui.App{Pages: webui.Pages{webui.Page[dotted]{Path: "/a", View: webui.Stack{}}}}
	errs := compileErrors(t, app)
	assert.Equal(t, len(errs), 1)
	refuses(t, errs[0].Error(), `"."`)
}

func TestValidateDuplicatePaths(t *testing.T) {
	t.Parallel()

	app := webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "A"}, View: webui.Stack{}},
		webui.Page[webui.NoArgs]{Path: "/a", View: webui.Stack{}},
	}}
	var all string
	for _, e := range compileErrors(t, app) {
		all += e.Error() + "\n"
	}
	refuses(t, all, "path")
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
	refuses(t, all, "Theme.Accent", `"red; } body { display:none"`)
	refuses(t, all, "Theme.OK", `"green"`)
	refuses(t, all, "Theme.Critical", `"#12345"`)
	assert.False(t, strings.Contains(all, "Theme.Warning"))

	for _, ok := range []webui.Color{"#36c", "#36cf", "#3d71d9", "#3D71D9FF"} {
		_, err := webui.App{Theme: webui.Theme{Accent: ok}}.Compile("")
		assert.NoError(t, err)
	}
}

func TestAnAccentThatCannotCarryAWhiteLabelIsACompileErrorThatSaysWhatToChange(t *testing.T) {
	t.Parallel()

	for _, accent := range []webui.Color{"#2f9e8f", "#ff6600", "#fff"} {
		errs := compileErrors(t, webui.App{Theme: webui.Theme{Accent: accent}})
		assert.True(t, len(errs) > 0)
		first := errs[0].Error()
		assert.Contains(t, first, `Theme.Accent is "`+string(accent)+`": white on it, the label of a primary button is `)
		assert.Contains(t, first, "4.5:1 is needed")
		assert.Contains(t, first, "Choose a darker colour")
	}
}

func TestAStatusColourThatCannotBeReadOnItsBadgeIsACompileError(t *testing.T) {
	t.Parallel()

	var all string
	for _, e := range compileErrors(t, webui.App{Theme: webui.Theme{Warning: "#fff200"}}) {
		all += e.Error() + "\n"
	}
	refuses(t, all, "Theme.Warning", `"#fff200"`, "light") // and the mode it fails in
}

func TestAColourOfAModeThatIsNotAColourIsACompileErrorThatNamesTheField(t *testing.T) {
	t.Parallel()

	var all string
	for _, e := range compileErrors(t, webui.App{Theme: webui.Theme{
		Dark:  webui.ModeColors{Canvas: "navy", Accent: "#12345"},
		Light: webui.ModeColors{Critical: "red; }"},
	}}) {
		all += e.Error() + "\n"
	}
	refuses(t, all, "Theme.Dark.Canvas", `"navy"`)
	refuses(t, all, "Theme.Dark.Accent", `"#12345"`)
	refuses(t, all, "Theme.Light.Critical", `"red; }"`)
	assert.False(t, strings.Contains(all, "white on it")) // what is not a colour has no contrast to check
}

func TestACanvasOfTheWrongLightnessForItsModeNamesTheCanvasOfThatMode(t *testing.T) {
	t.Parallel()

	errs := compileErrors(t, webui.App{Theme: webui.Theme{Dark: webui.ModeColors{Canvas: "#ffffff"}}})
	assert.True(t, len(errs) > 0)
	first := errs[0].Error()
	assert.Contains(t, first, `Theme.Dark.Canvas is "#ffffff": its text on the panel is `)
	assert.Contains(t, first, "in the dark theme")
	assert.Contains(t, first, "Choose a darker colour for Theme.Dark.Canvas")

	errs = compileErrors(t, webui.App{Theme: webui.Theme{Light: webui.ModeColors{Canvas: "#000000"}}})
	refuses(t, errs[0].Error(), "Theme.Light.Canvas")

	// The same canvas is fine in the mode it suits, and only the mode that is given one is checked against it.
	_, err := webui.App{Theme: webui.Theme{Dark: webui.ModeColors{Canvas: "#0e1224"}, Light: webui.ModeColors{Canvas: "#eef0f9"}}}.Compile("")
	assert.NoError(t, err)
}

// An accent that is shared is checked in both modes, and one that a mode gives is blamed on that mode's own field.
func TestAModesOwnAccentThatCannotCarryAWhiteLabelNamesThatField(t *testing.T) {
	t.Parallel()

	errs := compileErrors(t, webui.App{Theme: webui.Theme{Accent: "#1f7a6d", Light: webui.ModeColors{Accent: "#2f9e8f"}}})
	assert.True(t, len(errs) > 0)
	for _, e := range errs {
		refuses(t, e.Error(), "Theme.Light.Accent", `"#2f9e8f"`, "light")
	}
}

// What is not a colour has no contrast to check, so the one error is the one about the colour.
func TestAThemeThatHasABadColourIsNotAlsoCheckedForContrast(t *testing.T) {
	t.Parallel()

	errs := compileErrors(t, webui.App{Theme: webui.Theme{Accent: "red", Dark: webui.ModeColors{Canvas: "#ffffff"}}})
	assert.Equal(t, len(errs), 1)
	refuses(t, errs[0].Error(), "Theme.Accent", `"red"`)
}
