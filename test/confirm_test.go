package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

type parcel struct{ Name string }

var (
	parcelName  = webui.String[parcel]{Label: "Name", Load: func(p parcel) string { return p.Name }, Store: func(p *parcel, v string) { p.Name = v }}
	parcelsData = []parcel{{Name: "a"}, {Name: "b"}}
)

// confirmApp is a table with a row action, a bulk action and a toolbar action that ask, and a form whose Save asks; ran counts
// what was done, by the action's name.
func confirmApp(confirm string, guard func(context.Context, parcel) error) (http.Handler, map[string]int) {
	ran := map[string]int{}
	done := func(name string) func(context.Context, parcel) (webui.Outcome, error) {
		return func(context.Context, parcel) (webui.Outcome, error) { ran[name]++; return webui.Success(name), nil }
	}
	table := webui.Table[parcel]{
		Loader: webui.RowsLoader[parcel]{Load: func(context.Context) ([]parcel, webui.Loaded) { return parcelsData, webui.Ready() }}, Key: func(p parcel) string { return p.Name },
		Columns: []webui.Accessor[parcel]{parcelName},
		Actions: []webui.Control[parcel]{webui.Action[parcel]{Label: "Drop", Confirm: confirm, Guard: guard, Run: done("row")}},
		BulkActions: []webui.Action[[]parcel]{{
			Label: "Drop all", Confirm: confirm, Role: webui.RoleDestructive,
			Run: func(context.Context, []parcel) (webui.Outcome, error) {
				ran["bulk"]++
				return webui.Success("bulk"), nil
			},
		}},
	}
	form := webui.Form[parcel]{
		Load:   func(context.Context) (parcel, webui.Loaded) { return parcel{Name: "x"}, webui.Ready() },
		Fields: []webui.Accessor[parcel]{parcelName},
		Submit: webui.Action[parcel]{Label: "Save", Confirm: confirm, Run: done("form")},
	}
	bar := webui.Toolbar{Left: []webui.ToolbarItem{webui.Action[webui.NoArgs]{
		Label: "Sync", Confirm: confirm,
		Run: func(context.Context, webui.NoArgs) (webui.Outcome, error) {
			ran["toolbar"]++
			return webui.Success("toolbar"), nil
		},
	}}}
	h := webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/t", View: webui.Stack{bar, table}},
		webui.Page[webui.NoArgs]{Path: "/f", View: form},
	}}.MustCompile("/admin")
	return h, ran
}

// viewValues is a post to the panel at an address, which formValues cannot be: it posts to the first.
func viewValues(panel string, kv ...string) url.Values {
	v := url.Values{"_view": {panel}}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Add(kv[i], kv[i+1])
	}
	return v
}

// dialogs are the confirmations of a page, by the button in each that does the action: the name of that button, and what it posts as _act.
func dialogs(t *testing.T, p *page.Page) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, d := range p.ByRole("alertdialog", "") {
		var submits []page.El
		for _, b := range d.ByRole("button", "") {
			if b.Attr("type") == "submit" {
				submits = append(submits, b)
			}
		}
		submit := page.One(t, "the button that does it", submits)
		got[submit.Name()] = submit.Attr("value")
	}
	return got
}

// opener is the button of a page that opens a dialog, and does not submit: the one that the dialog is the invoker target of.
func opener(t *testing.T, p *page.Page, d page.El) page.El {
	t.Helper()
	var found []page.El
	for _, b := range p.ByAttr("commandfor", d.Attr("id")) {
		if b.Attr("command") == "show-modal" {
			found = append(found, b)
		}
	}
	return page.One(t, "the opener of "+d.Name(), found)
}

func TestAnActionThatConfirmsOpensADialogThatHoldsTheButtonThatSubmits(t *testing.T) {
	t.Parallel()

	h, _ := confirmApp("Are you sure?", nil)
	p := pageOf(t, h, "/admin/t")

	// The row's button, the bulk action's and the toolbar's each open a dialog of their own, by an invoker command, which asks what was set.
	// What the dialog submits is what the button would have: the same name and value, which the server reads as it did.
	assert.DeepEqual(t, dialogs(t, p), map[string]string{"Drop a": "row:0:a", "Drop b": "row:0:b", "Drop all": "bulk:0", "Sync": "0"})
	for _, d := range p.ByRole("alertdialog", "Are you sure?") {
		var names []string
		var closes bool
		for _, b := range d.ByRole("button", "") {
			names = append(names, b.Name())
			closes = closes || b.Attr("command") == "close"
		}
		assert.True(t, closes) // and says no
		open := opener(t, p, d)
		assert.Equal(t, open.Attr("type"), "button") // the opener does not submit
		assert.True(t, slices.Contains(names, open.Name()))
	}
}

func TestSubmittingFromTheDialogDoesTheActionAsTheButtonDoes(t *testing.T) {
	t.Parallel()

	h, ran := confirmApp("Sure?", nil)
	assert.Equal(t, post(h, "/admin/t", viewValues("p.1", "_act", "row:0:a")).Code, http.StatusSeeOther)
	assert.Equal(t, post(h, "/admin/t", viewValues("p.1", "_act", "bulk:0", "_sel", "a")).Code, http.StatusSeeOther)
	assert.Equal(t, post(h, "/admin/t", viewValues("p.0", "_act", "0")).Code, http.StatusSeeOther)
	assert.Equal(t, post(h, "/admin/f", formValues("f0", "y")).Code, http.StatusSeeOther)
	assert.Equal(t, ran["row"], 1)
	assert.Equal(t, ran["bulk"], 1)
	assert.Equal(t, ran["toolbar"], 1)
	assert.Equal(t, ran["form"], 1)
}

// A form whose Save asks is not saved by Enter in a field, which clicks its first submit button: that is a disabled one, ahead of the dialog's.
func TestAFormWhoseSaveAsksIsNotSubmittedByEnterInAField(t *testing.T) {
	t.Parallel()

	h, _ := confirmApp("Save it?", nil)
	p := pageOf(t, h, "/admin/f")
	dialog := page.One(t, "the dialog", p.ByRole("alertdialog", "Save it?"))
	assert.Equal(t, opener(t, p, dialog).Name(), "Save")
	first := p.ByAttr("type", "submit")[0]
	assert.True(t, first.Has("disabled") && first.Hidden() && first.Before(dialog))

	// One that does not ask is as it was.
	plain, _ := confirmApp("", nil)
	p = pageOf(t, plain, "/admin/f")
	assert.Equal(t, len(p.ByRole("alertdialog", "")), 0)
	for _, b := range p.ByAttr("type", "submit") {
		assert.False(t, b.Has("disabled"))
	}
	assert.Equal(t, len(p.ByRole("button", "Save")), 1)
}

// A button that its Guard refuses has no dialog to open, whether it is left out or drawn disabled, and the dialogs of the others
// keep the ids that they have by the position their actions were declared at.
func TestARefusedButtonHasNoDialog(t *testing.T) {
	t.Parallel()

	h, _ := confirmApp("Sure?", func(context.Context, parcel) error { return errors.New("read only") })
	p := pageOf(t, h, "/admin/t")
	assert.DeepEqual(t, dialogs(t, p), map[string]string{"Drop all": "bulk:0", "Sync": "0"}) // the bulk action has no Guard
	assert.Equal(t, len(p.ByAttr("data-guard", "")), 0)

	h, _ = confirmApp("Sure?", func(context.Context, parcel) error { return webui.Disabled("read only") })
	p = pageOf(t, h, "/admin/t")
	assert.Equal(t, len(p.ByAttr("data-guard", "read only")), 2) // a button for each row, drawn disabled
	assert.DeepEqual(t, dialogs(t, p), map[string]string{"Drop all": "bulk:0", "Sync": "0"})
}

func TestAnActionWithNoConfirmHasNoDialog(t *testing.T) {
	t.Parallel()

	h, _ := confirmApp("", nil)
	p := pageOf(t, h, "/admin/t")
	assert.Equal(t, len(p.ByRole("alertdialog", "")), 0)
	assert.Equal(t, len(p.ByAttr("command", "")), 0)
}

func TestAConfirmOnARowClickIsACompileError(t *testing.T) {
	t.Parallel()

	table := webui.Table[parcel]{
		Loader:   webui.RowsLoader[parcel]{Load: func(context.Context) ([]parcel, webui.Loaded) { return parcelsData, webui.Ready() }},
		Key:      func(p parcel) string { return p.Name },
		Columns:  []webui.Accessor[parcel]{parcelName},
		RowClick: webui.Action[parcel]{Confirm: "Sure?", Run: func(context.Context, parcel) (webui.Outcome, error) { return webui.Success("x"), nil }},
	}
	errs := compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: table}}})
	assert.Contains(t, errs[0].Error(), "Table.RowClick has a Confirm")
}
