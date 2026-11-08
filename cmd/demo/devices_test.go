package main

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func TestAQueryArgumentNarrowsTheRawEventsAndNotTheRecentOnes(t *testing.T) {
	h := handler(t)
	rows := func(target, title string) int {
		return len(page.One(t, title, pageAt(t, h, target).ByRole("table", title)).Rows())
	}
	// The recent events are the last 15 minutes, whatever the page is narrowed to.
	assert.Equal(t, rows("/admin/device/dev_27c38b", "Recent events"), 3)
	assert.Equal(t, rows("/admin/device/dev_27c38b?minutes=60", "Recent events"), 3)
	// The raw ones are as far back as the argument says, and all of them without it.
	assert.Equal(t, rows("/admin/device/dev_27c38b?tabs.tab=raw-events", "Raw events"), 8)
	assert.Equal(t, rows("/admin/device/dev_27c38b?minutes=15&tabs.tab=raw-events", "Raw events"), 3)
	assert.Equal(t, rows("/admin/device/dev_27c38b?minutes=30&tabs.tab=raw-events", "Raw events"), 5)
}

func TestSaveRejectsADuplicateIPThenStaysWhenOpenedDirectly(t *testing.T) {
	h := handler(t)
	// Fields: Group{ID, IP} f0_0 f0_1, Group{Status, Site} f1_*, Rate f2.
	taken, _ := service.Device("dev_27c75c") // another device's address
	rec := post(h, "/admin/device/dev_27c38b", url.Values{"_view": {"p.0.0"}, "f1_0": {"healthy"}, "f0_1": {taken.IP}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity) // the form, drawn again with what was typed, as the answer
	refused := page.Parse(t, rec.Body.String())
	assert.True(t, shown(refused, "already in use by another device"))
	typed, _ := refused.Field("f0_1")
	assert.Equal(t, typed.Value, taken.IP) // what was typed

	rec = post(h, "/admin/device/dev_27c38b", url.Values{"_view": {"p.0.0"}, "f1_0": {"healthy"}, "f0_1": {"10.9.9.9"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/dev_27c38b") // opened directly: nowhere to return to
	device, _ := service.Device("dev_27c38b")
	assert.Equal(t, device.IP, "10.9.9.9")
}

// The sites to choose from are read when the page is asked for, from where the program keeps them: with how many
// devices each has now, and a device that is moved changes them.
func TestADevicesSiteIsChosenFromTheSitesThereAreNow(t *testing.T) {
	h := handler(t)
	device, _ := service.Device("dev_27c38b")
	var other string
	counts := map[string]int{}
	for _, s := range service.Sites() {
		counts[s.Name] = s.Devices
		if s.Name != device.Site {
			other = s.Name
		}
	}
	offered := func() []string {
		site, _ := pageAt(t, h, "/admin/device/dev_27c38b").Field("f1_1")
		var texts []string
		for _, o := range site.Options {
			texts = append(texts, o.Text)
		}
		return texts
	}
	for name, n := range counts {
		assert.True(t, slices.Contains(offered(), fmt.Sprintf("%s (%d devices)", name, n)))
	}

	rec := post(h, "/admin/device/dev_27c38b", url.Values{"_view": {"p.0.0"}, "f0_1": {device.IP}, "f1_0": {device.Status}, "f1_1": {other}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	moved, _ := service.Device("dev_27c38b")
	assert.Equal(t, moved.Site, other)
	assert.True(t, slices.Contains(offered(), fmt.Sprintf("%s (%d devices)", other, counts[other]+1)))
	assert.True(t, slices.Contains(offered(), fmt.Sprintf("%s (%d devices)", device.Site, counts[device.Site]-1)))

	// A site that is not one of them is refused.
	rec = post(h, "/admin/device/dev_27c38b", url.Values{"_view": {"p.0.0"}, "f0_1": {device.IP}, "f1_0": {device.Status}, "f1_1": {"Atlantis"}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.True(t, shown(page.Parse(t, rec.Body.String()), "Choose from the options offered."))
}

// A column that has no use for a sort leaves it out: the devices' tags are a list of words, and an event's detail is read.
func TestTheDemoLeavesSortingOutOfTheColumnsThatHaveNoUseForIt(t *testing.T) {
	h := handler(t)
	devices := pageAt(t, h, "/admin/device")
	assert.Equal(t, len(linksTo(devices, "/admin/device?devices.sort=ip")), 1)
	assert.Equal(t, len(linksTo(devices, "/admin/device?devices.sort=tags")), 0)
	assert.Equal(t, len(devices.ByRole("button", "Filter Tags")), 1) // it still filters

	events := pageAt(t, h, "/admin/device/dev_27c38b")
	assert.Equal(t, len(linksTo(events, "/admin/device/dev_27c38b?recent-events.sort=kind")), 1)
	assert.Equal(t, len(linksTo(events, "/admin/device/dev_27c38b?recent-events.sort=detail")), 0)
}

// The device list has a toolbar: a link to the page that makes a device, and a button that does something. An editor may
// press it, and a viewer sees it disabled with the reason.
func TestTheDeviceListHasAToolbarOfALinkAndAnAction(t *testing.T) {
	editor := pageAt(t, handler(t), "/admin/device")
	// The page it leads to has a form, so it is told where the visitor came from, and Cancel and a save return there.
	assert.True(t, hasLink(editor, "Create device", "/admin/device/new?webui.from=%2Fadmin%2Fdevice"))
	sync := page.One(t, "the button that does Sync now", editor.ByAttr("name", "_act")) // it asks first, and the dialog holds the button that posts
	assert.Equal(t, sync.Name(), "Sync now")
	assert.Equal(t, sync.Attr("value"), "1")

	rec := post(handler(t), "/admin/device", url.Values{"_view": {"p.0"}, "_act": {"1"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), fmt.Sprintf("oSynced %d devices", len(service.devices)))

	// A viewer is offered neither: the button's Guard refuses them, and so does the page that the Link leads to. With nothing in
	// it for them, the toolbar is not there at all. A request that is made by hand is refused all the same.
	viewer := pageAt(t, handlerAs(t, Viewer), "/admin/device")
	assert.False(t, strings.Contains(viewer.Root().AllText(), "Sync now"))
	assert.False(t, strings.Contains(viewer.Root().AllText(), "Create device"))
	assert.Equal(t, len(viewer.ByAttr("name", "_act")), 0) // and no toolbar
	rec = post(handlerAs(t, Viewer), "/admin/device", url.Values{"_view": {"p.0"}, "_act": {"1"}})
	assert.Equal(t, flashOf(rec), "erequires the editor role")
}

// The page that makes a device is a form over a device that is new, and saving it opens the device it made.
func TestTheDeviceThatIsCreatedHasAnAddressOfItsOwn(t *testing.T) {
	h := handler(t)
	before := len(service.devices)
	t.Cleanup(func() {
		service.mu.Lock()
		service.devices = service.devices[:before]
		service.mu.Unlock()
	})

	form := pageAt(t, h, "/admin/device/new")
	assert.True(t, shown(form, "New device"))
	assert.Equal(t, len(form.ByRole("button", "Create")), 1)

	taken, _ := service.Device("dev_27c75c")
	rec := post(h, "/admin/device/new", url.Values{"_view": {"p"}, "f0": {taken.IP}, "f1_0": {"healthy"}, "f1_1": {"Malmö"}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.True(t, shown(page.Parse(t, rec.Body.String()), "already in use by another device"))

	rec = post(h, "/admin/device/new", url.Values{"_view": {"p"}, "f0": {"10.7.7.7"}, "f1_0": {"quiet"}, "f1_1": {"Malmö"}, "f2": {"edge", "gpu"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, len(service.devices), before+1)
	created := service.devices[before]
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/"+created.ID)
	assert.Equal(t, created.IP, "10.7.7.7")
	assert.Equal(t, created.Site, "Malmö")
	assert.DeepEqual(t, created.Tags, []string{"edge", "gpu"})
	assert.Equal(t, get(h, rec.Header().Get("Location")).Code, http.StatusOK)
}

// The Raw events tab is the events and nothing above them: its window is the page's Minutes argument, which only a link sets.
func TestTheRawEventsTabHasNoToolbar(t *testing.T) {
	h := handler(t)
	raw := pageAt(t, h, "/admin/device/dev_27c38b?minutes=30&tabs.tab=raw-events")
	assert.Equal(t, len(raw.ByAttr("name", "_act")), 0)
	assert.False(t, strings.Contains(raw.Root().AllText(), "Events from"))
}

// A device has notes, in a text area: its lines are kept, and a line break is one character whatever the browser sent.
func TestADevicesNotesAreATextAreaThatKeepsItsLines(t *testing.T) {
	h := handler(t)
	t.Cleanup(func() {
		service.mu.Lock()
		for i := range service.devices {
			if service.devices[i].ID == "dev_27c38b" {
				service.devices[i].Notes = ""
			}
		}
		service.mu.Unlock()
	})
	notes, _ := pageAt(t, h, "/admin/device/dev_27c38b").Field("f3")
	assert.Equal(t, notes.Type, "textarea")

	device, _ := service.Device("dev_27c38b")
	rec := post(h, "/admin/device/dev_27c38b", url.Values{"_view": {"p.0.0"}, "f0_1": {device.IP}, "f1_0": {device.Status}, "f3": {"Looked after by Ines.\r\nCall before a restart."}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	device, _ = service.Device("dev_27c38b")
	assert.Equal(t, device.Notes, "Looked after by Ines.\nCall before a restart.")
	notes, _ = pageAt(t, h, "/admin/device/dev_27c38b").Field("f3")
	assert.Equal(t, notes.Value, "Looked after by Ines.\nCall before a restart.")
}

func TestSearchAsksTheDetailPagesThatOfferThemselves(t *testing.T) {
	h := handler(t)
	results := func(q string) (groups map[string][]string) {
		groups = map[string][]string{}
		for _, r := range searchResults(t, h, q) {
			groups[r.Group] = append(groups[r.Group], r.Href)
		}
		return groups
	}

	// Each detail page is found under the entry it lights: devices under Devices, sites under Sites.
	groups := results("stock")
	assert.True(t, len(groups["Devices"]) > 0)
	assert.True(t, strings.HasPrefix(groups["Devices"][0], "/admin/device/dev_"))
	assert.DeepEqual(t, groups["Sites"], []string{"/admin/site/Stockholm"})

	// An alert is found by its message, though its page is not the one the alerts are listed on.
	alert := results("ingest lag")
	assert.True(t, len(alert["Alerts"]) > 0)
	assert.True(t, strings.HasPrefix(alert["Alerts"][0], "/admin/alert/alt_"))
}

// A device's crumb is its address, and not the id that the path has; so is the tab title, and a page below it names it so.
func TestADevicesCrumbIsItsAddressAndNotItsID(t *testing.T) {
	h := handler(t)
	d, _ := service.Device("dev_27c38b")
	p := pageAt(t, h, "/admin/device/dev_27c38b")
	crumbs := page.One(t, "the breadcrumb", p.ByRole("navigation", "Breadcrumb"))
	assert.Equal(t, page.One(t, "where we are", crumbs.ByAttr("aria-current", "page")).Text(), d.IP)
	assert.True(t, strings.HasPrefix(p.Title(), d.IP+" — "))
	assert.False(t, strings.Contains(crumbs.Text(), "dev_27c38b"))
}

// The Devices page has a small chart of each device's last hour, with how much it moved beside it, which the column is sorted and filtered by, and
// which has its values in words.
func TestTheDevicesPageHasAChartOfEachDevicesLastHour(t *testing.T) {
	p := pageAt(t, handler(t), "/admin/device")
	assert.Equal(t, len(linksTo(p, "/admin/device?devices.sort=last-hour")), 1)
	charts := 0
	for _, img := range page.One(t, "the devices", p.ByRole("table", "Devices")).ByRole("img", "") {
		if strings.HasPrefix(img.Name(), "Last hour: 12 values, from ") {
			charts++
		}
	}
	assert.True(t, charts >= 9) // a page of the table
	_, filters := p.Field("devices.min.last-hour")
	assert.True(t, filters)
	// The same device draws the same chart every time.
	again := pageAt(t, handler(t), "/admin/device")
	assert.Equal(t, len(again.ByRole("img", "")), len(p.ByRole("img", "")))
}

// changes are the changes of the Last hour column on a page, in the order the rows are in, as numbers.
func changes(t *testing.T, p *page.Page) []float64 {
	t.Helper()
	var out []float64
	table := page.One(t, "the devices", p.ByRole("table", "Devices"))
	last := slices.Index(table.Headers(), "Last hour")
	for _, row := range table.Rows() {
		change, _, _ := strings.Cut(row[last], "% ") // "−24.0% (worse)"
		v, err := strconv.ParseFloat(strings.NewReplacer("−", "-", "+", "").Replace(change), 64)
		assert.NoError(t, err)
		out = append(out, v)
	}
	return out
}

// The Last hour column sorts and filters by the change, which is the number the cell states, and not by its points.
func TestTheLastHourColumnSortsAndFiltersByItsChange(t *testing.T) {
	h := handler(t)
	asc := changes(t, pageAt(t, h, "/admin/device?devices.sort=last-hour"))
	assert.True(t, len(asc) >= 5)
	assert.True(t, slices.IsSorted(asc))
	desc := changes(t, pageAt(t, h, "/admin/device?devices.sort=last-hour&devices.desc=true"))
	assert.True(t, len(desc) >= 5)
	slices.Reverse(desc)
	assert.True(t, slices.IsSorted(desc))
	assert.True(t, desc[len(desc)-1] >= asc[len(asc)-1]) // the first page of the one holds the highest of all, and of the other the lowest

	rising := changes(t, pageAt(t, h, "/admin/device?devices.min.last-hour=0"))
	assert.True(t, len(rising) >= 1)
	for _, v := range rising {
		assert.True(t, v >= 0)
	}
}
