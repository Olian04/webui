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
