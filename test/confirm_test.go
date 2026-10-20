package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
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
		Rows: func(context.Context) ([]parcel, error) { return parcelsData, nil }, Key: func(p parcel) string { return p.Name },
		Columns: []webui.Accessor[parcel]{parcelName},
		Actions: []webui.Action[parcel]{{Label: "Drop", Confirm: confirm, Guard: guard, Run: done("row")}},
		BulkActions: []webui.Action[[]parcel]{{
			Label: "Drop all", Confirm: confirm, Role: webui.RoleDestructive,
			Run: func(context.Context, []parcel) (webui.Outcome, error) {
				ran["bulk"]++
				return webui.Success("bulk"), nil
			},
		}},
	}
	form := webui.Form[parcel]{
		Load:   func(context.Context) (parcel, error) { return parcel{Name: "x"}, nil },
		Fields: []webui.Accessor[parcel]{parcelName},
		Submit: webui.Action[parcel]{Label: "Save", Confirm: confirm, Run: done("form")},
	}
	bar := webui.Toolbar[webui.NoArgs]{Left: []webui.ToolbarItem[webui.NoArgs]{webui.Action[webui.NoArgs]{
		Label: "Sync", Confirm: confirm,
		Run: func(context.Context, webui.NoArgs) (webui.Outcome, error) {
			ran["toolbar"]++
			return webui.Success("toolbar"), nil
		},
	}}}
	h := webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/t", Body: webui.Stack{bar, table}},
		webui.Page[webui.NoArgs]{Path: "/f", Body: form},
	}}.MustCompile("/admin")
	return h, ran
}

// leafValues is a post to the leaf at an address, which formValues cannot be: it posts to the first.
func leafValues(leaf string, kv ...string) url.Values {
	v := url.Values{"_leaf": {leaf}}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Add(kv[i], kv[i+1])
	}
	return v
}

func TestAnActionThatConfirmsOpensADialogThatHoldsTheButtonThatSubmits(t *testing.T) {
	t.Parallel()

	h, _ := confirmApp("Are you sure?", nil)
	body := serve(h, http.MethodGet, "/admin/t").Body.String()

	// The row's button, the bulk action's and the toolbar's each open a dialog of their own, by an invoker command.
	for _, id := range []string{"confirm-p.1-r0a0", "confirm-p.1-r1a0", "confirm-p.1-b0", "confirm-p.0-t0"} {
		assert.Contains(t, body, `commandfor="`+id+`" command="show-modal"`)
		assert.Contains(t, body, `<dialog id="`+id+`" class="confirm" closedby="any" aria-labelledby="`+id+`-text">`)
		assert.Contains(t, body, `<p class="confirm-text" id="`+id+`-text">Are you sure?</p>`)
	}
	// What the dialog submits is what the button would have: the same name and value, which the server reads as it did.
	assert.Contains(t, body, `<button class="btn btn-secondary btn-sm" type="submit" name="_act" value="row:0:a">Drop</button>`)
	assert.Contains(t, body, `type="submit" name="_act" value="bulk:0">Drop all</button>`)
	assert.Contains(t, body, `type="submit" name="_act" value="0">Sync</button>`)
	assert.Contains(t, body, `command="close"`)
	// An id is a place and never a key.
	assert.False(t, strings.Contains(body, `id="confirm-p.1-r0a0-a"`))
	// The opener does not submit.
	assert.Contains(t, body, `type="button" commandfor="confirm-p.1-r0a0"`)
}

func TestSubmittingFromTheDialogDoesTheActionAsTheButtonDoes(t *testing.T) {
	t.Parallel()

	h, ran := confirmApp("Sure?", nil)
	assert.Equal(t, post(h, "/admin/t", leafValues("p.1", "_act", "row:0:a")).Code, http.StatusSeeOther)
	assert.Equal(t, post(h, "/admin/t", leafValues("p.1", "_act", "bulk:0", "_sel", "a")).Code, http.StatusSeeOther)
	assert.Equal(t, post(h, "/admin/t", leafValues("p.0", "_act", "0")).Code, http.StatusSeeOther)
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
	body := serve(h, http.MethodGet, "/admin/f").Body.String()
	assert.Contains(t, body, `commandfor="confirm-p-submit0" command="show-modal"`)
	disabled := strings.Index(body, `<button type="submit" disabled hidden tabindex="-1" aria-hidden="true"></button>`)
	dialog := strings.Index(body, `<dialog id="confirm-p-submit0"`)
	assert.True(t, disabled >= 0 && disabled < dialog)

	// One that does not ask is as it was.
	plain, _ := confirmApp("", nil)
	body = serve(plain, http.MethodGet, "/admin/f").Body.String()
	assert.False(t, strings.Contains(body, "<dialog"))
	assert.False(t, strings.Contains(body, "disabled hidden"))
	assert.Contains(t, body, `<button class="btn btn-primary btn-sm" type="submit">Save</button>`)
}

// A button that its Guard refuses is disabled with the reason, and has no dialog to open.
func TestAGatedButtonHasNoDialog(t *testing.T) {
	t.Parallel()

	h, _ := confirmApp("Sure?", func(context.Context, parcel) error { return errors.New("read only") })
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, body, `data-guard="read only"`)
	assert.False(t, strings.Contains(body, `id="confirm-p.1-r0a0"`))
	assert.True(t, strings.Contains(body, `id="confirm-p.1-b0"`)) // the bulk action is not gated
}

func TestAnActionWithNoConfirmHasNoDialog(t *testing.T) {
	t.Parallel()

	h, _ := confirmApp("", nil)
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.False(t, strings.Contains(body, "<dialog"))
	assert.False(t, strings.Contains(body, "command="))
}

func TestAConfirmOnARowClickIsACompileError(t *testing.T) {
	t.Parallel()

	table := webui.Table[parcel]{
		Rows:     func(context.Context) ([]parcel, error) { return parcelsData, nil },
		Key:      func(p parcel) string { return p.Name },
		Columns:  []webui.Accessor[parcel]{parcelName},
		RowClick: webui.Action[parcel]{Confirm: "Sure?", Run: func(context.Context, parcel) (webui.Outcome, error) { return webui.Success("x"), nil }},
	}
	errs := compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", Body: table}}})
	assert.Contains(t, errs[0].Error(), "Table.RowClick has a Confirm")
}
