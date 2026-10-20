package main

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/test/util/assert"
)

func TestMomentsAreShownInUTCAndFilteredAsMoments(t *testing.T) {
	h := handler(t)

	// The first alert was raised at the seed's "now". It reads "just now", by the column's Format, and
	// the <time> element still holds the moment.
	body := get(h, "/admin/alert").Body.String()
	assert.Contains(t, body, `<time datetime="2026-10-09T11:30:00Z">just now</time>`)
	assert.Contains(t, body, `<time datetime="2026-10-09T10:37:00Z">53 min ago</time>`)

	// An end narrows the alerts to those raised by then: they are 53 minutes apart,
	// counting back from the first, so by 08:00 there are seven.
	narrow := get(h, "/admin/alert?alerts.max.raised=2026-10-09T08:00").Body.String()
	assert.False(t, strings.Contains(narrow, ">alt_000<"))
	assert.Contains(t, narrow, ">alt_010<")

	// A Timestamp reads the same on a device.
	device := get(h, "/admin/device/dev_27c38b").Body.String()
	assert.Contains(t, device, `<time datetime="2026-10-09T11:30:00Z">2026-10-09 11:30</time>`)
}

func TestATimestampWithAStoreIsADateAndTimePicker(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `type="datetime-local" value="2026-10-10T23:30:00" step="1"`) // 36 hours after the seed's now

	// Saved from the picker, in UTC, as Unix seconds.
	post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.25"}, "f1_1": {"80"}, "f2": {"2026-11-01T02:00"}})
	assert.Equal(t, service.Settings().Maintain, int(time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC).Unix()))
}

func TestAURLColumnIsTheOnlyWayAValueBecomesALink(t *testing.T) {
	body := get(handler(t), "/admin/alert/alt_000").Body.String()
	assert.Contains(t, body, `<a class="ext" href="https://example.com/runbooks/critical" target="_blank" rel="noopener noreferrer">`)
	assert.Contains(t, body, "Open the runbook")
}

func TestAFormattedColumnReadsAsItsFormatAndSortsAndFiltersByItsValue(t *testing.T) {
	h := handler(t)

	// Stored reads "1 GiB" and sorts by the bytes: as text, "1 GiB" would come before "1 KiB" and "9 ..." after "10 ...".
	asc := get(h, "/admin/device?devices.sort=stored").Body.String()
	assert.Contains(t, asc, ">1 KiB<")
	assert.False(t, strings.Contains(asc, ">1 GiB<"))
	desc := get(h, "/admin/device?devices.sort=stored&devices.desc=true").Body.String()
	assert.Contains(t, desc, ">1 GiB<")
	assert.False(t, strings.Contains(desc, ">1 KiB<"))

	// The bounds of its filter are bytes, and the export holds the number.
	big := get(h, "/admin/device?devices.min.stored=1073741824").Body.String()
	assert.Contains(t, big, ">1 GiB<")
	assert.False(t, strings.Contains(big, ">512 MiB<"))
	assert.Contains(t, get(h, "/admin/device?devices.export=csv").Body.String(), ",1073761364,") // dev 20: 2^30 + 20*977 bytes
}
