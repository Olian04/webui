package webui_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
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
		View: webui.Table[Device]{
			Title:  "Entries",
			Loader: webui.RowsLoader[Device]{Load: func(context.Context) ([]Device, webui.Loaded) { return entries, webui.Ready() }},
			Key:    func(d Device) string { return d.Id },
			RowClick: webui.Action[Device]{
				Guard: guard,
				Run: func(ctx context.Context, d Device) (webui.Outcome, error) {
					if before, ok := strings.CutSuffix(d.Id, "/"); ok {
						return webui.Success("").Then(webui.Open(ctx, folderPath, detailsArgs{Id: before})), nil
					}
					return webui.Success("").Then(webui.Open(ctx, objectPath, detailsArgs{Id: d.Id})), nil
				},
			},
			Columns: []webui.Accessor[Device]{formID},
		},
	}
	folder := webui.Page[detailsArgs]{Path: folderPath, View: webui.Stack{}}
	object := webui.Page[detailsArgs]{Path: objectPath, View: webui.Stack{}}
	return webui.App{Pages: webui.Pages{list, folder, object}}.MustCompile("/admin")
}

func TestARowClickCanBeAnActionThatChoosesTheDestinationFromTheRow(t *testing.T) {
	t.Parallel()

	h := browserApp(nil)
	p := pageOf(t, h, "/admin/entry")

	// The first cell holds one button that covers the row, and no anchor.
	assert.DeepEqual(t, rowClicks(p), []string{"click:0:photos/", "click:0:cat.png", "click:0:locked.txt"})
	assert.Equal(t, len(p.ByRole("table", "")[0].ByRole("button", "photos/")), 1) // it is called by the row's own text

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
	p := pageOf(t, h, "/admin/entry")

	// The refused row is shown and does nothing; the same Guard refuses a POST for it.
	assert.Equal(t, len(rowClicks(p)), 2)
	for _, v := range rowClicks(p) {
		assert.False(t, v == "click:0:locked.txt")
	}
	rec := post(h, "/admin/entry", formValues("_act", "click:0:locked.txt"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Contains(t, follow(h, rec).Body.String(), "locked")
}

func TestARowClickActionNamesTheRowByKeyAndSaysSoWhenItIsGone(t *testing.T) {
	t.Parallel()

	h := browserApp(nil)
	rec := post(h, "/admin/entry", formValues("_act", "click:0:ghost"))
	assert.Equal(t, rec.Code, http.StatusSeeOther) // back to the table, with a message
	assert.Equal(t, post(h, "/admin/entry", formValues("_act", "click:1:nope")).Code, http.StatusSeeOther)
}

func TestARowClickActionNeedsARunAndAKey(t *testing.T) {
	t.Parallel()

	run := func(context.Context, Device) (webui.Outcome, error) { return webui.Outcome{}, nil }
	problems := func(tbl webui.Table[Device]) string {
		tbl.Loader = webui.PagedLoader[Device]{Load: okRows}
		tbl.Columns = []webui.Accessor[Device]{formID}
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", View: tbl}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	key := func(d Device) string { return d.Id }

	refuses(t, problems(webui.Table[Device]{RowClick: webui.Action[Device]{Run: run}}), "Key")
	refuses(t, problems(webui.Table[Device]{Key: key, RowClick: webui.Action[Device]{}}), "Table.RowClick", "Run")
}

// rowClicks are the values of the buttons that a row of a table is clicked with, in order.
func rowClicks(p *page.Page) []string {
	var out []string
	for _, f := range fieldsNamed(p, "_act") {
		if strings.HasPrefix(f.Value, "click:") {
			out = append(out, f.Value)
		}
	}
	return out
}
