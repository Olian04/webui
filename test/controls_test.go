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
		View: webui.Stack{},
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: table}, detail}}.MustCompile("/admin")
}

func openParcel(label string, detail webui.Page[parcelArgs]) webui.Link[parcel, parcelArgs] {
	return webui.Link[parcel, parcelArgs]{Label: label, Page: detail, Args: func(_ context.Context, p parcel) parcelArgs { return parcelArgs(p) }}
}

// A link is a control of a row as an action is: a button in the row's own column that leads to the page the row names, for the rows whose page the visitor may open,
// and with no Key, since it posts nothing.
func TestALinkInARowsActionsIsAButtonThatLeadsToTheRowsPage(t *testing.T) {
	t.Parallel()

	detail := webui.Page[parcelArgs]{Path: "/parcel/{name}", View: webui.Stack{}}
	h := controlPages("b", webui.Table[parcel]{Actions: []webui.Control[parcel]{openParcel("Open", detail)}})
	p := pageOf(t, h, "/admin/t")

	assert.Equal(t, len(p.ByRole("link", "Open a")), 1) // named by the row it is on, as a button is
	assert.Equal(t, len(linksTo(p, "/admin/parcel/a")), 1)
	assert.Equal(t, len(linksTo(p, "/admin/parcel/b")), 0) // that page refuses the visitor: no button for it
	assert.Equal(t, len(p.ByAttr("name", "_act")), 0)      // nothing posts
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

	detail := webui.Page[parcelArgs]{Path: "/parcel/{name}", View: webui.Stack{}}
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
	p := pageOf(t, h, "/admin/t")
	drop := page.One(t, "Drop", p.ByAttr("name", "_act")[:1])
	open := page.One(t, "Open", p.ByRole("link", "Open a"))
	track := page.One(t, "Track", p.ByRoleContaining("link", "Track a"))
	assert.True(t, drop.Before(open) && open.Before(track)) // in the order they were declared

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
	p := pageOf(t, h, "/admin/t")
	first := page.One(t, "the first row's link", p.ByAttr("href", "https://example.com/a"))
	assert.Equal(t, first.Attr("target"), "_blank")
	assert.Equal(t, first.Attr("rel"), "noopener noreferrer")
}

// What a control needs depends on where it is, and a table needs a Key only for what posts.
func TestWhatAControlNeedsDependsOnWhereItIsAndATableNeedsAKeyOnlyForWhatPosts(t *testing.T) {
	t.Parallel()

	detail := webui.Page[parcelArgs]{Path: "/parcel/{name}", View: webui.Stack{}}
	load := func(context.Context) ([]parcel, webui.Loaded) { return parcelsData, webui.Ready() }
	cols := []webui.Accessor[parcel]{parcelName}
	run := func(context.Context, parcel) (webui.Outcome, error) { return webui.Success("done"), nil }
	for name, tc := range map[string]struct {
		table webui.Table[parcel]
		names []string // nil for a table that compiles
	}{
		"a link in the actions has no label":  {webui.Table[parcel]{Actions: []webui.Control[parcel]{webui.Link[parcel, parcelArgs]{Page: detail, Args: func(context.Context, parcel) parcelArgs { return parcelArgs{} }}}}, []string{"Table.Actions[0]", "Label"}},
		"an href in the actions has no label": {webui.Table[parcel]{Actions: []webui.Control[parcel]{webui.Href[parcel]{URL: func(parcel) string { return "/x" }}}}, []string{"Table.Actions[0]", "Label"}},
		"an href with no url":                 {webui.Table[parcel]{Actions: []webui.Control[parcel]{webui.Href[parcel]{Label: "Go"}}}, []string{"Table.Actions[0]", "Href", "URL"}},
		"an action needs a key":               {webui.Table[parcel]{Actions: []webui.Control[parcel]{webui.Action[parcel]{Label: "Drop", Run: run}}}, []string{"Table", "Key"}},
		"a click that is an action needs one": {webui.Table[parcel]{RowClick: webui.Action[parcel]{Run: run}}, []string{"Table", "Key"}},
		"a nil control":                       {webui.Table[parcel]{Actions: []webui.Control[parcel]{nil}}, []string{"Table.Actions[0]"}},
		"a click that asks first":             {webui.Table[parcel]{Key: func(p parcel) string { return p.Name }, RowClick: webui.Action[parcel]{Run: run, Confirm: "Sure?"}}, []string{"Table.RowClick", "Confirm"}},
		"links and addresses need no key":     {webui.Table[parcel]{Actions: []webui.Control[parcel]{webui.Href[parcel]{Label: "Go", URL: func(parcel) string { return "/x" }}}, RowClick: webui.Href[parcel]{URL: func(parcel) string { return "/x" }}}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.table.Loader, tc.table.Columns = webui.RowsLoader[parcel]{Load: load}, cols
			app := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", View: tc.table}, detail}}
			_, err := app.Compile("/admin")
			if tc.names == nil {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			refuses(t, err.Error(), tc.names...)
		})
	}
}
