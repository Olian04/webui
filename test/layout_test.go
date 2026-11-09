package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

type layoutArgs struct {
	Id string
	Q  string
}

// layoutApp is the README's nested body: a Split of a form and a table, then
// Tabs. It counts loads so a test can prove which leaves were read.
func layoutApp(guard func(context.Context, layoutArgs) error) (http.Handler, *[4]atomic.Int32) {
	var loads [4]atomic.Int32
	table := func(i int, title string) webui.Table[Device] {
		return webui.Table[Device]{
			Title: title,
			Loader: webui.PagedLoader[Device]{Load: func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
				loads[i].Add(1)
				return webui.PagedWindow[Device]{Items: []Device{{Id: title + "-row"}}, Total: 1}, webui.Ready()
			}},
			Columns: []webui.Accessor[Device]{formID},
		}
	}
	page := webui.Page[layoutArgs]{
		Path:  "/device/{id}",
		Guard: guard,
		View: webui.Stack{
			webui.Split{
				webui.Form[Device]{
					Title:  "Form",
					Load:   func(context.Context) (Device, webui.Loaded) { loads[0].Add(1); return Device{Id: "x"}, webui.Ready() },
					Fields: []webui.Accessor[Device]{formID},
				},
				table(1, "Events"),
			},
			webui.Tabs{
				{Label: "Overview", View: table(2, "Overview")},
				{Label: "Raw", View: table(3, "Raw")},
			},
		},
	}
	return webui.App{Pages: webui.Pages{page}}.MustCompile("/admin"), &loads
}

func TestTabsLoadOnlyTheSelectedPanel(t *testing.T) {
	t.Parallel()

	h, loads := layoutApp(nil)

	strip := func(p *page.Page) []page.Link {
		return page.One(t, "the tabs", p.ByRole("navigation", "Views: Overview, Raw")).Links()
	}
	shows := func(p *page.Page, text string) bool { return len(p.ByText(text)) > 0 }

	p := pageOf(t, h, "/admin/device/d1")
	assert.True(t, shows(p, "Overview-row"))
	assert.False(t, shows(p, "Raw-row"))
	assert.Equal(t, loads[2].Load(), int32(1))
	assert.Equal(t, loads[3].Load(), int32(0)) // the hidden tab was never read

	// The strip is links; the first needs no argument, the others carry theirs.
	assert.DeepEqual(t, strip(p), []page.Link{{Name: "Overview", Href: "/admin/device/d1", Current: true}, {Name: "Raw", Href: "/admin/device/d1?tabs.tab=raw"}})

	raw := pageOf(t, h, "/admin/device/d1?tabs.tab=raw")
	assert.True(t, shows(raw, "Raw-row"))
	assert.False(t, shows(raw, "Overview-row"))
	assert.Equal(t, loads[3].Load(), int32(1))
	assert.DeepEqual(t, strip(raw), []page.Link{{Name: "Overview", Href: "/admin/device/d1"}, {Name: "Raw", Href: "/admin/device/d1?tabs.tab=raw", Current: true}})

	// An unknown tab lands on the first rather than on nothing.
	assert.True(t, shows(pageOf(t, h, "/admin/device/d1?tabs.tab=Gone"), "Overview-row"))
}

func TestLayoutsGivePanelsPositionalIDs(t *testing.T) {
	t.Parallel()

	h, _ := layoutApp(nil)
	var ids []string
	for _, v := range pageOf(t, h, "/admin/device/d1").Views() {
		ids = append(ids, v.Attr("data-view"))
	}
	assert.DeepEqual(t, ids, []string{"p.0.0", "p.0.1", "p.1.0"})
}

func TestPanelRequestReturnsOnlyThatPanel(t *testing.T) {
	t.Parallel()

	h, loads := layoutApp(nil)
	req := httptest.NewRequest(http.MethodGet, "/admin/device/d1", nil)
	req.Header.Set("X-Webui-View", "p.0.1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, rec.Code, http.StatusOK)
	fragment := page.Parse(t, rec.Body.String())
	assert.True(t, page.IsFragment(rec.Body.String()))
	assert.Equal(t, fragment.Views()[0].Attr("data-view"), "p.0.1")
	assert.Equal(t, len(fragment.Views()), 1)
	assert.Equal(t, len(fragment.ByText("Events-row")), 1)
	assert.Contains(t, rec.Header().Get("Vary"), "X-Webui-View")
	assert.Equal(t, rec.Header().Get("Cache-Control"), "no-store")
	// Only that panel was read; the rest of the screen holds still.
	assert.Equal(t, loads[0].Load(), int32(0))
	assert.Equal(t, loads[1].Load(), int32(1))
	assert.Equal(t, loads[2].Load(), int32(0))

	// A full page says it varies on the header too, so no cache serves one for the other.
	assert.Contains(t, serve(h, http.MethodGet, "/admin/device/d1").Header().Get("Vary"), "X-Webui-View")
}

func TestPanelRequestStillRunsThePageGuardAndRejectsUnknownIDs(t *testing.T) {
	t.Parallel()

	var denied bool
	h, loads := layoutApp(func(context.Context, layoutArgs) error {
		if denied {
			return errors.New("not for you")
		}
		return nil
	})

	ask := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/admin/device/d1", nil)
		req.Header.Set("X-Webui-View", id)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	assert.Equal(t, ask("p.9").Code, http.StatusNotFound)
	assert.Equal(t, ask("p.0").Code, http.StatusNotFound) // a layout is not a panel
	assert.Equal(t, ask("../../etc").Code, http.StatusNotFound)

	denied = true
	rec := ask("p.0.1") // the panel URL is the page URL: the same Guard runs first
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.True(t, strings.Contains(page.Parse(t, rec.Body.String()).Root().Text(), "not for you"))
	assert.Equal(t, loads[1].Load(), int32(0))
}

func serveView(h http.Handler, target, panel string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("X-Webui-View", panel)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// A tab is followed, with script, by asking for the Tabs alone: its strip and the tab that is selected. Nothing else of the
// page is read, which is the point: a tab is not a new page, and the panels around it have nothing new to say.
func TestATabsIsAskedForAloneAndOnlyTheSelectedTabIsRead(t *testing.T) {
	t.Parallel()

	h, loads := layoutApp(nil)
	rec := serveView(h, "/admin/device/d1?tabs.tab=raw", "p.1")
	body := rec.Body.String()

	assert.Equal(t, rec.Code, http.StatusOK)
	alone := page.Parse(t, body)
	assert.True(t, page.IsFragment(body)) // not the page
	assert.Equal(t, len(alone.ByAttr("data-tabs", "p.1")), 1)
	assert.Equal(t, len(alone.ByText("Raw-row")), 1)
	strip := page.One(t, "the strip", alone.ByRoleContaining("navigation", "Views:"))
	assert.True(t, slices.Contains(strip.Links(), page.Link{Name: "Raw", Href: "/admin/device/d1?tabs.tab=raw", Current: true}))
	assert.Equal(t, len(alone.ByText("Overview-row")), 0)
	assert.Equal(t, len(alone.ByRole("complementary", "")), 0)
	for i, want := range []int32{0, 0, 0, 1} { // the form, the events, and the other tab were not read
		assert.Equal(t, loads[i].Load(), want)
	}

	// The page carries what script needs to ask: the id, and the argument that selects a tab.
	assert.Equal(t, len(pageOf(t, h, "/admin/device/d1").ByAttr("data-tabs", "p.1")), 1)

	// It is the page's own request: the Guard runs, and an id that is not a Tabs or a panel is a 404.
	refused, _ := layoutApp(func(context.Context, layoutArgs) error { return errors.New("no") })
	assert.Equal(t, serveView(refused, "/admin/device/d1", "p.1").Code, http.StatusForbidden)
	assert.Equal(t, serveView(h, "/admin/device/d1", "p.9").Code, http.StatusNotFound)
}

// A Tabs has as many tabs as it is given, the selected one is marked as where the reader is, and only the selected one is read.
func TestAStripOfManyTabsHasAllOfThemAndLoadsOnlyTheSelectedOne(t *testing.T) {
	t.Parallel()

	panels := make([]webui.Tab, 20)
	for i := range panels {
		panels[i] = webui.Tab{Label: "A tab with a label number " + strconv.Itoa(i), View: quickTable("Panel " + strconv.Itoa(i))}
	}
	h := streamApp(webui.Tabs(panels))
	p := pageOf(t, h, "/admin/p?tabs.tab=a-tab-with-a-label-number-19")
	strip := page.One(t, "the strip", p.ByRoleContaining("navigation", "Views:"))
	assert.Equal(t, len(strip.Links()), 20)
	var current []page.Link
	for _, l := range strip.Links() {
		if l.Current {
			current = append(current, l)
		}
	}
	assert.DeepEqual(t, current, []page.Link{{Name: "A tab with a label number 19", Href: "/admin/p?tabs.tab=a-tab-with-a-label-number-19", Current: true}})
	assert.Equal(t, len(p.ByText("Panel 19-row")), 1)
	assert.Equal(t, len(p.ByText("Panel 0-row")), 0)
}

// When the HTML was built is in the page for a reader with no script, and the badge for a stale page is there for script to show.
func TestThePageSaysWhenItWasBuiltAndHasTheBadgeForScriptToShow(t *testing.T) {
	t.Parallel()

	h := streamApp(webui.Stack{quickTable("First")})
	p := pageOf(t, h, "/admin/p")
	assert.Equal(t, len(p.ByAttr("data-built", "")), 1) // the server says it, for the page without script
	assert.True(t, page.One(t, "when", p.ByAttr("data-built", "")).Has("datetime"))
	assert.True(t, page.One(t, "the badge", p.ByAttr("data-stale", "")).Hidden()) // and the badge is there for script to show
}

// The render time and the delivery time are not two measures of one thing: the page says the render time, which the server knows, and has a place for
// script to say the delivery.
func TestTheStatusBarSaysTheRenderTimeAndHasAPlaceForTheDeliveryTime(t *testing.T) {
	t.Parallel()

	h := streamApp(webui.Stack{quickTable("First")})
	p := pageOf(t, h, "/admin/p")
	render := page.One(t, "the render time", p.ByAttr("data-build", ""))
	assert.True(t, regexp.MustCompile(`^\d+(\.\d)? ms$`).MatchString(render.Text()))
	assert.True(t, render.Has("title")) // which says what it is the time of
	loaded := page.One(t, "the delivery time", p.ByAttr("data-loaded", ""))
	assert.True(t, loaded.Hidden()) // for script to fill in
	assert.Equal(t, loaded.Text(), "")
}

// Two strips of tabs on one page (side by side, or one in the tab of another) are two navigations, and a reader who lists the landmarks tells them
// apart by their names, which are "Views" and what the first tabs are called.
func TestEachStripOfTabsHasItsOwnName(t *testing.T) {
	t.Parallel()

	h := streamApp(webui.Split{
		webui.Tabs{{Label: "One", View: quickTable("A")}, {Label: "Two", View: quickTable("B")}},
		webui.Tabs{{Label: "Red", View: quickTable("C")}, {Label: "Green", View: quickTable("D")}, {Label: "Blue", View: quickTable("E")}, {Label: "Grey", View: quickTable("F")}},
	})
	p := pageOf(t, h, "/admin/p")
	assert.Equal(t, len(p.ByRole("navigation", "Views: One, Two")), 1)
	assert.Equal(t, len(p.ByRole("navigation", "Views: Red, Green, Blue, …")), 1)
}
