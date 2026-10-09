package webui_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

const (
	folderPath webui.PageID[detailsArgs] = "/folder/{id}"
	objectPath webui.PageID[detailsArgs] = "/object/{id}"
)

// browserApp lists entries that are folders or objects, and a click on a row goes to
// the kind of page that entry is: something a Link, which has one destination, cannot say.
func browserApp(guard func(context.Context, Device) error) http.Handler {
	entries := []Device{{Id: "photos/"}, {Id: "cat.png"}, {Id: "locked.txt"}}
	list := webui.Page[webui.NoArgs]{
		Path: "/entry",
		Nav:  webui.Nav{Label: "Entries"},
		Body: webui.Table[Device]{
			Title: "Entries",
			Rows:  func(context.Context) ([]Device, error) { return entries, nil },
			Key:   func(d Device) string { return d.Id },
			RowClick: webui.Action[Device]{
				Guard: guard,
				Run: func(ctx context.Context, d Device) (webui.Outcome, error) {
					if strings.HasSuffix(d.Id, "/") {
						return webui.Success("").Then(webui.Open(ctx, folderPath, detailsArgs{Id: strings.TrimSuffix(d.Id, "/")})), nil
					}
					return webui.Success("").Then(webui.Open(ctx, objectPath, detailsArgs{Id: d.Id})), nil
				},
			},
			Columns: []webui.Accessor[Device]{formID},
		},
	}
	folder := webui.Page[detailsArgs]{Path: folderPath, Body: webui.Stack{}}
	object := webui.Page[detailsArgs]{Path: objectPath, Body: webui.Stack{}}
	return webui.App{Pages: webui.Pages{list, folder, object}}.MustCompile("/admin")
}

func TestARowClickCanBeAnActionThatChoosesTheDestinationFromTheRow(t *testing.T) {
	t.Parallel()

	h := browserApp(nil)
	body := serve(h, http.MethodGet, "/admin/entry").Body.String()

	// The first cell holds one button that covers the row, and no anchor.
	assert.Contains(t, body, `<button class="rowlink rowbutton" type="submit" name="_act" value="click:0:photos/">`)
	assert.Equal(t, strings.Count(body, `class="rowlink rowbutton"`), 3)
	assert.False(t, strings.Contains(body, `<a class="rowlink"`))
	assert.Contains(t, body, `<tr class="clickable">`)

	// A click is a POST, which goes where the row says.
	folder := post(h, "/admin/entry", formValues("_act", "click:0:photos/"))
	assert.Equal(t, folder.Code, http.StatusSeeOther)
	assert.Equal(t, folder.Header().Get("Location"), "/admin/folder/photos")
	object := post(h, "/admin/entry", formValues("_act", "click:0:cat.png"))
	assert.Equal(t, object.Header().Get("Location"), "/admin/object/cat.png")
}

func TestARowClickActionIsGatedPerRowByItsGuard(t *testing.T) {
	t.Parallel()

	h := browserApp(func(_ context.Context, d Device) error {
		if d.Id == "locked.txt" {
			return errors.New("locked")
		}
		return nil
	})
	body := serve(h, http.MethodGet, "/admin/entry").Body.String()

	// The refused row is shown and does nothing; the same Guard refuses a POST for it.
	assert.Equal(t, strings.Count(body, `class="rowlink rowbutton"`), 2)
	assert.False(t, strings.Contains(body, `value="click:0:locked.txt"`))
	assert.Equal(t, post(h, "/admin/entry", formValues("_act", "click:0:locked.txt")).Code, http.StatusForbidden)
}

func TestARowClickActionNamesTheRowByKeyAndSaysSoWhenItIsGone(t *testing.T) {
	t.Parallel()

	h := browserApp(nil)
	rec := post(h, "/admin/entry", formValues("_act", "click:0:ghost"))
	assert.Equal(t, rec.Code, http.StatusSeeOther) // back to the table, with a message
	assert.Equal(t, post(h, "/admin/entry", formValues("_act", "click:1:nope")).Code, http.StatusSeeOther)
}

func TestARowClickActionNeedsARunAndAKeyAndCannotBeSearched(t *testing.T) {
	t.Parallel()

	run := func(context.Context, Device) (webui.Outcome, error) { return webui.Outcome{}, nil }
	problems := func(tbl webui.Table[Device]) string {
		tbl.Load = okRows
		tbl.Columns = []webui.Accessor[Device]{formID}
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: tbl}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	key := func(d Device) string { return d.Id }

	assert.Contains(t, problems(webui.Table[Device]{RowClick: webui.Action[Device]{Run: run}}), "declares actions but no Key")
	assert.Contains(t, problems(webui.Table[Device]{Key: key, RowClick: webui.Action[Device]{}}), "Table.RowClick has no Run")
	assert.Contains(t, problems(webui.Table[Device]{Key: key, Search: true, RowClick: webui.Action[Device]{Run: run}}), "RowClick is an Action")
}
