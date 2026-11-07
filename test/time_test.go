package webui_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// object has a moment that is always a time.Time, and one that may be absent: the zero time is not set for the first, and nil is for the second.
type object struct {
	Name     string
	Modified time.Time
	Created  *time.Time
}

func unix(seconds int64) *time.Time {
	t := time.Unix(seconds, 0).UTC()
	return &t
}

var (
	objName     = webui.String[object]{Label: "Name", Load: func(o object) string { return o.Name }}
	objModified = webui.Time[object]{Label: "Modified", Load: func(o object) time.Time { return o.Modified }}
	objCreated  = webui.OptionalTime[object]{Label: "Created", Load: func(o object) *time.Time { return o.Created }}
)

func objectsApp(seen *webui.PagedQuery) http.Handler {
	items := []object{
		{Name: "a", Modified: time.Date(2026, 10, 9, 9, 0, 0, 0, time.FixedZone("CEST", 2*3600)), Created: unix(1791540000)}, // 07:00 UTC, though 09:00 on its own clock
		{Name: "b", Modified: time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)},
		{Name: "c", Modified: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Created: unix(1791624000)},
		{Name: "d", Created: unix(1791500000)},
	}
	cols := []webui.Accessor[object]{objName, objModified, objCreated}
	rows := webui.Table[object]{Title: "Objects", Columns: cols, Loader: webui.RowsLoader[object]{Load: func(context.Context) ([]object, webui.Loaded) { return items, webui.Ready() }}}
	load := webui.Table[object]{Title: "Loaded", Columns: cols, Loader: webui.PagedLoader[object]{Load: func(_ context.Context, q webui.PagedQuery) (webui.PagedWindow[object], webui.Loaded) {
		*seen = q
		return webui.PagedWindow[object]{Items: items, Total: len(items)}, webui.Ready()
	}}}
	detail := webui.Form[object]{
		Title: "Object",
		Load: func(context.Context) (object, webui.Loaded) {
			return object{Modified: time.Date(2026, 10, 9, 11, 27, 0, 0, time.UTC)}, webui.Ready()
		},
		Fields: []webui.Accessor[object]{objModified, objCreated},
	}
	return webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/rows", View: rows},
		webui.Page[webui.NoArgs]{Path: "/load", View: load},
		webui.Page[webui.NoArgs]{Path: "/one", View: detail},
	}}.MustCompile("")
}

func TestATimeAndAnOptionalTimeAreShownInUTCAsAMoment(t *testing.T) {
	t.Parallel()

	body := serve(objectsApp(&webui.PagedQuery{}), http.MethodGet, "/rows").Body.String()

	assert.Contains(t, body, `<time datetime="2026-10-09T07:00:00Z">2026-10-09 07:00</time>`) // a zone is converted
	assert.Contains(t, body, `<time datetime="2026-10-10T00:00:00Z">2026-10-10 00:00</time>`) // midnight is a moment like any other
	assert.Contains(t, body, `<time datetime="2026-10-09T10:00:00Z">2026-10-09 10:00</time>`) // 1791540000
	// The zero time and nil are not set: they show as nothing, and are no moment in 0001 or 1970.
	assert.False(t, strings.Contains(body, "0001-01-01"))
	assert.False(t, strings.Contains(body, "1970-01-01"))
}

func TestAMomentColumnSortsChronologicallyNotAsText(t *testing.T) {
	t.Parallel()

	h := objectsApp(&webui.PagedQuery{})
	body := serve(h, http.MethodGet, "/rows?objects.sort=modified").Body.String()
	// a is 07:00 UTC and b 08:00 UTC, though a's wall clock is the greater. The zero time,
	// which is not set, sorts first, and the next day last.
	at := func(s string) int { return strings.Index(body, ">"+s+"</td>") }
	assert.True(t, at("d") < at("a"))
	assert.True(t, at("a") < at("b"))
	assert.True(t, at("b") < at("c"))
}

func TestAMomentColumnIsFilteredByAStartAndAnEndInUTC(t *testing.T) {
	t.Parallel()

	h := objectsApp(&webui.PagedQuery{})
	body := serve(h, http.MethodGet, "/rows?objects.min.modified=2026-10-09T07:30&objects.max.modified=2026-10-09T23:00").Body.String()

	assert.Contains(t, body, ">b</td>")
	assert.False(t, strings.Contains(body, ">a</td>")) // 07:00, before the start
	assert.False(t, strings.Contains(body, ">c</td>")) // the next day
	// The inputs are date and time inputs, holding what was asked.
	assert.Contains(t, body, `type="datetime-local" name="objects.min.modified" value="2026-10-09T07:30"`)
	assert.Contains(t, body, `type="datetime-local" name="objects.max.modified" value="2026-10-09T23:00"`)
}

func TestALoadTableIsHandedAMomentsBoundsAsUnixSeconds(t *testing.T) {
	t.Parallel()

	var seen webui.PagedQuery
	h := objectsApp(&seen)
	serve(h, http.MethodGet, "/load?loaded.min.modified=2026-10-09T07:30&loaded.max.created=1791600000")

	assert.Equal(t, *seen.Ranges["Modified"].Min, float64(time.Date(2026, 10, 9, 7, 30, 0, 0, time.UTC).Unix()))
	assert.Equal(t, seen.Ranges["Modified"].Max == nil, true)
	assert.Equal(t, *seen.Ranges["Created"].Max, float64(1791600000))
}

func TestAMomentInAFormIsReadOnlyOutputAndAnEmptyOneIsADash(t *testing.T) {
	t.Parallel()

	body := serve(objectsApp(&webui.PagedQuery{}), http.MethodGet, "/one").Body.String()
	assert.Contains(t, body, `<time datetime="2026-10-09T11:27:00Z">2026-10-09 11:27</time>`)
	assert.Contains(t, body, `<span class="dim" aria-hidden="true">—</span>`) // Created is nil: not set
	assert.False(t, strings.Contains(body, "<input class=\"input\""))
}

// editMoments is a form whose two moments can be changed. stored is what the last
// submission left in the model.
func editMoments(stored *object) http.Handler {
	when := webui.Time[object]{Label: "Modified", Load: func(o object) time.Time { return o.Modified }, Store: func(o *object, v time.Time) { o.Modified = v }}
	made := webui.OptionalTime[object]{Label: "Created", Load: func(o object) *time.Time { return o.Created }, Store: func(o *object, v *time.Time) { o.Created = v }}
	form := webui.Form[object]{
		Title: "Object",
		Load: func(context.Context) (object, webui.Loaded) {
			return object{Modified: time.Date(2026, 10, 9, 11, 27, 0, 0, time.UTC), Created: unix(1791540000)}, webui.Ready()
		},
		Fields: []webui.Accessor[object]{when, made},
		Submit: webui.Action[object]{Run: func(_ context.Context, o object) (webui.Outcome, error) {
			*stored = o
			return webui.Success("Saved"), nil
		}},
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/edit", View: form}}}.MustCompile("")
}

func TestAMomentWithAStoreIsADateAndTimePickerInUTC(t *testing.T) {
	t.Parallel()

	body := serve(editMoments(&object{}), http.MethodGet, "/edit").Body.String()

	// The native picker, holding the moment in UTC to the second: not the text a table shows.
	assert.Contains(t, body, `type="datetime-local" value="2026-10-09T11:27:00" step="1"`)
	assert.Contains(t, body, `type="datetime-local" value="2026-10-09T10:00:00" step="1"`) // 1791540000
	assert.Equal(t, strings.Count(body, "<time"), 1)                                       // the status bar's build time, and no moment of the form
	// What the picker means is said once, beside the label.
	assert.Contains(t, body, `<span class="tip-item">Date and time in UTC</span>`)
}

func TestAMomentIsStoredInUTCAndAClearedPickerIsNotSet(t *testing.T) {
	t.Parallel()

	var stored object
	h := editMoments(&stored)
	rec := post(h, "/edit", formValues("f0", "2026-10-10T08:15:30", "f1", "2026-10-10T08:15"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.True(t, stored.Modified.Equal(time.Date(2026, 10, 10, 8, 15, 30, 0, time.UTC)))
	assert.Equal(t, stored.Modified.Location(), time.UTC)
	assert.True(t, stored.Created.Equal(time.Date(2026, 10, 10, 8, 15, 0, 0, time.UTC)))

	// A cleared picker is no value, not an error: the zero time for a Time, and nil for an OptionalTime.
	rec = post(h, "/edit", formValues("f0", "", "f1", ""))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.True(t, stored.Modified.IsZero())
	assert.True(t, stored.Created == nil)
}

func TestAMomentThatIsNotOneIsRefusedAndWhatWasTypedStays(t *testing.T) {
	t.Parallel()

	var stored object
	h := editMoments(&stored)
	rec := post(h, "/edit", formValues("f0", "next tuesday", "f1", "2026-10-10T08:15"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	body := rec.Body.String()
	assert.Contains(t, body, `value="next tuesday"`)
	assert.Contains(t, body, "Could not read this value.")
	assert.True(t, stored.Modified.IsZero()) // nothing was saved
}

func TestAMomentThatIsNotSetIsDroppedByAnyRangeEvenOneWithOnlyAnEnd(t *testing.T) {
	t.Parallel()

	h := objectsApp(&webui.PagedQuery{})

	// b has no Created (nil is not set), and d no Modified (the zero time is). A filter
	// with only an end would keep them if "not set" were taken for a very early time.
	created := serve(h, http.MethodGet, "/rows?objects.max.created=1791999999").Body.String()
	assert.False(t, strings.Contains(created, ">b</td>")) // Created is nil
	assert.Contains(t, created, ">a</td>")

	modified := serve(h, http.MethodGet, "/rows?objects.max.modified=2026-12-31T00:00").Body.String()
	assert.False(t, strings.Contains(modified, ">d</td>")) // Modified is the zero time
	assert.Contains(t, modified, ">a</td>")

	// And with a start, as before.
	started := serve(h, http.MethodGet, "/rows?objects.min.created=1").Body.String()
	assert.False(t, strings.Contains(started, ">b</td>"))
}

// A Time is blank for the zero time and may be left that way, unless it is Required: the zero time is an empty value of its own, so it is a flag
// there, as for a String. An OptionalTime has no Required, since a moment that may be absent is not wanted.
func TestATimeThatIsRequiredRefusesAClearedPicker(t *testing.T) {
	t.Parallel()

	var stored object
	required := webui.Time[object]{Label: "Modified", Required: true, Load: func(o object) time.Time { return o.Modified }, Store: func(o *object, v time.Time) { o.Modified = v }}
	form := webui.Form[object]{
		Load:   func(context.Context) (object, webui.Loaded) { return object{}, webui.Ready() }, // no moment: the picker is blank
		Fields: []webui.Accessor[object]{required},
		Submit: webui.Action[object]{Run: func(_ context.Context, o object) (webui.Outcome, error) {
			stored = o
			return webui.Success("Saved"), nil
		}},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/edit", View: form}}}.MustCompile("")

	body := serve(h, http.MethodGet, "/edit").Body.String()
	assert.Contains(t, body, `type="datetime-local" value=""`)
	assert.Contains(t, body, `class="req"`)
	rec := post(h, "/edit", formValues("f0", ""))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "This field is required.")
	assert.True(t, stored.Modified.IsZero())

	assert.Equal(t, post(h, "/edit", formValues("f0", "2026-10-10T08:15")).Code, http.StatusSeeOther)
	assert.False(t, stored.Modified.IsZero())
}
