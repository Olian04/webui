package page

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

const sample = `<!doctype html><html><head><title>Devices — Acme</title><script>var x = "<b>not text</b>";</script></head><body>
<header class="topbar"><a href="/admin/" aria-label="Acme home">Acme</a></header>
<aside id="sidebar"><nav aria-label="Main">
  <a class="whatever-class" href="/admin/device" aria-current="page"><i aria-hidden="true">★</i> Devices</a>
  <a href="/admin/alert" title="Alerts">Alerts <span>3</span></a>
  <a href="https://example.com/docs" target="_blank" rel="noopener noreferrer">Docs</a>
  <a>not a link</a>
</nav></aside>
<main id="main">
  <h1 class="sr-only">Devices</h1>
  <table aria-label="Devices">
    <thead><tr><th scope="col" aria-sort="none"><a href="?sort=id">ID</a> <details><summary aria-label="Filter ID"><i aria-hidden="true">F</i></summary><input name="id.filter" aria-label="Filter ID"><button>Apply</button></details></th><th scope="col">IP</th></tr></thead>
    <tbody>
      <tr><td>dev_1</td><td>10.0.0.1</td></tr>
      <tr><td>dev_2</td><td><span class="dim" aria-hidden="true">—</span></td></tr>
    </tbody>
  </table>
  <form action="/admin/device" method="post">
    <input type="hidden" name="_view" value="p.0">
    <label for="ip">IP address</label><input id="ip" name="ip" value="10.0.0.1" required>
    <label>Enabled <input type="checkbox" name="on" checked></label>
    <label for="kind">Kind</label>
    <select id="kind" name="kind"><option value="a">Alpha</option><option value="b" selected>Beta</option></select>
    <textarea name="note" aria-label="Note">
two
lines</textarea>
    <input type="radio" name="size" value="s"><input type="radio" name="size" value="m" checked>
    <button type="submit" name="_act" value="save">Save</button>
    <button type="button" disabled aria-label="More actions">…</button>
    <span class="tip-body" aria-hidden="true">Between 0 and 100.</span>
    <div hidden>never shown</div>
  </form>
  <div role="status" id="toast">Saved <b>dev_1</b></div>
  <div class="panel" data-view="p.0" data-refresh-after="5"><h2>Collector</h2></div>
  <noscript><p>Turn on script for the live view.</p></noscript>
</main>
</body></html>`

func TestAPageIsReadByRoleAndName(t *testing.T) {
	t.Parallel()

	p := Parse(t, sample)
	assert.Equal(t, p.Title(), "Devices — Acme")
	assert.Equal(t, len(p.ByRole("navigation", "Main")), 1)
	assert.Equal(t, len(p.ByRole("navigation", "Breadcrumb")), 0)
	assert.Equal(t, len(p.ByRole("complementary", "")), 1)
	assert.Equal(t, len(p.ByRole("banner", "")), 1)
	assert.Equal(t, len(p.ByRole("main", "")), 1)
	assert.Equal(t, len(p.ByRole("heading", "Devices")), 1)
	assert.Equal(t, One(t, "the table", p.ByRole("table", "Devices")).Tag(), "table")
	assert.Equal(t, len(p.ByRole("button", "Save")), 1)
	assert.Equal(t, len(p.ByRole("button", "More actions")), 1)
	assert.Equal(t, len(p.ByRole("button", "")), 4) // Save, More actions, Apply and the summary of the filter, which is a button to a reader
	assert.Equal(t, len(p.ByRoleContaining("link", "Alert")), 1)
	assert.Equal(t, len(p.ByRoleMatching("link", regexp.MustCompile(`^Dev`))), 1)
}

func TestALinkIsWhatItIsCalledAndWhereItLeads(t *testing.T) {
	t.Parallel()

	got := Parse(t, sample).Links()
	assert.DeepEqual(t, got, []Link{
		{Name: "Acme home", Href: "/admin/"},
		{Name: "Devices", Href: "/admin/device", Current: true},
		{Name: "Alerts 3", Href: "/admin/alert"},
		{Name: "Docs", Href: "https://example.com/docs", NewTab: true},
		{Name: "ID", Href: "?sort=id"},
	})
}

func TestATableIsItsHeadersAndItsRows(t *testing.T) {
	t.Parallel()

	table := One(t, "the table", Parse(t, sample).ByRole("table", "Devices"))
	assert.DeepEqual(t, table.Headers(), []string{"ID", "IP"})
	// A cell that holds only what is aria-hidden is empty to a reader.
	assert.DeepEqual(t, table.Rows(), [][]string{{"dev_1", "10.0.0.1"}, {"dev_2", ""}})
}

func TestAFieldPostsWhatABrowserWouldPost(t *testing.T) {
	t.Parallel()

	p := Parse(t, sample)
	view, ok := p.Field("_view")
	assert.True(t, ok)
	assert.DeepEqual(t, view, Field{Name: "_view", Type: "hidden", Value: "p.0"})

	ip, _ := p.Field("ip")
	assert.True(t, ip.Required && !ip.Disabled)
	assert.Equal(t, ip.Value, "10.0.0.1")

	on, _ := p.Field("on")
	assert.True(t, on.Checked)

	kind, _ := p.Field("kind")
	assert.Equal(t, kind.Value, "b")
	assert.DeepEqual(t, kind.Options, []Option{{Value: "a", Text: "Alpha"}, {Value: "b", Text: "Beta", Selected: true}})

	note, _ := p.Field("note")
	assert.Equal(t, note.Value, "two\nlines") // the newline that a textarea starts with is not its value

	size, _ := p.Field("size") // a group of radios is the one that is checked
	assert.Equal(t, size.Value, "m")
	assert.True(t, size.Checked)

	_, ok = p.Field("nothing")
	assert.False(t, ok)
}

func TestAControlIsFoundByItsLabel(t *testing.T) {
	t.Parallel()

	p := Parse(t, sample)
	assert.Equal(t, One(t, "IP address", p.ByLabel("IP address")).Attr("name"), "ip")
	assert.Equal(t, One(t, "Enabled", p.ByLabel("Enabled")).Attr("name"), "on") // a control that its label wraps
	assert.Equal(t, One(t, "Kind", p.ByLabel("Kind")).Attr("name"), "kind")
	assert.Equal(t, One(t, "Note", p.ByLabel("Note")).Attr("name"), "note") // an aria-label
}

func TestTextIsWhatIsShownAndAllTextKeepsWhatOnlyAReaderAsksFor(t *testing.T) {
	t.Parallel()

	p := Parse(t, sample)
	main := One(t, "main", p.ByRole("main", ""))
	assert.True(t, !strings.Contains(main.Text(), "Between 0 and 100."))
	assert.True(t, strings.Contains(main.AllText(), "Between 0 and 100."))
	assert.True(t, !strings.Contains(main.AllText(), "never shown"))
	assert.True(t, !strings.Contains(main.Text(), "not text"))                         // a script is not text
	assert.True(t, strings.Contains(main.Text(), "Turn on script for the live view.")) // a client with no script reads <noscript>
	assert.Equal(t, One(t, "the toast", p.Live()).Text(), "Saved dev_1")
}

func TestAnElementIsFoundByTextAndByTheAttributesThatScriptsNeed(t *testing.T) {
	t.Parallel()

	p := Parse(t, sample)
	got := p.ByText("Saved dev_1")
	assert.Equal(t, len(got), 1)
	assert.Equal(t, got[0].Attr("id"), "toast") // the innermost that has it, not main and body above it

	views := p.Views()
	assert.Equal(t, len(views), 1)
	assert.Equal(t, views[0].Attr("data-view"), "p.0")
	assert.Equal(t, views[0].Attr("data-refresh-after"), "5")
	assert.True(t, views[0].Has("data-refresh-after"))
	assert.Equal(t, len(p.ByAttr("data-view", "p.1")), 0)
	assert.Equal(t, len(p.ByAttr("name", "ip")), 1)
}

func TestHiddenIsWhatNoReaderSees(t *testing.T) {
	t.Parallel()

	p := Parse(t, sample)
	tip := One(t, "the popover", p.ByAllText("Between 0 and 100."))
	assert.True(t, tip.Hidden())
	assert.False(t, One(t, "the table", p.ByRole("table", "Devices")).Hidden())
}

func TestASearchBelowAnElementDoesNotFindTheElementItself(t *testing.T) {
	t.Parallel()

	nav := One(t, "the navigation", Parse(t, sample).ByRole("navigation", "Main"))
	assert.Equal(t, len(nav.ByRole("navigation", "")), 0)
	assert.Equal(t, len(nav.Links()), 3) // the <a> with no href is not a link
}

func TestAFragmentIsWhatAPanelIsAnsweredWith(t *testing.T) {
	t.Parallel()

	assert.True(t, IsFragment(`<div data-view="p.0"><h2>Collector</h2></div>`))
	assert.True(t, IsFragment("\n  <li>x</li>"))
	assert.False(t, IsFragment(sample))
	assert.False(t, IsFragment("<HTML><body></body></HTML>"))
	assert.Equal(t, len(Parse(t, `<div data-view="p.0"><h2>Collector</h2></div>`).ByRole("heading", "Collector")), 1)
}

func TestAnAccessibleNameFollowsLabelledByAndTheNativeRules(t *testing.T) {
	t.Parallel()

	p := Parse(t, `<h2 id="h">Alerts</h2><section aria-labelledby="h"></section>
<table><caption>Open alerts</caption><tr><td>x</td></tr></table>
<img src="a.png" alt="A chart"><img src="b.png" alt="">
<input type="submit" value="Go"><button title="Close"></button>`)
	assert.Equal(t, len(p.ByRole("region", "Alerts")), 1)
	assert.Equal(t, len(p.ByRole("table", "Open alerts")), 1)
	assert.Equal(t, len(p.ByRole("img", "A chart")), 1)
	assert.Equal(t, len(p.ByRole("presentation", "")), 1) // an empty alt says the picture is decoration
	assert.Equal(t, len(p.ByRole("button", "Go")), 1)
	assert.Equal(t, len(p.ByRole("button", "Close")), 1)
}
