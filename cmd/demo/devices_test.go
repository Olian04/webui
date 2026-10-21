package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestAQueryArgumentNarrowsTheRawEventsAndNotTheRecentOnes(t *testing.T) {
	h := handler(t)
	rows := func(target, title string) int {
		body := get(h, target).Body.String()
		body = body[strings.Index(body, title):]
		return strings.Count(body[:strings.Index(body, "</table>")], `<tr class="">`) // body rows; the header has none
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
	rec := post(h, "/admin/device/dev_27c38b", url.Values{"_leaf": {"p.0.0"}, "f1_0": {"healthy"}, "f0_1": {taken.IP}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity) // the form, drawn again with what was typed, as the answer
	page := rec
	assert.Contains(t, page.Body.String(), "already in use by another device")
	assert.Contains(t, page.Body.String(), `value="`+taken.IP+`"`) // what was typed

	rec = post(h, "/admin/device/dev_27c38b", url.Values{"_leaf": {"p.0.0"}, "f1_0": {"healthy"}, "f0_1": {"10.9.9.9"}})
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
	body := get(h, "/admin/device/dev_27c38b").Body.String()
	for name, n := range counts {
		assert.Contains(t, body, fmt.Sprintf(">%s (%d devices)</option>", name, n))
	}

	rec := post(h, "/admin/device/dev_27c38b", url.Values{"_leaf": {"p.0.0"}, "f0_1": {device.IP}, "f1_0": {device.Status}, "f1_1": {other}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	moved, _ := service.Device("dev_27c38b")
	assert.Equal(t, moved.Site, other)
	body = get(h, "/admin/device/dev_27c38b").Body.String()
	assert.Contains(t, body, fmt.Sprintf(">%s (%d devices)</option>", other, counts[other]+1))
	assert.Contains(t, body, fmt.Sprintf(">%s (%d devices)</option>", device.Site, counts[device.Site]-1))

	// A site that is not one of them is refused.
	rec = post(h, "/admin/device/dev_27c38b", url.Values{"_leaf": {"p.0.0"}, "f0_1": {device.IP}, "f1_0": {device.Status}, "f1_1": {"Atlantis"}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Choose from the options offered.")
}

// A column that has no use for a sort leaves it out: the devices' tags are a list of words, and an event's detail is read.
func TestTheDemoLeavesSortingOutOfTheColumnsThatHaveNoUseForIt(t *testing.T) {
	h := handler(t)
	devices := get(h, "/admin/device").Body.String()
	assert.Contains(t, devices, `href="/admin/device?devices.sort=ip"`)
	assert.False(t, strings.Contains(devices, "devices.sort=tags"))
	assert.Contains(t, devices, `aria-label="Filter Tags"`) // it still filters

	events := get(h, "/admin/device/dev_27c38b").Body.String()
	assert.Contains(t, events, "recent-events.sort=kind")
	assert.False(t, strings.Contains(events, "recent-events.sort=detail"))
}

// The device list has a toolbar: a link to the page that makes a device, and a button that does something. An editor may
// press it, and a viewer sees it disabled with the reason.
func TestTheDeviceListHasAToolbarOfALinkAndAnAction(t *testing.T) {
	editor := get(handler(t), "/admin/device").Body.String()
	// The page it leads to has a form, so it is told where the visitor came from, and Cancel and a save return there.
	assert.Contains(t, editor, `<a class="btn btn-primary btn-sm" href="/admin/device/new?webui.from=%2Fadmin%2Fdevice">Create device</a>`)
	assert.Contains(t, editor, `<button class="btn btn-secondary btn-sm" type="submit" name="_act" value="1">Sync now</button>`)

	rec := post(handler(t), "/admin/device", url.Values{"_leaf": {"p.0"}, "_act": {"1"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), fmt.Sprintf("oSynced %d devices", len(service.devices)))

	viewer := get(handlerAs(t, Viewer), "/admin/device").Body.String()
	assert.Contains(t, viewer, `<span class="gate" data-guard="requires the editor role">`)
	rec = post(handlerAs(t, Viewer), "/admin/device", url.Values{"_leaf": {"p.0"}, "_act": {"1"}})
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

	form := get(h, "/admin/device/new").Body.String()
	assert.Contains(t, form, "New device")
	assert.Contains(t, form, `>Create</button>`)

	taken, _ := service.Device("dev_27c75c")
	rec := post(h, "/admin/device/new", url.Values{"_leaf": {"p"}, "f0": {taken.IP}, "f1_0": {"healthy"}, "f1_1": {"Malmö"}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "already in use by another device")

	rec = post(h, "/admin/device/new", url.Values{"_leaf": {"p"}, "f0": {"10.7.7.7"}, "f1_0": {"quiet"}, "f1_1": {"Malmö"}, "f2": {"edge", "gpu"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, len(service.devices), before+1)
	created := service.devices[before]
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/"+created.ID)
	assert.Equal(t, created.IP, "10.7.7.7")
	assert.Equal(t, created.Site, "Malmö")
	assert.DeepEqual(t, created.Tags, []string{"edge", "gpu"})
	assert.Equal(t, get(h, rec.Header().Get("Location")).Code, http.StatusOK)
}

// A page's argument that only a link sets is shown as a value in a toolbar inside the tab it is for: it is read, not an input.
func TestTheEventsOfADeviceShowTheMinutesArgumentAsAValueInAToolbar(t *testing.T) {
	h := handler(t)
	// The value is in the Raw events tab, which it is for, and not on the page.
	overview := get(h, "/admin/device/dev_27c38b").Body.String()
	assert.False(t, strings.Contains(overview, "Events from"))
	raw := get(h, "/admin/device/dev_27c38b?minutes=30&tabs.tab=raw-events").Body.String()
	assert.Contains(t, raw, `<span class="toolbar-lbl">Events from</span> <span class="toolbar-val"><span class="badges"><span class="badge badge-mute"><span class="dot" aria-hidden="true"></span>The last 30 minutes</span></span></span>`)
	// With no minutes it says there is none, and the toolbar has nothing to submit.
	all := get(h, "/admin/device/dev_27c38b?tabs.tab=raw-events").Body.String()
	assert.Contains(t, all, `<span class="toolbar-lbl">Events from</span> <span class="toolbar-val"><span class="dim" aria-hidden="true">—</span>`)
	assert.False(t, strings.Contains(raw, "_toolbar"))
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
	assert.Contains(t, get(h, "/admin/device/dev_27c38b").Body.String(), `<textarea id="p-0-0-f3" class="input textarea" name="f3"`)

	device, _ := service.Device("dev_27c38b")
	rec := post(h, "/admin/device/dev_27c38b", url.Values{"_leaf": {"p.0.0"}, "f0_1": {device.IP}, "f1_0": {device.Status}, "f3": {"Looked after by Ines.\r\nCall before a restart."}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	device, _ = service.Device("dev_27c38b")
	assert.Equal(t, device.Notes, "Looked after by Ines.\nCall before a restart.")
	assert.Contains(t, get(h, "/admin/device/dev_27c38b").Body.String(), ">\nLooked after by Ines.\nCall before a restart.</textarea>")
}

func TestSearchAsksTheDetailPagesThatOfferThemselves(t *testing.T) {
	h := handler(t)
	results := func(q string) (groups map[string][]string) {
		var got struct {
			Results []struct{ Group, Title, Href string }
		}
		assert.NoError(t, json.Unmarshal(get(h, "/admin/_webui/search?q="+url.QueryEscape(q)).Body.Bytes(), &got))
		groups = map[string][]string{}
		for _, r := range got.Results {
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
	body := get(h, "/admin/device/dev_27c38b").Body.String()
	assert.Contains(t, body, `<span class="cur" aria-current="page">`+d.IP+`</span>`)
	assert.Contains(t, body, "<title>"+d.IP+" — ")
	assert.False(t, strings.Contains(body, `aria-current="page">dev_27c38b</span>`))
}
