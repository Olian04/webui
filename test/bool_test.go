package webui_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// flags holds a yes or no that is always one of them: a bool, where an OptionalBool's is a *bool.
type flags struct {
	Name     string
	Compress bool
}

var (
	flagName     = webui.String[flags]{Label: "Name", Load: func(f flags) string { return f.Name }, Store: func(f *flags, v string) { f.Name = v }}
	flagCompress = webui.Bool[flags]{Label: "Compress", Help: "Saves bandwidth.", Load: func(f flags) bool { return f.Compress }, Store: func(f *flags, v bool) { f.Compress = v }}
)

func flagsApp(start flags, fields ...webui.Accessor[flags]) (http.Handler, func() flags) {
	var saved = start
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/f", View: webui.Form[flags]{
		Load:   func(context.Context) (flags, webui.Loaded) { return saved, webui.Ready() },
		Fields: fields,
		Submit: webui.Action[flags]{Run: func(_ context.Context, f flags) (webui.Outcome, error) {
			saved = f
			return webui.Success("Saved"), nil
		}},
	}}}}.MustCompile("/admin")
	return h, func() flags { return saved }
}

// A Bool is a toggle switch where it is edited: on for yes, and the field a browser sends for it is "true".
func TestABoolIsASwitchThatIsOnForYes(t *testing.T) {
	t.Parallel()

	h, _ := flagsApp(flags{Compress: true}, flagCompress)
	p := pageOf(t, h, "/admin/f")
	sw := page.One(t, "the Compress switch", p.ByRole("switch", "Compress"))
	assert.Equal(t, sw.Attr("type"), "checkbox") // a checkbox to the browser, so it needs no script
	f, _ := p.Field("f0")
	assert.Equal(t, f.Value, "true")
	assert.True(t, f.Checked)
	assert.Equal(t, len(p.ByRole("combobox", "")), 0) // not the Yes/No drop-down that an OptionalBool is

	h, _ = flagsApp(flags{Compress: false}, flagCompress)
	f, _ = pageOf(t, h, "/admin/f").Field("f0")
	assert.Equal(t, f.Type, "checkbox")
	assert.False(t, f.Checked)
}

// An off switch is not sent at all, so a field that is absent is no, and one that says "true" is yes.
func TestAnAbsentBoolIsNoAndTrueIsYes(t *testing.T) {
	t.Parallel()

	h, saved := flagsApp(flags{Compress: true}, flagName, flagCompress)
	rec := post(h, "/admin/f", formValues("f0", "x"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), flags{Name: "x", Compress: false}) // not sent: no, and not "left as it was"

	rec = post(h, "/admin/f", formValues("f0", "y", "f1", "true"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), flags{Name: "y", Compress: true})

	rec = post(h, "/admin/f", formValues("f0", "z", "f1", "false"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), flags{Name: "z", Compress: false})
}

func TestABoolRefusesWhatIsNotYesOrNo(t *testing.T) {
	t.Parallel()

	h, saved := flagsApp(flags{Compress: true}, flagCompress)
	rec := post(h, "/admin/f", formValues("f0", "maybe"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.DeepEqual(t, saved(), flags{Compress: true})
}

// A Bool with no Store is only read: a Yes or a No, as an OptionalBool is, and never an input.
func TestAReadOnlyBoolIsAYesOrANo(t *testing.T) {
	t.Parallel()

	readOnly := webui.Bool[flags]{Label: "Compress", Load: func(f flags) bool { return f.Compress }}
	h, _ := flagsApp(flags{Compress: true}, readOnly)
	p := pageOf(t, h, "/admin/f")
	assert.Equal(t, len(p.ByText("Yes")), 1)
	// Not a field's input: the page posts nothing but the view it is.
	for _, f := range p.Fields() {
		assert.True(t, f.Type == "hidden" || f.Name == "")
	}
	assert.Equal(t, len(p.ByRole("switch", "Compress")), 0)

	h, _ = flagsApp(flags{Compress: false}, readOnly)
	assert.Equal(t, len(pageOf(t, h, "/admin/f").ByText("No")), 1)
}

// The one accessor is written once and used where a table shows it too: Yes and No in the cells, a filter that is a choice of the two, and a file
// that holds the words in a CSV and true and false in JSON.
func TestABoolIsAColumnWithAFilterAndExportsAsTrueAndFalse(t *testing.T) {
	t.Parallel()

	rows := []flags{{Name: "a", Compress: true}, {Name: "b", Compress: false}, {Name: "c", Compress: true}}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: webui.Table[flags]{
		Title:   "Flags",
		Loader:  webui.RowsLoader[flags]{Load: func(context.Context) ([]flags, webui.Loaded) { return rows, webui.Ready() }},
		Columns: []webui.Accessor[flags]{flagName, flagCompress},
		Export:  true,
	}}}}.MustCompile("/admin")

	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, body, ">Yes<")
	assert.Contains(t, body, ">No<")

	// Filtered by the choice it is: only the rows that are yes.
	filtered := serve(h, http.MethodGet, "/admin/t?flags.filter.compress=true").Body.String()
	assert.Contains(t, filtered, ">a<")
	assert.Contains(t, filtered, ">c<")
	assert.False(t, strings.Contains(filtered, ">b<"))

	csv := serve(h, http.MethodGet, "/admin/t?flags.export=csv").Body.String()
	assert.Contains(t, csv, "a,Yes") // a CSV holds what the table shows
	assert.Contains(t, csv, "b,No")
	js := serve(h, http.MethodGet, "/admin/t?flags.export=json").Body.String()
	assert.Contains(t, js, `"Compress": true`)
	assert.Contains(t, js, `"Compress": false`)
	assert.False(t, strings.Contains(js, "null")) // a Bool is never not set
}

func TestABoolThatCannotBeDrawnIsACompileError(t *testing.T) {
	t.Parallel()

	var all string
	for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/f", View: webui.Form[flags]{
		Load:   func(context.Context) (flags, webui.Loaded) { return flags{}, webui.Ready() },
		Fields: []webui.Accessor[flags]{webui.Bool[flags]{Label: "Compress"}, webui.Bool[flags]{Load: func(flags) bool { return false }}},
		Submit: webui.Action[flags]{Run: func(context.Context, flags) (webui.Outcome, error) { return webui.Success("ok"), nil }},
	}}}}) {
		all += e.Error() + "\n"
	}
	assert.Contains(t, all, "Load")
	assert.Contains(t, all, "Label")
}
