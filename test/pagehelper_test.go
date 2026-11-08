package webui_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// These are the page helper held against what the library really writes: a sidebar, a table, a form, a notice, and the fragment that a request for
// one panel is answered with. The helper's own tests are in test/util/page, on HTML that was written to show each thing it reads.

func TestThePageHelperReadsASidebarAndATableOfARealPage(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.False(t, page.IsFragment(body))
	p := page.Parse(t, body)

	main := page.One(t, "the main navigation", p.ByRole("navigation", "Main"))
	assert.DeepEqual(t, main.Links(), []page.Link{{Name: "Devices", Href: "/admin/device", Current: true}})

	table := page.One(t, "the Devices table", p.ByRole("table", "Devices"))
	assert.Equal(t, table.Headers()[0], "ID")
	rows := table.Rows()
	assert.Equal(t, len(rows), 10)
	assert.Equal(t, rows[0][0], "dev00")

	// A row leads to its page.
	var leads []string
	for _, l := range table.Links() {
		leads = append(leads, l.Href)
	}
	assert.True(t, len(leads) > 10)
	assert.Equal(t, len(p.ByRoleContaining("link", "dev00")), 1)

	// The panel that can be asked for alone is named by its position.
	views := p.Views()
	assert.Equal(t, len(views), 1)
	assert.Equal(t, views[0].Attr("data-view"), "p")
}

func TestThePageHelperReadsTheFragmentThatAPanelIsAnsweredWith(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serveView(h, "/admin/device", "p").Body.String()
	assert.True(t, page.IsFragment(body))
	assert.Equal(t, len(page.Parse(t, body).ByRole("table", "Devices")), 1)
	assert.Equal(t, len(page.Parse(t, body).ByRole("navigation", "Main")), 0) // no sidebar: it is the panel and nothing around it
}

func TestThePageHelperReadsAFormItsFieldsAndItsNotices(t *testing.T) {
	t.Parallel()

	h := outcomeApp()
	form := page.Parse(t, serve(h, http.MethodGet, "/admin/device/a").Body.String())
	view, ok := form.Field("_view")
	assert.True(t, ok)
	assert.Equal(t, view.Value, "p")
	ip, ok := form.Field("f1")
	assert.True(t, ok)
	assert.Equal(t, ip.Value, "10.0.0.1")

	// An accepted post is followed by a page that says so, in a live region.
	saved := follow(h, post(h, "/admin/device/a", url.Values{"_view": {"p"}, "f1": {"10.0.0.1"}, "f2": {"5"}}))
	notices := page.Parse(t, saved.Body.String()).Live()
	assert.True(t, len(notices) > 0)
	found := false
	for _, n := range notices {
		found = found || n.Text() == "Saved"
	}
	assert.True(t, found)

	// A refused one answers with the page again, the form drawn with what was typed, and says what is wrong with the field.
	refused := post(h, "/admin/device/a", url.Values{"_view": {"p"}, "f1": {"10.0.0.4"}, "f2": {"5"}})
	assert.Equal(t, refused.Code, http.StatusUnprocessableEntity)
	again := page.Parse(t, refused.Body.String())
	typed, _ := again.Field("f1")
	assert.Equal(t, typed.Value, "10.0.0.4")
	assert.True(t, len(again.ByText("taken")) > 0)
}
