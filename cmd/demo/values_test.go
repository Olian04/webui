package main

import (
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func TestMomentsAreShownInUTCAndFilteredAsMoments(t *testing.T) {
	h := handler(t)

	// The first alert was raised at the seed's "now". It reads "just now", by the column's Format, and
	// the <time> element still holds the moment.
	raised := moments(mainOf(t, pageAt(t, h, "/admin/alert")))
	assert.Equal(t, raised[0], moment{"2026-10-09T11:30:00Z", "just now"})
	assert.Equal(t, raised[1], moment{"2026-10-09T10:37:00Z", "53 min ago"})

	// An end narrows the alerts to those raised by then: they are 53 minutes apart,
	// counting back from the first, so by 08:00 there are seven.
	narrow := cells(t, page.One(t, "the alerts", pageAt(t, h, "/admin/alert?alerts.max.raised=2026-10-09T08:00").ByRole("table", "")), "ID")
	assert.False(t, slices.Contains(narrow, "alt_000"))
	assert.True(t, slices.Contains(narrow, "alt_010"))

	// A Time reads the same on a device.
	device := moments(mainOf(t, pageAt(t, h, "/admin/device/dev_27c38b")))
	assert.True(t, slices.Contains(device, moment{"2026-10-09T11:30:00Z", "2026-10-09 11:30"}))
}

func TestATimeWithAStoreIsADateAndTimePicker(t *testing.T) {
	h := handler(t)
	picker, _ := pageAt(t, h, "/admin/settings").Field("f2")
	assert.Equal(t, picker.Type, "datetime-local")
	assert.Equal(t, picker.Value, "2026-10-10T23:30:00") // 36 hours after the seed's now

	// Saved from the picker, in UTC, as Unix seconds.
	post(h, "/admin/settings", url.Values{"_view": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.25"}, "f1_1": {"80"}, "f2": {"2026-11-01T02:00"}})
	assert.True(t, service.Settings().Maintain.Equal(time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC)))
}

func TestAURLColumnIsTheOnlyWayAValueBecomesALink(t *testing.T) {
	p := pageAt(t, handler(t), "/admin/alert/alt_000")
	runbook := page.One(t, "the runbook", p.ByRole("link", "Open the runbook(opens in a new tab)"))
	assert.Equal(t, runbook.Attr("href"), "https://example.com/runbooks/critical")
	assert.Equal(t, runbook.Attr("target"), "_blank")
	assert.Equal(t, runbook.Attr("rel"), "noopener noreferrer")
}

func TestAFormattedColumnReadsAsItsFormatAndSortsAndFiltersByItsValue(t *testing.T) {
	h := handler(t)

	// Stored reads "1 GiB" and sorts by the bytes: as text, "1 GiB" would come before "1 KiB" and "9 ..." after "10 ...".
	stored := func(target string) []string {
		return cells(t, page.One(t, "the devices", pageAt(t, h, target).ByRole("table", "Devices")), "Stored")
	}
	asc := stored("/admin/device?devices.sort=stored")
	assert.True(t, slices.Contains(asc, "1 KiB"))
	assert.False(t, slices.Contains(asc, "1 GiB"))
	desc := stored("/admin/device?devices.sort=stored&devices.desc=true")
	assert.True(t, slices.Contains(desc, "1 GiB"))
	assert.False(t, slices.Contains(desc, "1 KiB"))

	// The bounds of its filter are bytes, and the export holds the number.
	big := stored("/admin/device?devices.min.stored=1073741824")
	assert.True(t, slices.Contains(big, "1 GiB"))
	assert.False(t, slices.Contains(big, "512 MiB"))
	assert.Contains(t, get(h, "/admin/device?devices.export=csv").Body.String(), ",1073761364,") // dev 20: 2^30 + 20*977 bytes
}
