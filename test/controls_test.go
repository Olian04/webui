package webui_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type parcelArgs struct {
	Name string `webui:"name"`
}

// controlPages is a table of the two parcels with the controls given, and the page that a Link leads to, whose Guard refuses the parcel that it is told to.
func controlPages(refuse string, table webui.Table[parcel]) http.Handler {
	table.Loader = webui.RowsLoader[parcel]{Load: func(context.Context) ([]parcel, webui.Loaded) { return parcelsData, webui.Ready() }}
	table.Columns = []webui.Accessor[parcel]{parcelName}
	detail := webui.Page[parcelArgs]{
		Path: "/parcel/{name}",
		Guard: func(_ context.Context, a parcelArgs) error {
			if a.Name == refuse {
				return webui.Forbidden("no")
			}
			return nil
		},
		Body: webui.Stack{},
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", Body: table}, detail}}.MustCompile("/admin")
}

func openParcel(label string, detail webui.Page[parcelArgs]) webui.Link[parcel, parcelArgs] {
	return webui.Link[parcel, parcelArgs]{Label: label, Page: detail, Args: func(_ context.Context, p parcel) parcelArgs { return parcelArgs(p) }}
}

// A link is a control of a row as an action is: a button in the row's own column that leads to the page the row names, for the rows whose page the visitor may open,
// and with no Key, since it posts nothing.
func TestALinkInARowsActionsIsAButtonThatLeadsToTheRowsPage(t *testing.T) {
	t.Parallel()

	detail := webui.Page[parcelArgs]{Path: "/parcel/{name}", Body: webui.Stack{}}
	h := controlPages("b", webui.Table[parcel]{Actions: []webui.Control[parcel]{openParcel("Open", detail)}})
	body := serve(h, http.MethodGet, "/admin/t").Body.String()

	assert.Contains(t, body, `href="/admin/parcel/a"`)
	assert.Contains(t, body, `Open <span class="sr-only">a</span>`)   // named by the row it is on, as a button is
	assert.False(t, strings.Contains(body, `href="/admin/parcel/b"`)) // that page refuses the visitor: no button for it
	assert.False(t, strings.Contains(body, `name="_act"`))            // nothing posts
}

// An address is a control of a row too: a button that opens another site in a tab of its own.
func TestAnHrefInARowsActionsIsAButtonThatOpensAnotherSite(t *testing.T) {
	t.Parallel()

	h := controlPages("", webui.Table[parcel]{Actions: []webui.Control[parcel]{
		webui.Href[parcel]{Label: "Track", URL: func(p parcel) string { return "https://example.com/track/" + p.Name }},
		webui.Href[parcel]{Label: "Never", URL: func(parcel) string { return "javascript:alert(1)" }}, // not a link: not drawn
	}})
	body := serve(h, http.MethodGet, "/admin/t").Body.String()

	assert.Contains(t, body, `href="https://example.com/track/a"`)
	assert.Contains(t, body, `rel="noopener noreferrer" target="_blank"`)
	assert.False(t, strings.Contains(body, "Never"))
	assert.False(t, strings.Contains(body, "javascript:"))
}

// The three are interchangeable in one list of controls: the buttons are in the order declared, and only the action posts.
func TestAnActionALinkAndAnHrefShareARowsActionsInTheOrderDeclared(t *testing.T) {
	t.Parallel()

	detail := webui.Page[parcelArgs]{Path: "/parcel/{name}", Body: webui.Stack{}}
	var ran []string
	table := webui.Table[parcel]{
		Key: func(p parcel) string { return p.Name },
		Actions: []webui.Control[parcel]{
			webui.Action[parcel]{Label: "Drop", Run: func(_ context.Context, p parcel) (webui.Outcome, error) {
				ran = append(ran, p.Name)
				return webui.Success("done"), nil
			}},
			openParcel("Open", detail),
			webui.Href[parcel]{Label: "Track", URL: func(p parcel) string { return "https://example.com/" + p.Name }},
		},
	}
	h := controlPages("", table)
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	drop, open, track := strings.Index(body, "Drop <span"), strings.Index(body, "Open <span"), strings.Index(body, "Track <span")
	assert.True(t, drop >= 0 && drop < open && open < track)

	// The action posts, naming the row by its key.
	assert.Equal(t, post(h, "/admin/t", formValues("_act", "row:0:b")).Code, http.StatusSeeOther)
	assert.DeepEqual(t, ran, []string{"b"})

	// A post that names a control that leads somewhere is not understood, whatever the key: only an action can be posted. It is answered as any post
	// that names nothing the page has is, and nothing ran.
	for _, act := range []string{"row:1:b", "row:2:b"} {
		rec := post(h, "/admin/t", formValues("_act", act))
		assert.Equal(t, rec.Code, http.StatusSeeOther)
		assert.False(t, strings.Contains(rec.Header().Get("Set-Cookie"), "done"))
	}
	assert.DeepEqual(t, ran, []string{"b"})
}

// A click on a row can be an address too, which opens in a tab of its own.
func TestARowThatLeadsToAnHrefOpensAnotherSiteInATabOfItsOwn(t *testing.T) {
	t.Parallel()

	h := controlPages("", webui.Table[parcel]{RowClick: webui.Href[parcel]{URL: func(p parcel) string { return "https://example.com/" + p.Name }}})
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, body, `<a class="rowlink" href="https://example.com/a" target="_blank" rel="noopener noreferrer">`)
}

// What a control needs depends on where it is, and a table needs a Key only for what posts.
func TestWhatAControlNeedsDependsOnWhereItIsAndATableNeedsAKeyOnlyForWhatPosts(t *testing.T) {
	t.Parallel()

	detail := webui.Page[parcelArgs]{Path: "/parcel/{name}", Body: webui.Stack{}}
	load := func(context.Context) ([]parcel, webui.Loaded) { return parcelsData, webui.Ready() }
	cols := []webui.Accessor[parcel]{parcelName}
	run := func(context.Context, parcel) (webui.Outcome, error) { return webui.Success("done"), nil }
	for name, tc := range map[string]struct {
		table webui.Table[parcel]
		want  string // "" for a table that compiles
	}{
		"a link in the actions has no label":  {webui.Table[parcel]{Actions: []webui.Control[parcel]{webui.Link[parcel, parcelArgs]{Page: detail, Args: func(context.Context, parcel) parcelArgs { return parcelArgs{} }}}}, "Table.Actions[0] has no Label"},
		"an href in the actions has no label": {webui.Table[parcel]{Actions: []webui.Control[parcel]{webui.Href[parcel]{URL: func(parcel) string { return "/x" }}}}, "Table.Actions[0] has no Label"},
		"an href with no url":                 {webui.Table[parcel]{Actions: []webui.Control[parcel]{webui.Href[parcel]{Label: "Go"}}}, "Table.Actions[0] is an Href with no URL"},
		"an action needs a key":               {webui.Table[parcel]{Actions: []webui.Control[parcel]{webui.Action[parcel]{Label: "Drop", Run: run}}}, "a Table declares actions but no Key"},
		"a click that is an action needs one": {webui.Table[parcel]{RowClick: webui.Action[parcel]{Run: run}}, "a Table declares actions but no Key"},
		"a nil control":                       {webui.Table[parcel]{Actions: []webui.Control[parcel]{nil}}, "Table.Actions[0] is nil"},
		"a click that asks first":             {webui.Table[parcel]{Key: func(p parcel) string { return p.Name }, RowClick: webui.Action[parcel]{Run: run, Confirm: "Sure?"}}, "Table.RowClick has a Confirm"},
		"links and addresses need no key":     {webui.Table[parcel]{Actions: []webui.Control[parcel]{webui.Href[parcel]{Label: "Go", URL: func(parcel) string { return "/x" }}}, RowClick: webui.Href[parcel]{URL: func(parcel) string { return "/x" }}}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.table.Loader, tc.table.Columns = webui.RowsLoader[parcel]{Load: load}, cols
			app := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", Body: tc.table}, detail}}
			_, err := app.Compile("/admin")
			if tc.want == "" {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
