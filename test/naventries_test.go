package webui_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type siteArgs struct{ Name string }

type queryArgs struct {
	Site string
	Kind string
}

// siteNames are the sites the entries are made of.
var siteNames = []string{"Stockholm", "Malmö", "Göteborg"}

// siteAddresses loads the addresses of the site page that these names make, and siteLabel says what each is called: its name.
func siteAddresses(names ...string) func(context.Context) ([]siteArgs, webui.Loaded) {
	return func(context.Context) ([]siteArgs, webui.Loaded) {
		var out []siteArgs
		for _, n := range names {
			out = append(out, siteArgs{Name: n})
		}
		return out, webui.Ready()
	}
}

func siteLabel(a siteArgs) string { return a.Name }

// queryLabel names the addresses of a page whose arguments are in the query: none is every device, a site is the site, and a kind of gpu is its GPUs.
func queryLabel(a queryArgs) string {
	switch {
	case a.Site == "":
		return "All devices"
	case a.Kind == "gpu":
		return a.Site + " GPUs"
	}
	return a.Site
}

// entriesApp has a page for each site, listed in the sidebar between two fixed entries, a page
// below it that borrows its place, and the guard that says which sites a visitor may open.
func entriesApp(load func(context.Context) ([]siteArgs, webui.Loaded), guard func(context.Context, siteArgs) error) http.Handler {
	return labelledEntriesApp(load, siteLabel, guard)
}

func labelledEntriesApp(load func(context.Context) ([]siteArgs, webui.Loaded), label func(siteArgs) string, guard func(context.Context, siteArgs) error) http.Handler {
	home := webui.Page[webui.NoArgs]{Path: "/home", View: webui.Stack{}}
	site := webui.Page[siteArgs]{Path: "/site/{name}", Guard: guard, View: webui.Stack{}}
	log := webui.Page[siteArgs]{Path: "/site/{name}/log", View: webui.Stack{}}
	alerts := webui.Page[webui.NoArgs]{Path: "/alerts", View: webui.Stack{}}
	return webui.App{
		Navigation: []webui.NavItem{webui.NavGroup{Label: "Platform", Items: []webui.NavItem{
			webui.NavPage{Page: home, Label: "Home"},
			webui.NavPages[siteArgs]{Page: site, Load: load, Label: label, Icon: "location-dot"},
			webui.NavPage{Page: alerts, Label: "Alerts"},
		}}},
		Pages: webui.Pages{home, site, log, alerts},
	}.MustCompile("/admin")
}

// sidebar is the entries of the sidebar, in order, as "label>href" with a star on the lit one.
func sidebar(t *testing.T, h http.Handler, target string) []string {
	t.Helper()

	var out []string
	for _, l := range sidebarNav(t, pageOf(t, h, target)).Links() {
		out = append(out, l.Name+">"+l.Href+map[bool]string{true: "*", false: ""}[l.Current])
	}
	return out
}

func TestAPageWithNavEntriesHasAnEntryForEachOfThemInTheSidebarInOrder(t *testing.T) {
	t.Parallel()

	h := entriesApp(siteAddresses(siteNames...), nil)
	got := sidebar(t, h, "/admin/home")
	assert.DeepEqual(t, got, []string{
		"Home>/admin/home*",
		// In the place of the page, among the entries of the section it names, each with the page's icon.
		"Stockholm>/admin/site/Stockholm",
		"Malmö>/admin/site/Malm%C3%B6",
		"Göteborg>/admin/site/G%C3%B6teborg",
		"Alerts>/admin/alerts",
	})
	p := pageOf(t, h, "/admin/home")
	icons := 0
	for _, e := range sidebarNav(t, p).ByRole("link", "") {
		if strings.Contains(iconOf(e), "fa-location-dot") {
			icons++
		}
	}
	assert.Equal(t, icons, 3)
	captions := 0
	for _, o := range outline(t, p) {
		if strings.HasPrefix(o, "# ") {
			captions++
		}
	}
	assert.Equal(t, captions, 1) // one caption: they are one group
}

func TestTheEntryOfThePageOnScreenIsLitAndSoIsTheEntryOfThePageThatBorrowsItsPlace(t *testing.T) {
	t.Parallel()

	h := entriesApp(siteAddresses(siteNames...), nil)
	lit := func(target string) []string {
		var out []string
		for _, e := range sidebar(t, h, target) {
			if strings.HasSuffix(e, "*") {
				out = append(out, e)
			}
		}
		return out
	}
	assert.DeepEqual(t, lit("/admin/site/Stockholm"), []string{"Stockholm>/admin/site/Stockholm*"})
	assert.DeepEqual(t, lit("/admin/site/Malm%C3%B6"), []string{"Malmö>/admin/site/Malm%C3%B6*"}) // the one for that site, and no other
	// The page below it borrows the place, and lights the entry of the same site.
	assert.DeepEqual(t, lit("/admin/site/G%C3%B6teborg/log"), []string{"Göteborg>/admin/site/G%C3%B6teborg*"})
	// A site that has no entry lights none of them; and a page elsewhere lights its own.
	assert.Equal(t, len(lit("/admin/site/Unknown")), 0)
	assert.DeepEqual(t, lit("/admin/alerts"), []string{"Alerts>/admin/alerts*"})
	// What is in the address besides the arguments, the state of a table, does not change which.
	assert.DeepEqual(t, lit("/admin/site/Stockholm?devices.offset=25&webui.from=%2Fadmin%2Fhome"), []string{"Stockholm>/admin/site/Stockholm*"})
}

func TestEntriesOfAPageWithQueryArgumentsLightTheOneThatFitsTheAddressBest(t *testing.T) {
	t.Parallel()

	load := func(context.Context) ([]queryArgs, webui.Loaded) {
		return []queryArgs{{}, {Site: "Stockholm"}, {Site: "Stockholm", Kind: "gpu"}}, webui.Ready()
	}
	devices := webui.Page[queryArgs]{Path: "/device", View: webui.Stack{}}
	h := webui.App{
		Navigation: []webui.NavItem{webui.NavGroup{Label: "Devices", Items: []webui.NavItem{webui.NavPages[queryArgs]{Page: devices, Load: load, Label: queryLabel}}}},
		Pages:      webui.Pages{devices},
	}.MustCompile("/admin")
	lit := func(target string) string {
		var out []string
		for _, e := range sidebar(t, h, target) {
			if strings.HasSuffix(e, "*") {
				out = append(out, strings.SplitN(e, ">", 2)[0])
			}
		}
		return strings.Join(out, ",")
	}
	assert.Equal(t, lit("/admin/device"), "All devices")
	assert.Equal(t, lit("/admin/device?site=Stockholm"), "Stockholm")
	assert.Equal(t, lit("/admin/device?site=Stockholm&kind=gpu"), "Stockholm GPUs")
	assert.Equal(t, lit("/admin/device?site=Stockholm&kind=other"), "Stockholm") // the best that fits
	assert.Equal(t, lit("/admin/device?site=Malm%C3%B6"), "All devices")
	assert.Equal(t, lit("/admin/device?site=Stockholm&devices.sort=id"), "Stockholm")
}

func TestAnEntryTheVisitorMayNotOpenIsLeftOutAndTheGuardIsAskedWithItsOwnArguments(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	h := entriesApp(siteAddresses(siteNames...), func(_ context.Context, a siteArgs) error {
		seen[a.Name] = true
		if a.Name == "Malmö" {
			return errors.New("not for you")
		}
		return nil
	})
	got := sidebar(t, h, "/admin/home")
	assert.Equal(t, len(got), 4) // Home, Stockholm, Göteborg, Alerts
	assert.False(t, strings.Contains(strings.Join(got, "|"), "Malm"))
	assert.True(t, seen["Stockholm"] && seen["Malmö"] && seen["Göteborg"])
	assert.False(t, seen[""]) // never with no arguments, which a Guard that looks the thing up would refuse
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/home").Body.String(), "not for you"))
}

func TestEntriesThatFailPanicOrAreSlowLeaveThePageDrawnWithoutThem(t *testing.T) {
	t.Parallel()

	for name, load := range map[string]func(context.Context) ([]siteArgs, webui.Loaded){
		"a failure": func(context.Context) ([]siteArgs, webui.Loaded) {
			return []siteArgs{{Name: "Stockholm"}}, webui.Failed(errors.New("db secret-host is down")) // what came with a failure is not listed
		},
		"a panic": func(context.Context) ([]siteArgs, webui.Loaded) { panic("boom") },
		"slow": func(ctx context.Context) ([]siteArgs, webui.Loaded) {
			<-ctx.Done() // until the deadline the library gave it
			return nil, webui.Failed(ctx.Err())
		},
	} {
		start := time.Now()
		h := entriesApp(load, nil)
		rec := serve(h, http.MethodGet, "/admin/home")
		assert.Equal(t, rec.Code, http.StatusOK)
		assert.DeepEqual(t, sidebar(t, h, "/admin/home"), []string{"Home>/admin/home*", "Alerts>/admin/alerts"})
		assert.False(t, strings.Contains(rec.Body.String(), "secret-host"))
		if time.Since(start) > 10*time.Second {
			t.Errorf("%s: took %s", name, time.Since(start))
		}
	}
}

func TestALabelThatPanicsOrALoadThatAsksToBeRefreshedDoNotTakeThePageDown(t *testing.T) {
	t.Parallel()

	// A Label that panics leaves the page's entries out, as a Load that panics does.
	h := labelledEntriesApp(siteAddresses(siteNames...), func(siteArgs) string { panic("boom") }, nil)
	assert.Equal(t, serve(h, http.MethodGet, "/admin/home").Code, http.StatusOK)
	assert.DeepEqual(t, sidebar(t, h, "/admin/home"), []string{"Home>/admin/home*", "Alerts>/admin/alerts"})

	// A RefreshAfter means nothing in a sidebar: the entries are listed and the page is as any other.
	h = entriesApp(func(ctx context.Context) ([]siteArgs, webui.Loaded) {
		addresses, loaded := siteAddresses(siteNames...)(ctx)
		return addresses, loaded.RefreshAfter(time.Second)
	}, nil)
	assert.Equal(t, len(sidebar(t, h, "/admin/home")), 5)
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/home").Body.String(), "data-refresh-after"))
}

func TestOnlyTheFirstFiftyEntriesAreShownAndAnEntryWithNoLabelOrNoAddressIsLeftOut(t *testing.T) {
	t.Parallel()

	many := func(context.Context) ([]siteArgs, webui.Loaded) {
		out := []siteArgs{{Name: "no-label"}, {Name: ""}} // the first is given no words, and an empty path argument has no address
		for i := range 80 {
			out = append(out, siteArgs{Name: fmt.Sprintf("s%02d", i)})
		}
		return out, webui.Ready()
	}
	label := func(a siteArgs) string {
		switch a.Name {
		case "no-label":
			return ""
		case "":
			return "Empty"
		}
		return "Site " + strings.TrimPrefix(a.Name, "s")
	}
	h := labelledEntriesApp(many, label, nil)
	got := sidebar(t, h, "/admin/home")
	// Of the fifty it takes, two are not entries: Home, Alerts and 48.
	assert.Equal(t, len(got), 50)
	assert.True(t, strings.Contains(strings.Join(got, "|"), "Site 47>"))
	assert.False(t, strings.Contains(strings.Join(got, "|"), "Site 48>"))
}

func TestEntriesAreOnlyLoadedForAFullPage(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	h := entriesApp(func(context.Context) ([]siteArgs, webui.Loaded) {
		calls.Add(1)
		return nil, webui.Ready()
	}, nil)
	serve(h, http.MethodGet, "/admin/home")
	assert.Equal(t, int(calls.Load()), 1)
	serve(h, http.MethodGet, "/admin/home")
	assert.Equal(t, int(calls.Load()), 2) // nothing is kept between pages: it is the visitor's, and it may change

	before := calls.Load()
	for _, target := range []string{"/admin/_webui/search?q=x", "/admin/_webui/" + "nothing"} {
		serve(searchBox(h), http.MethodGet, target)
	}
	assert.Equal(t, calls.Load(), before) // not for an asset, or the list that the box shows

	// The page of results is a page, in the shell, and has the sidebar.
	serve(h, http.MethodGet, "/admin/_webui/search?q=x")
	assert.Equal(t, calls.Load(), before+1)
}

func TestTheRootGoesToTheFirstFixedEntryAndNotToAPageThatListsSeveral(t *testing.T) {
	t.Parallel()

	site := webui.Page[siteArgs]{Path: "/site/{name}", View: webui.Stack{}}
	home := webui.Page[webui.NoArgs]{Path: "/home", View: webui.Stack{}}
	h := webui.App{
		Navigation: []webui.NavItem{
			webui.NavGroup{Label: "A", Items: []webui.NavItem{webui.NavPages[siteArgs]{Page: site, Load: siteAddresses("Stockholm"), Label: siteLabel}}},
			webui.NavGroup{Label: "B", Items: []webui.NavItem{webui.NavPage{Page: home, Label: "Home"}}},
		},
		Pages: webui.Pages{site, home},
	}.MustCompile("/admin")
	assert.Equal(t, serve(h, http.MethodGet, "/admin/").Header().Get("Location"), "/admin/home")
}

func TestThePageThatListsSeveralIsStillAPageWithItsOwnGuard(t *testing.T) {
	t.Parallel()

	h := entriesApp(siteAddresses(siteNames...), func(_ context.Context, a siteArgs) error {
		if a.Name == "Malmö" {
			return errors.New("not for you")
		}
		return nil
	})
	assert.Equal(t, serve(h, http.MethodGet, "/admin/site/Malm%C3%B6").Code, http.StatusForbidden) // however the entry was left out
	assert.Equal(t, serve(h, http.MethodGet, "/admin/site/Stockholm").Code, http.StatusOK)
}

func TestAPageThatListsSeveralIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	site := webui.Page[siteArgs]{Path: "/site/{name}", View: webui.Stack{}}
	problems := func(items ...webui.NavItem) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Navigation: items, Pages: webui.Pages{site}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	// A page with arguments has no single address, so it is not an entry of its own: the way out is said.
	refuses(t, problems(webui.NavPage{Page: site, Label: "Site"}), "App.Navigation[0]", "/site/{name}", "NavPages")
	// The entries are the page's to list, and the list has to be there.
	refuses(t, problems(webui.NavPages[siteArgs]{Page: site, Label: siteLabel}), "App.Navigation[0]", "Load")
	refuses(t, problems(webui.NavPages[siteArgs]{Page: site, Load: siteAddresses("x")}), "App.Navigation[0]", "Label")
	// And an icon is the group's, and the entries' own.
	_, err := webui.App{Navigation: []webui.NavItem{webui.NavGroup{Label: "Platform", Icon: "server", Items: []webui.NavItem{webui.NavPages[siteArgs]{Page: site, Load: siteAddresses("x"), Label: siteLabel, Icon: "location-dot"}}}}, Pages: webui.Pages{site}}.Compile("/admin")
	assert.NoError(t, err)
}

func TestAPageThatBorrowsTheNavigationPlaceLightsOnlyTheEntryItsAddressFits(t *testing.T) {
	t.Parallel()

	load := func(context.Context) ([]queryArgs, webui.Loaded) {
		return []queryArgs{{}, {Site: "Stockholm"}, {Site: "Malmö"}}, webui.Ready()
	}
	list := webui.Page[queryArgs]{Path: "/device", View: webui.Stack{}}
	log := webui.Page[queryArgs]{Path: "/device/log", View: webui.Stack{}} // no entry of its own: it borrows the place
	h := webui.App{
		Navigation: []webui.NavItem{webui.NavGroup{Label: "Devices", Items: []webui.NavItem{webui.NavPages[queryArgs]{Page: list, Load: load, Label: queryLabel}}}},
		Pages:      webui.Pages{list, log},
	}.MustCompile("/admin")
	lit := func(target string) string {
		var out []string
		for _, e := range sidebar(t, h, target) {
			if strings.HasSuffix(e, "*") {
				out = append(out, strings.SplitN(e, ">", 2)[0])
			}
		}
		return strings.Join(out, ",")
	}
	// Not every entry: the page carries no site, so only the entry that needs none fits.
	assert.Equal(t, lit("/admin/device/log"), "All devices")
	// With a site in its own address, the entry of that site.
	assert.Equal(t, lit("/admin/device/log?site=Stockholm"), "Stockholm")
	assert.Equal(t, lit("/admin/device/log?site=Malm%C3%B6"), "Malmö")
	assert.Equal(t, lit("/admin/device/log?site=Elsewhere"), "All devices")
}

func TestTheSearchResultsOfAPageThatListsEntriesAreGroupedUnderTheNearestNamedPageAboveIt(t *testing.T) {
	t.Parallel()

	sites := webui.Page[webui.NoArgs]{Path: "/site", View: webui.Stack{}}
	site := webui.Page[siteArgs]{
		Path: "/site/{name}", View: webui.Stack{},
		Search: func(context.Context, string, int) ([]webui.Hit[siteArgs], error) {
			return []webui.Hit[siteArgs]{{Args: siteArgs{Name: "Stockholm"}, Title: "Stockholm"}}, nil
		},
	}
	h := webui.App{
		Navigation: []webui.NavItem{
			webui.NavPage{Page: sites, Label: "Sites"},
			webui.NavGroup{Label: "Sites", Items: []webui.NavItem{webui.NavPages[siteArgs]{Page: site, Load: siteAddresses("Stockholm"), Label: siteLabel}}},
		},
		Pages: webui.Pages{sites, site},
	}.MustCompile("/admin")
	assert.Equal(t, search(t, h, "stock").Results[0].Group, "Sites") // not the path "/site/{name}"
}
