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

type object struct {
	Name     string
	Modified string // ISO 8601
	Created  int    // Unix seconds
}

var (
	objName     = webui.String[object]{Label: "Name", Load: func(o object) string { return o.Name }}
	objModified = webui.Datetime[object]{Label: "Modified", Load: func(o object) string { return o.Modified }}
	objCreated  = webui.Timestamp[object]{Label: "Created", Load: func(o object) int { return o.Created }}
)

func objectsApp(seen *webui.Query) http.Handler {
	items := []object{
		{Name: "a", Modified: "2026-10-09T09:00:00+02:00", Created: 1791540000}, // Modified is 07:00 UTC, though its text sorts after b's
		{Name: "b", Modified: "2026-10-09T08:00:00Z", Created: 0},
		{Name: "c", Modified: "2026-10-10", Created: 1791624000},
		{Name: "d", Modified: "not a date", Created: 1791500000},
	}
	cols := []webui.Accessor[object]{objName, objModified, objCreated}
	rows := webui.Table[object]{Title: "Objects", Columns: cols, Rows: func(context.Context) ([]object, error) { return items, nil }}
	load := webui.Table[object]{Title: "Loaded", Columns: cols, Load: func(_ context.Context, q webui.Query) (webui.Window[object], error) {
		*seen = q
		return webui.Window[object]{Items: items, Total: len(items)}, nil
	}}
	detail := webui.Form[object]{
		Title:  "Object",
		Load:   func(context.Context) (object, error) { return object{Modified: "2026-10-09T11:27:00Z"}, nil },
		Fields: []webui.Accessor[object]{objModified, objCreated},
	}
	return webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/rows", Body: rows},
		webui.Page[webui.NoArgs]{Path: "/load", Body: load},
		webui.Page[webui.NoArgs]{Path: "/one", Body: detail},
	}}.MustCompile("")
}

func TestADatetimeAndATimestampAreShownInUTCAsAMoment(t *testing.T) {
	t.Parallel()

	body := serve(objectsApp(&webui.Query{}), http.MethodGet, "/rows").Body.String()

	assert.Contains(t, body, `<time datetime="2026-10-09T07:00:00Z">2026-10-09 07:00</time>`) // a zone is converted
	assert.Contains(t, body, `<time datetime="2026-10-10">2026-10-10</time>`)                 // a date alone stays one
	assert.Contains(t, body, ">not a date<")                                                  // text that is not a moment is as written
	assert.False(t, strings.Contains(body, `datetime="not a date"`))
	assert.Contains(t, body, `<time datetime="2026-10-09T10:00:00Z">2026-10-09 10:00</time>`) // 1791540000
}

func TestAMomentColumnSortsChronologicallyNotAsText(t *testing.T) {
	t.Parallel()

	h := objectsApp(&webui.Query{})
	body := serve(h, http.MethodGet, "/rows?objects.sort=modified").Body.String()
	// a is 07:00 UTC and b 08:00 UTC, though a's text is the greater. The text that is
	// no moment sorts first, and the date alone last.
	at := func(s string) int { return strings.Index(body, ">"+s+"</td>") }
	assert.True(t, at("d") < at("a"))
	assert.True(t, at("a") < at("b"))
	assert.True(t, at("b") < at("c"))
}

func TestAMomentColumnIsFilteredByAStartAndAnEndInUTC(t *testing.T) {
	t.Parallel()

	h := objectsApp(&webui.Query{})
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

	var seen webui.Query
	h := objectsApp(&seen)
	serve(h, http.MethodGet, "/load?loaded.min.modified=2026-10-09T07:30&loaded.max.created=1791600000")

	assert.Equal(t, *seen.Ranges["Modified"].Min, float64(time.Date(2026, 10, 9, 7, 30, 0, 0, time.UTC).Unix()))
	assert.Equal(t, seen.Ranges["Modified"].Max == nil, true)
	assert.Equal(t, *seen.Ranges["Created"].Max, float64(1791600000))
}

func TestAMomentInAFormIsReadOnlyOutputAndAnEmptyOneIsADash(t *testing.T) {
	t.Parallel()

	body := serve(objectsApp(&webui.Query{}), http.MethodGet, "/one").Body.String()
	assert.Contains(t, body, `<time datetime="2026-10-09T11:27:00Z">2026-10-09 11:27</time>`)
	assert.Contains(t, body, `<span class="dim" aria-hidden="true">—</span>`) // Created is zero: not set
	assert.False(t, strings.Contains(body, "<input class=\"input\""))
}

// editMoments is a form whose two moments can be changed. stored is what the last
// submission left in the model.
func editMoments(stored *object) http.Handler {
	when := webui.Datetime[object]{Label: "Modified", Load: func(o object) string { return o.Modified }, Store: func(o *object, v string) { o.Modified = v }}
	made := webui.Timestamp[object]{Label: "Created", Load: func(o object) int { return o.Created }, Store: func(o *object, v int) { o.Created = v }}
	form := webui.Form[object]{
		Title: "Object",
		Load: func(context.Context) (object, error) {
			return object{Modified: "2026-10-09T11:27:00Z", Created: 1791540000}, nil
		},
		Fields: []webui.Accessor[object]{when, made},
		Submit: webui.Action[object]{Run: func(_ context.Context, o object) (webui.Outcome, error) {
			*stored = o
			return webui.Success("Saved"), nil
		}},
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/edit", Body: form}}}.MustCompile("")
}

func TestAMomentWithAStoreIsADateAndTimePickerInUTC(t *testing.T) {
	t.Parallel()

	body := serve(editMoments(&object{}), http.MethodGet, "/edit").Body.String()

	// The native picker, holding the moment in UTC to the second: not the text a table shows.
	assert.Contains(t, body, `type="datetime-local" value="2026-10-09T11:27:00" step="1"`)
	assert.Contains(t, body, `type="datetime-local" value="2026-10-09T10:00:00" step="1"`) // 1791540000
	assert.False(t, strings.Contains(body, "<time"))
	// What the picker means is said once, beside the label.
	assert.Contains(t, body, `<span class="tip-item">Date and time in UTC</span>`)
}

func TestAMomentIsStoredAsRFC3339InUTCAndAsUnixSeconds(t *testing.T) {
	t.Parallel()

	var stored object
	h := editMoments(&stored)
	rec := post(h, "/edit", formValues("f0", "2026-10-10T08:15:30", "f1", "2026-10-10T08:15"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, stored.Modified, "2026-10-10T08:15:30Z")
	assert.Equal(t, stored.Created, int(time.Date(2026, 10, 10, 8, 15, 0, 0, time.UTC).Unix()))

	// A cleared picker is no value, not an error.
	rec = post(h, "/edit", formValues("f0", "", "f1", ""))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, stored.Modified, "")
	assert.Equal(t, stored.Created, 0)
}

func TestAMomentThatIsNotOneIsRefusedAndWhatWasTypedStays(t *testing.T) {
	t.Parallel()

	var stored object
	rec := post(editMoments(&stored), "/edit", formValues("f0", "next tuesday", "f1", "2026-10-10T08:15"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), `value="next tuesday"`)
	assert.Contains(t, rec.Body.String(), "Could not read this value.")
	assert.Equal(t, stored.Modified, "") // nothing was saved
}

func TestAMomentThatIsNotSetIsDroppedByAnyRangeEvenOneWithOnlyAnEnd(t *testing.T) {
	t.Parallel()

	h := objectsApp(&webui.Query{})

	// b has no Timestamp (zero is not set), and d no Datetime that is one. A filter
	// with only an end would keep them if "not set" were taken for a very early time.
	created := serve(h, http.MethodGet, "/rows?objects.max.created=1791999999").Body.String()
	assert.False(t, strings.Contains(created, ">b</td>")) // Created is zero
	assert.Contains(t, created, ">a</td>")

	modified := serve(h, http.MethodGet, "/rows?objects.max.modified=2026-12-31T00:00").Body.String()
	assert.False(t, strings.Contains(modified, ">d</td>")) // "not a date"
	assert.Contains(t, modified, ">a</td>")

	// And with a start, as before.
	started := serve(h, http.MethodGet, "/rows?objects.min.created=1").Body.String()
	assert.False(t, strings.Contains(started, ">b</td>"))
}
