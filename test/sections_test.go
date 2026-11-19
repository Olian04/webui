package webui_test

import (
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// A section of the sidebar is a category that the visitor opens and shuts: it is open when it holds the page on screen. An app can give a section an icon, and a place in the
// order, in App.Sections.

func sectionsApp(sections ...webui.NavSection) *webui.App {
	entry := func(path webui.PageID[webui.NoArgs], label, section string) webui.Page[webui.NoArgs] {
		return webui.Page[webui.NoArgs]{Path: path, Nav: webui.Nav{Label: label, Icon: "house", Section: section}, View: webui.Stack{}}
	}
	return &webui.App{
		Sections: sections,
		Pages: webui.Pages{
			entry("/home", "Home", ""),
			entry("/a", "Alpha", "Fleet"),
			entry("/b", "Beta", "Fleet"),
			entry("/c", "Gamma", "Access"),
			entry("/d", "Delta", "Extras"),
		},
	}
}

// sections are the categories of the sidebar, in order, each as its name and whether it is open, with an icon when it has one.
func sections(t *testing.T, p *page.Page) []string {
	t.Helper()

	var out []string
	for _, c := range sidebarNav(t, p).Children() {
		if c.Tag() != "details" {
			continue
		}
		for _, part := range c.Children() {
			if part.Tag() != "summary" {
				continue
			}
			s := part.Text()
			// An icon of its own is drawn before the name, and the chevron after it, which every category has.
			if kids := part.Children(); len(kids) == 3 {
				s = "[" + kids[0].Attr("class") + "] " + s
			}
			if c.Has("open") {
				s += " (open)"
			}
			out = append(out, s)
		}
	}
	return out
}

func TestASectionIsACategoryThatIsOpenWhenItHoldsThePageOnScreen(t *testing.T) {
	t.Parallel()

	h := sectionsApp().MustCompile("/admin")
	assert.DeepEqual(t, sections(t, pageOf(t, h, "/admin/a")), []string{"Fleet (open)", "Access", "Extras"})
	assert.DeepEqual(t, sections(t, pageOf(t, h, "/admin/c")), []string{"Fleet", "Access (open)", "Extras"})
	assert.DeepEqual(t, sections(t, pageOf(t, h, "/admin/home")), []string{"Fleet", "Access", "Extras"}) // nothing is open that does not hold it

	// What is in a shut category is still there, in the page, for a reader who opens it.
	assert.DeepEqual(t, outline(t, pageOf(t, h, "/admin/home")), []string{"Home", "# Fleet", "Alpha", "Beta", "# Access", "Gamma", "# Extras", "Delta"})
}

func TestASectionTheAppNamedHasItsIconAndStandsInTheOrderOfTheListBeforeTheOthers(t *testing.T) {
	t.Parallel()

	h := sectionsApp(webui.NavSection{Name: "Extras", Icon: "book"}, webui.NavSection{Name: "Access", Icon: "server"}).MustCompile("/admin")
	got := sections(t, pageOf(t, h, "/admin/home"))
	// The named ones first, in the app's order, with their icons; the one that was not named after them, with none.
	assert.Equal(t, len(got), 3)
	assert.True(t, strings.HasPrefix(got[0], "[") && strings.HasSuffix(got[0], "Extras") && strings.Contains(got[0], "fa-book"))
	assert.True(t, strings.HasPrefix(got[1], "[") && strings.HasSuffix(got[1], "Access") && strings.Contains(got[1], "fa-server"))
	assert.Equal(t, got[2], "Fleet")
	// The entries that belong to no section are first, as they are.
	assert.Equal(t, outline(t, pageOf(t, h, "/admin/home"))[0], "Home")
}

func TestASectionThatNoEntryGivesItselfIsNotDrawn(t *testing.T) {
	t.Parallel()

	h := sectionsApp(webui.NavSection{Name: "Nothing", Icon: "book"}).MustCompile("/admin")
	assert.DeepEqual(t, sections(t, pageOf(t, h, "/admin/home")), []string{"Fleet", "Access", "Extras"})
}

func TestASectionOfTheAppMustHaveANameOfItsOwnAndAnIconThatThereIs(t *testing.T) {
	t.Parallel()

	errorsOf := func(items ...webui.NavSection) string {
		var all string
		for _, e := range compileErrors(t, *sectionsApp(items...)) {
			all += e.Error() + "\n"
		}
		return all
	}
	refuses(t, errorsOf(webui.NavSection{Icon: "book"}), "App.Sections[0]", "Name")
	refuses(t, errorsOf(webui.NavSection{Name: "Fleet"}, webui.NavSection{Name: "Fleet"}), "App.Sections[1]", `"Fleet"`)
	refuses(t, errorsOf(webui.NavSection{Name: "Fleet", Icon: "rocket-ship"}), "App.Sections[0]", `"rocket-ship"`, "Icon")
	_, err := sectionsApp(webui.NavSection{Name: "Fleet", Icon: "display"}).Compile("/admin")
	assert.NoError(t, err)
}
