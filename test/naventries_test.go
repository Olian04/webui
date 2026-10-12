package webui_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
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

func siteEntries(names ...string) func(context.Context) ([]webui.NavEntry[siteArgs], error) {
	return func(context.Context) ([]webui.NavEntry[siteArgs], error) {
		var out []webui.NavEntry[siteArgs]
		for _, n := range names {
			out = append(out, webui.NavEntry[siteArgs]{Args: siteArgs{Name: n}, Label: n})
		}
		return out, nil
	}
}

// entriesApp has a page for each site, listed in the sidebar between two fixed entries, a page
// below it that borrows its place, and the guard that says which sites a visitor may open.
func entriesApp(entries func(context.Context) ([]webui.NavEntry[siteArgs], error), guard func(context.Context, siteArgs) error) http.Handler {
	home := webui.Page[webui.NoArgs]{Path: "/home", Nav: webui.Nav{Label: "Home", Section: "Platform"}, Body: webui.Stack{}}
	site := webui.Page[siteArgs]{
		Path: "/site/{name}", Nav: webui.Nav{Icon: "location-dot", Section: "Platform"},
		NavEntries: entries, Guard: guard, Body: webui.Stack{},
	}
	log := webui.Page[siteArgs]{Path: "/site/{name}/log", Body: webui.Stack{}}
	alerts := webui.Page[webui.NoArgs]{Path: "/alerts", Nav: webui.Nav{Label: "Alerts", Section: "Platform"}, Body: webui.Stack{}}
	return webui.App{Pages: webui.Pages{home, site, log, alerts}}.MustCompile("/admin")
}

// sidebar is the entries of the sidebar, in order, as "label>href" with a star on the lit one.
func sidebar(t *testing.T, h http.Handler, target string) []string {
	t.Helper()

	body := serve(h, http.MethodGet, target).Body.String()
	nav := body[strings.Index(body, `<nav class="side-scroll"`):]
	nav = nav[:strings.Index(nav, `</nav>`)]
	var out []string
	for _, m := range regexp.MustCompile(`<a class="nav-item( active)?" href="([^"]*)" title="([^"]*)"`).FindAllStringSubmatch(nav, -1) {
		out = append(out, m[3]+">"+m[2]+map[bool]string{true: "*", false: ""}[m[1] != ""])
	}
	return out
}

func TestAPageWithNavEntriesHasAnEntryForEachOfThemInTheSidebarInOrder(t *testing.T) {
	t.Parallel()

	h := entriesApp(siteEntries(siteNames...), nil)
	got := sidebar(t, h, "/admin/home")
	assert.DeepEqual(t, got, []string{
		"Home>/admin/home*",
		// In the place of the page, among the entries of the section it names, each with the page's icon.
		"Stockholm>/admin/site/Stockholm",
		"Malmö>/admin/site/Malm%C3%B6",
		"Göteborg>/admin/site/G%C3%B6teborg",
		"Alerts>/admin/alerts",
	})
	body := serve(h, http.MethodGet, "/admin/home").Body.String()
	assert.Equal(t, strings.Count(body, `fa-location-dot`), 3)
	assert.Equal(t, strings.Count(body, `class="nav-section"`), 1) // one caption: they are one group
}

func TestTheEntryOfThePageOnScreenIsLitAndSoIsTheEntryOfThePageThatBorrowsItsPlace(t *testing.T) {
	t.Parallel()

	h := entriesApp(siteEntries(siteNames...), nil)
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

	entries := func(context.Context) ([]webui.NavEntry[queryArgs], error) {
		return []webui.NavEntry[queryArgs]{
			{Args: queryArgs{}, Label: "All devices"},
			{Args: queryArgs{Site: "Stockholm"}, Label: "Stockholm"},
			{Args: queryArgs{Site: "Stockholm", Kind: "gpu"}, Label: "Stockholm GPUs"},
		}, nil
	}
	h := webui.App{Pages: webui.Pages{webui.Page[queryArgs]{Path: "/device", Nav: webui.Nav{Section: "Devices"}, NavEntries: entries, Body: webui.Stack{}}}}.MustCompile("/admin")
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
	h := entriesApp(siteEntries(siteNames...), func(_ context.Context, a siteArgs) error {
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

	for name, entries := range map[string]func(context.Context) ([]webui.NavEntry[siteArgs], error){
		"an error": func(context.Context) ([]webui.NavEntry[siteArgs], error) {
			return nil, errors.New("db secret-host is down")
		},
		"a panic": func(context.Context) ([]webui.NavEntry[siteArgs], error) { panic("boom") },
		"slow": func(ctx context.Context) ([]webui.NavEntry[siteArgs], error) {
			<-ctx.Done() // until the deadline the library gave it
			return nil, ctx.Err()
		},
	} {
		start := time.Now()
		h := entriesApp(entries, nil)
		rec := serve(h, http.MethodGet, "/admin/home")
		assert.Equal(t, rec.Code, http.StatusOK)
		assert.DeepEqual(t, sidebar(t, h, "/admin/home"), []string{"Home>/admin/home*", "Alerts>/admin/alerts"})
		assert.False(t, strings.Contains(rec.Body.String(), "secret-host"))
		if time.Since(start) > 10*time.Second {
			t.Errorf("%s: took %s", name, time.Since(start))
		}
	}
}

func TestOnlyTheFirstFiftyEntriesAreShownAndAnEntryWithNoLabelOrNoAddressIsLeftOut(t *testing.T) {
	t.Parallel()

	many := func(context.Context) ([]webui.NavEntry[siteArgs], error) {
		out := []webui.NavEntry[siteArgs]{
			{Args: siteArgs{Name: "no-label"}},         // says nothing
			{Args: siteArgs{Name: ""}, Label: "Empty"}, // an empty path argument has no address
		}
		for i := range 80 {
			out = append(out, webui.NavEntry[siteArgs]{Args: siteArgs{Name: fmt.Sprintf("s%02d", i)}, Label: fmt.Sprintf("Site %02d", i)})
		}
		return out, nil
	}
	h := entriesApp(many, nil)
	got := sidebar(t, h, "/admin/home")
	// Of the fifty it takes, two are not entries: Home, Alerts and 48.
	assert.Equal(t, len(got), 50)
	assert.True(t, strings.Contains(strings.Join(got, "|"), "Site 47>"))
	assert.False(t, strings.Contains(strings.Join(got, "|"), "Site 48>"))
}

func TestEntriesAreOnlyLoadedForAFullPage(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	h := entriesApp(func(context.Context) ([]webui.NavEntry[siteArgs], error) {
		calls.Add(1)
		return nil, nil
	}, nil)
	serve(h, http.MethodGet, "/admin/home")
	assert.Equal(t, int(calls.Load()), 1)
	serve(h, http.MethodGet, "/admin/home")
	assert.Equal(t, int(calls.Load()), 2) // nothing is kept between pages: it is the visitor's, and it may change

	before := calls.Load()
	for _, target := range []string{"/admin/_webui/search?q=x", "/admin/_webui/" + "nothing"} {
		serve(h, http.MethodGet, target)
	}
	assert.Equal(t, calls.Load(), before) // not for an asset, or the search
}

func TestTheRootGoesToTheFirstFixedEntryAndNotToAPageThatListsSeveral(t *testing.T) {
	t.Parallel()

	site := webui.Page[siteArgs]{Path: "/site/{name}", Nav: webui.Nav{Section: "A"}, NavEntries: siteEntries("Stockholm"), Body: webui.Stack{}}
	home := webui.Page[webui.NoArgs]{Path: "/home", Nav: webui.Nav{Label: "Home", Section: "B"}, Body: webui.Stack{}}
	h := webui.App{Pages: webui.Pages{site, home}}.MustCompile("/admin")
	assert.Equal(t, serve(h, http.MethodGet, "/admin/").Header().Get("Location"), "/admin/home")
}

func TestThePageThatListsSeveralIsStillAPageWithItsOwnGuard(t *testing.T) {
	t.Parallel()

	h := entriesApp(siteEntries(siteNames...), func(_ context.Context, a siteArgs) error {
		if a.Name == "Malmö" {
			return errors.New("not for you")
		}
		return nil
	})
	assert.Equal(t, serve(h, http.MethodGet, "/admin/site/Malm%C3%B6").Code, http.StatusForbidden) // however the entry was left out
	assert.Equal(t, serve(h, http.MethodGet, "/admin/site/Stockholm").Code, http.StatusOK)
}

func TestNavEntriesAreCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(p webui.Page[siteArgs]) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{p}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	// Entries say their own words: a label on the page is a mistake.
	assert.Contains(t, problems(webui.Page[siteArgs]{Path: "/site/{name}", Nav: webui.Nav{Label: "Site"}, NavEntries: siteEntries("x"), Body: webui.Stack{}}),
		`NavEntries and the navigation label "Site"`)
	// And without NavEntries, a page with arguments still has no entry, with the way out said.
	assert.Contains(t, problems(webui.Page[siteArgs]{Path: "/site/{name}", Nav: webui.Nav{Label: "Site"}, Body: webui.Stack{}}), "give the page NavEntries")
	// Icon and Section are the group's.
	_, err := webui.App{Pages: webui.Pages{webui.Page[siteArgs]{Path: "/site/{name}", Nav: webui.Nav{Icon: "location-dot", Section: "Platform"}, NavEntries: siteEntries("x"), Body: webui.Stack{}}}}.Compile("/admin")
	assert.NoError(t, err)
}
