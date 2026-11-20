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

// A NavGroup of the sidebar is a category that the visitor opens and shuts: it is open when it holds the page on screen. It has an icon of its own, and stands where the app wrote it.

func groupsApp(navigation func(a, b, c, d, home webui.Page[webui.NoArgs]) []webui.NavItem) http.Handler {
	p := func(path webui.PageID[webui.NoArgs]) webui.Page[webui.NoArgs] {
		return webui.Page[webui.NoArgs]{Path: path, View: webui.Stack{}}
	}
	home, a, b, c, d := p("/home"), p("/a"), p("/b"), p("/c"), p("/d")
	return webui.App{Navigation: navigation(a, b, c, d, home), Pages: webui.Pages{home, a, b, c, d}}.MustCompile("/admin")
}

func defaultGroups(a, b, c, d, home webui.Page[webui.NoArgs]) []webui.NavItem {
	return []webui.NavItem{
		webui.NavPage{Page: home, Label: "Home"},
		webui.NavGroup{Label: "Fleet", Items: []webui.NavItem{webui.NavPage{Page: a, Label: "Alpha"}, webui.NavPage{Page: b, Label: "Beta"}}},
		webui.NavGroup{Label: "Access", Items: []webui.NavItem{webui.NavPage{Page: c, Label: "Gamma"}}},
		webui.NavGroup{Label: "Extras", Items: []webui.NavItem{webui.NavPage{Page: d, Label: "Delta"}}},
	}
}

// A category of the sidebar is a disclosure of an exclusive accordion: they share one name, so that the browser closes the one that was open when another is opened, with no script.
func TestTheCategoriesOfTheSidebarShareOneNameSoThatOnlyOneIsOpenAtATime(t *testing.T) {
	t.Parallel()

	h := groupsApp(defaultGroups)
	var names []string
	for _, c := range sidebarNav(t, pageOf(t, h, "/admin/a")).Children() {
		if c.Tag() == "details" {
			names = append(names, c.Attr("name"))
		}
	}
	assert.Equal(t, len(names), 3)
	assert.True(t, names[0] != "")
	assert.DeepEqual(t, names, []string{names[0], names[0], names[0]})
}

// groups are the categories of the sidebar, in order, each as its name and whether it is open, with an icon when it has one.
func groups(t *testing.T, p *page.Page) []string {
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

func TestAGroupIsACategoryThatIsOpenWhenItHoldsThePageOnScreen(t *testing.T) {
	t.Parallel()

	h := groupsApp(defaultGroups)
	assert.DeepEqual(t, groups(t, pageOf(t, h, "/admin/a")), []string{"Fleet (open)", "Access", "Extras"})
	assert.DeepEqual(t, groups(t, pageOf(t, h, "/admin/c")), []string{"Fleet", "Access (open)", "Extras"})
	assert.DeepEqual(t, groups(t, pageOf(t, h, "/admin/home")), []string{"Fleet", "Access", "Extras"}) // nothing is open that does not hold it

	// What is in a shut category is still there, in the page, for a reader who opens it.
	assert.DeepEqual(t, outline(t, pageOf(t, h, "/admin/home")), []string{"Home", "# Fleet", "Alpha", "Beta", "# Access", "Gamma", "# Extras", "Delta"})
}

func TestAGroupHasItsIconAndStandsWhereTheAppWroteIt(t *testing.T) {
	t.Parallel()

	h := groupsApp(func(a, _, c, d, _ webui.Page[webui.NoArgs]) []webui.NavItem {
		return []webui.NavItem{
			webui.NavGroup{Label: "Extras", Icon: "book", Items: []webui.NavItem{webui.NavPage{Page: d, Label: "Delta"}}},
			webui.NavGroup{Label: "Access", Icon: "server", Items: []webui.NavItem{webui.NavPage{Page: c, Label: "Gamma"}}},
			webui.NavGroup{Label: "Fleet", Items: []webui.NavItem{webui.NavPage{Page: a, Label: "Alpha"}}},
		}
	})
	got := groups(t, pageOf(t, h, "/admin/home"))
	assert.Equal(t, len(got), 3)
	assert.True(t, strings.HasPrefix(got[0], "[") && strings.HasSuffix(got[0], "Extras") && strings.Contains(got[0], "fa-book"))
	assert.True(t, strings.HasPrefix(got[1], "[") && strings.HasSuffix(got[1], "Access") && strings.Contains(got[1], "fa-server"))
	assert.Equal(t, got[2], "Fleet") // no icon of its own
}

func TestAnAddressInTheSidebarOpensWhereItSaysAndInATabOfItsOwnWhenItIsOnTheWeb(t *testing.T) {
	t.Parallel()

	h := groupsApp(func(_, _, _, _, home webui.Page[webui.NoArgs]) []webui.NavItem {
		return []webui.NavItem{
			webui.NavPage{Page: home, Label: "Home"},
			webui.NavGroup{Label: "Help", Items: []webui.NavItem{
				webui.NavURL{Label: "Docs", URL: "https://example.com/docs"},
				webui.NavURL{Label: "Log out", URL: "/logout"},
			}},
		}
	})
	p := pageOf(t, h, "/admin/home")
	// the category is shut on this page, so its entries are read by their address
	docs := page.One(t, "the docs entry", sidebarNav(t, p).ByAttr("href", "https://example.com/docs"))
	assert.Equal(t, docs.AllText(), "Docs(opens in a new tab)")
	assert.Equal(t, docs.Attr("target"), "_blank")
	logout := page.One(t, "the log out entry", sidebarNav(t, p).ByAttr("href", "/logout"))
	assert.Equal(t, logout.Attr("href"), "/logout")
	assert.False(t, logout.Has("target"))
}

func TestAGroupIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	a := webui.Page[webui.NoArgs]{Path: "/a", View: webui.Stack{}}
	errorsOf := func(items ...webui.NavItem) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Navigation: items, Pages: webui.Pages{a}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	item := webui.NavPage{Page: a, Label: "A"}
	refuses(t, errorsOf(webui.NavGroup{Items: []webui.NavItem{item}}), "App.Navigation[0]", "Label")
	refuses(t, errorsOf(webui.NavGroup{Label: "Fleet"}), "App.Navigation[0]", "Items")
	refuses(t, errorsOf(webui.NavGroup{Label: "Fleet", Icon: "rocket-ship", Items: []webui.NavItem{item}}), "App.Navigation[0]", `"rocket-ship"`, "Icon")
	refuses(t, errorsOf(webui.NavGroup{Label: "Outer", Items: []webui.NavItem{webui.NavGroup{Label: "Inner", Items: []webui.NavItem{item}}}}), "App.Navigation[0].Items[0]", "group")
	refuses(t, errorsOf(item, webui.NavGroup{Label: "Fleet", Items: []webui.NavItem{item}}), "App.Navigation[1].Items[0]", `"/a"`) // a page is listed once
	refuses(t, errorsOf(webui.NavURL{Label: "Out", URL: "javascript:alert(1)"}), "App.Navigation[0]", "URL")
	refuses(t, errorsOf(webui.NavPage{Label: "Nowhere"}), "App.Navigation[0]", "Page")
	_, err := webui.App{Navigation: []webui.NavItem{webui.NavGroup{Label: "Fleet", Icon: "display", Items: []webui.NavItem{item}}}, Pages: webui.Pages{a}}.Compile("/admin")
	assert.NoError(t, err)
}

func TestTheMenuHoldsAddressesAndPagesInGroupsAndNotSeveralPagesAtOnce(t *testing.T) {
	t.Parallel()

	a := webui.Page[webui.NoArgs]{Path: "/a", View: webui.Stack{}}
	site := webui.Page[siteArgs]{Path: "/site/{name}", View: webui.Stack{}}
	app := func(items ...webui.NavItem) webui.App {
		return webui.App{Navigation: []webui.NavItem{webui.NavPage{Page: a, Label: "A"}}, Menu: items, Pages: webui.Pages{a, site}}
	}
	_, err := app(webui.NavGroup{Label: "Help", Items: []webui.NavItem{webui.NavURL{Label: "Docs", URL: "/docs"}, webui.NavPage{Page: a, Label: "Open A"}}}).Compile("/admin")
	assert.NoError(t, err) // a page may be in the menu as well as in the sidebar

	var all string
	for _, e := range compileErrors(t, app(webui.NavPages[siteArgs]{Page: site, Load: siteAddresses("x"), Label: siteLabel})) {
		all += e.Error() + "\n"
	}
	refuses(t, all, "App.Menu[0]", "App.Navigation")
}

func TestAPageInTheMenuIsLeftOutForAVisitorWhoseGuardRefusesIt(t *testing.T) {
	t.Parallel()

	build := func(allowed bool) http.Handler {
		home := webui.Page[webui.NoArgs]{Path: "/home", View: webui.Stack{}}
		account := webui.Page[webui.NoArgs]{Path: "/account", View: webui.Stack{}, Guard: func(context.Context, webui.NoArgs) error {
			if allowed {
				return nil
			}
			return errors.New("not for you")
		}}
		return webui.App{
			Navigation: []webui.NavItem{webui.NavPage{Page: home, Label: "Home"}},
			Menu:       []webui.NavItem{webui.NavURL{Label: "Docs", URL: "/docs"}, webui.NavPage{Page: account, Label: "Account"}},
			Pages:      webui.Pages{home, account},
		}.MustCompile("/admin")
	}
	assert.DeepEqual(t, menuOutline(externalLinks(t, pageOf(t, build(true), "/admin/home"))), []string{"Docs", "Account"})
	assert.DeepEqual(t, menuOutline(externalLinks(t, pageOf(t, build(false), "/admin/home"))), []string{"Docs"})
}

// A category is a heading over its entries, so one with nothing left to open is not drawn: its pages are refused to this visitor, or the page that it lists several of has no
// addresses to list, or failed to load them. The same holds for a group in the menu, whose caption stands above its first entry.
func TestAGroupWithNothingLeftInItIsNotDrawn(t *testing.T) {
	t.Parallel()

	open := webui.Page[webui.NoArgs]{Path: "/open", View: webui.Stack{}}
	shut := webui.Page[webui.NoArgs]{Path: "/shut", View: webui.Stack{}, Guard: func(context.Context, webui.NoArgs) error { return errors.New("not for you") }}
	site := webui.Page[siteArgs]{Path: "/site/{name}", View: webui.Stack{}}
	broken := webui.Page[siteArgs]{Path: "/broken/{name}", View: webui.Stack{}}
	barred := webui.Page[siteArgs]{Path: "/barred/{name}", View: webui.Stack{}, Guard: func(context.Context, siteArgs) error { return errors.New("not for you") }}
	none := func(context.Context) ([]siteArgs, webui.Loaded) { return nil, webui.Ready() }
	failed := func(context.Context) ([]siteArgs, webui.Loaded) {
		return []siteArgs{{Name: "x"}}, webui.Failed(errors.New("backend is down"))
	}
	h := webui.App{
		Navigation: []webui.NavItem{
			webui.NavGroup{Label: "Refused", Items: []webui.NavItem{webui.NavPage{Page: shut, Label: "Shut"}}},
			webui.NavGroup{Label: "Listing nothing", Items: []webui.NavItem{webui.NavPages[siteArgs]{Page: site, Load: none, Label: siteLabel}}},
			webui.NavGroup{Label: "Listing fails", Items: []webui.NavItem{webui.NavPages[siteArgs]{Page: broken, Load: failed, Label: siteLabel}}},
			webui.NavGroup{Label: "Every address refused", Items: []webui.NavItem{webui.NavPages[siteArgs]{Page: barred, Load: siteAddresses("a", "b"), Label: siteLabel}}},
			webui.NavGroup{Label: "Open", Items: []webui.NavItem{webui.NavPage{Page: open, Label: "Open page"}}},
		},
		Menu:  []webui.NavItem{webui.NavGroup{Label: "Menu refused", Items: []webui.NavItem{webui.NavPage{Page: shut, Label: "Shut"}}}},
		Pages: webui.Pages{open, shut, site, broken, barred},
	}.MustCompile("/admin")

	p := pageOf(t, h, "/admin/open")
	assert.DeepEqual(t, groups(t, p), []string{"Open (open)"})
	body := serve(h, http.MethodGet, "/admin/open").Body.String()
	for _, gone := range []string{"Refused", "Listing nothing", "Listing fails", "Every address refused", "Menu refused"} {
		assert.False(t, strings.Contains(body, gone))
	}
}
