package webui_test

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
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

// pageOf reads the page that a GET of target is answered with.
func pageOf(t *testing.T, h http.Handler, target string) *page.Page {
	t.Helper()

	return page.Parse(t, serve(h, http.MethodGet, target).Body.String())
}

// sidebarNav is the main navigation of a page: the entries as a reader meets them, with the captions of the sections between them.
func sidebarNav(t *testing.T, p *page.Page) page.El {
	t.Helper()

	return page.One(t, "main navigation", p.ByRole("navigation", "Main"))
}

// outline lists what a reader of the sidebar is told, in order: the name of each entry, and the caption of each section after a "#".
func outline(t *testing.T, p *page.Page) []string {
	t.Helper()

	var out []string
	for _, c := range sidebarNav(t, p).Children() {
		if c.Role() == "link" {
			out = append(out, c.Name())
		} else if text := c.Text(); text != "" {
			out = append(out, "# "+text)
		}
	}
	return out
}

// activeEntries are the addresses of the sidebar's entries that say they are where the reader is.
func activeEntries(t *testing.T, h http.Handler, target string) []string {
	t.Helper()

	var out []string
	for _, l := range sidebarNav(t, pageOf(t, h, target)).Links() {
		if l.Current {
			out = append(out, l.Href)
		}
	}
	return out
}

// linksTo are the links of a page that lead to href.
func linksTo(p *page.Page, href string) []page.Link {
	var out []page.Link
	for _, l := range p.Links() {
		if l.Href == href {
			out = append(out, l)
		}
	}
	return out
}

// crumbs is the breadcrumb of a page.
func crumbs(t *testing.T, p *page.Page) page.El {
	t.Helper()

	return page.One(t, "breadcrumb", p.ByRole("navigation", "Breadcrumb"))
}

// named counts the links of a list that are called name.
func named(links []page.Link, name string) int {
	n := 0
	for _, l := range links {
		if l.Name == name {
			n++
		}
	}
	return n
}

// iconOf is the class of the icon that an entry draws, which names a Font Awesome icon; "" for an entry with none.
func iconOf(e page.El) string {
	for _, c := range e.Children() {
		if c.Attr("aria-hidden") == "true" {
			return c.Attr("class")
		}
	}
	return ""
}

// trail is the breadcrumb as a reader meets it, in order: each crumb that is a link as "name>href", and the one that is where the reader is by its name.
func trail(t *testing.T, p *page.Page) []string {
	t.Helper()

	var out []string
	for _, c := range crumbs(t, p).Children() {
		if c.Role() == "link" {
			out = append(out, c.Name()+">"+c.Attr("href"))
		} else if text := c.Text(); text != "" {
			out = append(out, text)
		}
	}
	return out
}

// externalLinks is the menu of the app's links to places outside it, behind the button that is called External links.
func externalLinks(t *testing.T, p *page.Page) page.El {
	t.Helper()

	for _, g := range p.ByRole("group", "") {
		for _, c := range g.Children() {
			if c.Tag() == "summary" && c.Name() == "External links" {
				return g
			}
		}
	}
	t.Fatalf("no menu of external links")
	return page.El{}
}

// menuOutline lists what a reader of a menu is told, in order: the name of each link, and the caption of each section after a "#". A caption is text with
// nothing inside it.
func menuOutline(e page.El) []string {
	var out []string
	for _, c := range e.Children() {
		switch {
		case c.Role() == "link":
			out = append(out, c.Name())
		case c.Tag() == "summary":
		case len(c.Children()) == 0 && c.Text() != "":
			out = append(out, "# "+c.Text())
		default:
			out = append(out, menuOutline(c)...)
		}
	}
	return out
}

// hasLink is whether a page has a link called name that leads to href.
func hasLink(p *page.Page, name, href string) bool {
	for _, l := range p.Links() {
		if l.Name == name && l.Href == href {
			return true
		}
	}
	return false
}

// column is the header of a column of a table, by what it is called.
func column(t *testing.T, p *page.Page, table, name string) page.El {
	t.Helper()

	return page.One(t, "column "+name, page.One(t, "table "+table, p.ByRole("table", table)).ByRole("columnheader", name))
}

// carried are the values, by name, that the form of a control posts besides the control itself: what applying a filter carries along.
func carried(t *testing.T, p *page.Page, control string) map[string][]string {
	t.Helper()

	f, ok := p.Field(control)
	if !ok {
		t.Fatalf("no control named %s", control)
	}
	out := map[string][]string{}
	for _, g := range p.Fields() {
		if g.Form == f.Form && g.Type == "hidden" {
			out[g.Name] = append(out[g.Name], g.Value)
		}
	}
	return out
}

// fieldsNamed are the controls of a page that post name, in order.
func fieldsNamed(p *page.Page, name string) []page.Field {
	var out []page.Field
	for _, f := range p.Fields() {
		if f.Name == name {
			out = append(out, f)
		}
	}
	return out
}

// items are the texts of the items of the lists of a page, in order, as a reader meets them.
func items(p *page.Page) []string {
	var out []string
	for _, li := range p.ByRole("listitem", "") {
		out = append(out, li.Text())
	}
	return out
}

// moment is a time that a page shows: the instant it holds, and the text a visitor reads for it.
type moment struct{ At, Text string }

// moments are the times in the main part of the page, in the order they are met.
func moments(t *testing.T, p *page.Page) []moment {
	t.Helper()
	var out []moment
	for _, e := range page.One(t, "main", p.ByRole("main", "")).ByAttr("datetime", "") {
		out = append(out, moment{At: e.Attr("datetime"), Text: e.Text()})
	}
	return out
}

// toast is a message that a page gives the visitor about what they did: an alert when it is a failure, a status otherwise, and what it says.
type toast struct{ Role, Text string }

// toasts are the messages of a page, in order. The region that holds them is always there, and says nothing until one is put in it.
func toasts(t *testing.T, body string) []toast {
	t.Helper()
	var out []toast
	for _, e := range page.Parse(t, body).Live() {
		if e.Text() != "" {
			out = append(out, toast{e.Role(), e.Text()})
		}
	}
	return out
}

// control is a button of a page that does an action: what it is called, what it posts as _act (the position its action was declared at, and for a row,
// the row), and whether it is drawn disabled.
type control struct {
	Name, Posts string
	Disabled    bool
}

// controls are the buttons below the element that do an action, in the order they are met.
func controls(e page.El) []control {
	var out []control
	for _, b := range e.ByAttr("name", "_act") {
		if b.Tag() == "button" {
			out = append(out, control{b.Name(), b.Attr("value"), b.Has("disabled")})
		}
	}
	return out
}

// mainOf is the main part of a page.
func mainOf(t *testing.T, p *page.Page) page.El {
	t.Helper()
	return page.One(t, "main", p.ByRole("main", ""))
}

// panels are the panels of a page by the address of each (data-view) and the title it carries, in the order they are met.
func panels(p *page.Page) [][2]string {
	var out [][2]string
	for _, v := range p.Views() {
		title := ""
		if hs := v.ByRole("heading", ""); len(hs) > 0 {
			title = hs[0].Name()
		}
		out = append(out, [2]string{v.Attr("data-view"), title})
	}
	return out
}

// shown is whether a visitor reads text in the body of a response.
func shown(t *testing.T, body, text string) bool {
	t.Helper()
	return len(page.Parse(t, body).ByText(text)) > 0
}

// describedBy is what a control is described by, as a reader says it: the text of each element that its aria-describedby names, drawn or not, in that order.
func describedBy(t *testing.T, p *page.Page, control page.El) []string {
	t.Helper()
	var out []string
	for _, id := range strings.Fields(control.Attr("aria-describedby")) {
		out = append(out, page.One(t, "the element "+id, p.ByAttr("id", id)).AllText())
	}
	return out
}

// cellsOf are the values of one column of the table, by its header, in the order of the rows.
func cellsOf(t *testing.T, table page.El, header string) []string {
	t.Helper()
	i := slices.Index(table.Headers(), header)
	if i < 0 {
		t.Fatalf("no column %q in %v", header, table.Headers())
	}
	var out []string
	for _, row := range table.Rows() {
		out = append(out, row[i])
	}
	return out
}
