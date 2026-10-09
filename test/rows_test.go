package webui_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// rowsApp is a table that only says what its rows are: 25 devices, and nothing
// about sorting, filtering or paging. The library does those from the columns.
func rowsApp(load func(context.Context) ([]Device, error)) http.Handler {
	if load == nil {
		load = func(context.Context) ([]Device, error) {
			var all []Device
			for i := range 25 {
				all = append(all, Device{Id: fmt.Sprintf("dev%02d", i), Ip: "10.0.0.1", Count: (i * 7) % 25, Duration: 1})
			}
			return all, nil
		}
	}
	page := webui.Page[webui.NoArgs]{
		Path: "/device",
		Body: webui.Table[Device]{
			Title:    "Devices",
			PageSize: 10,
			Rows:     load,
			Columns: []webui.Accessor[Device]{
				webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }},
				webui.Badge[Device]{
					Label: "Status",
					Load: func(d Device) string {
						if d.Count%2 == 0 {
							return "healthy"
						}
						return "degraded"
					},
					Kinds: map[string]webui.Tone{"healthy": webui.ToneOK, "degraded": webui.ToneWarning},
				},
				webui.Int[Device]{Label: "Occurrences", Load: func(d Device) int { return d.Count }},
			},
		},
	}
	return webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
}

// ids are the device ids on a page, in the order shown.
func ids(body string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`<td class="">(dev\d\d)</td>`).FindAllStringSubmatch(body, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestRowsAreSortedFilteredAndPagedByTheLibrary(t *testing.T) {
	t.Parallel()

	h := rowsApp(nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Equal(t, len(ids(body)), 10) // paged: a Rows source never has to know
	assert.Contains(t, body, "1–10 of 25")

	second := serve(h, http.MethodGet, "/admin/device?devices.offset=20").Body.String()
	assert.Equal(t, len(ids(second)), 5)
	assert.Contains(t, second, "21–25 of 25")
}

func TestRowsAreSortedByTheColumnsOwnValueATextAsTextANumberAsANumber(t *testing.T) {
	t.Parallel()

	h := rowsApp(nil)

	// The occurrences are (i*7) % 25: each of 0 to 24 once, in a scrambled order.
	asc := ids(serve(h, http.MethodGet, "/admin/device?devices.sort=occurrences").Body.String())
	assert.Equal(t, asc[0], "dev00") // 0
	assert.Equal(t, asc[1], "dev18") // 1 (18*7 = 126)
	desc := ids(serve(h, http.MethodGet, "/admin/device?devices.sort=occurrences&devices.desc=true").Body.String())
	assert.Equal(t, desc[0], "dev07") // 24, which as text would sort before 3

	byID := ids(serve(h, http.MethodGet, "/admin/device?devices.sort=id&devices.desc=true").Body.String())
	assert.Equal(t, byID[0], "dev24")
}

func TestRowsAreFilteredByEachKindOfFilter(t *testing.T) {
	t.Parallel()

	h := rowsApp(nil)

	// Text: contains, ignoring case.
	text := ids(serve(h, http.MethodGet, "/admin/device?devices.filter.id=V1").Body.String())
	assert.Equal(t, len(text), 10) // dev10 to dev19

	// An option: a Badge's chosen values.
	odd := serve(h, http.MethodGet, "/admin/device?devices.filter.status=degraded").Body.String()
	for _, id := range ids(odd) {
		n, err := strconv.Atoi(strings.TrimPrefix(id, "dev"))
		assert.NoError(t, err)
		assert.Equal(t, ((n*7)%25)%2, 1)
	}

	// A range: a number compared as a number, both ends inclusive.
	ranged := serve(h, http.MethodGet, "/admin/device?devices.min.occurrences=20&devices.max.occurrences=24").Body.String()
	assert.Contains(t, ranged, "1–5 of 5") // 20, 21, 22, 23 and 24, each held by one device
}

func TestAFilterShrinksTheTotalAndThePagesNotJustTheRows(t *testing.T) {
	t.Parallel()

	h := rowsApp(nil)
	body := serve(h, http.MethodGet, "/admin/device?devices.filter.id=dev0").Body.String()
	assert.Contains(t, body, "1–10 of 10") // dev00 to dev09
	assert.Contains(t, body, `disabled>Next</button>`)
}

func TestRowsThatFailFailThePanelNotThePage(t *testing.T) {
	t.Parallel()

	h := rowsApp(func(context.Context) ([]Device, error) { return nil, errors.New("db password is hunter2") })
	rec := serve(h, http.MethodGet, "/admin/device")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), "Could not load")
	assert.False(t, strings.Contains(rec.Body.String(), "hunter2"))
}

func TestATableNeedsExactlyOneOfRowsAndLoad(t *testing.T) {
	t.Parallel()

	page := func(table webui.Table[Device]) webui.App {
		return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: table}}}
	}
	rows := func(context.Context) ([]Device, error) { return nil, nil }
	load := func(context.Context, webui.Query) (webui.Window[Device], error) { return webui.Window[Device]{}, nil }

	var neither string
	for _, e := range compileErrors(t, page(webui.Table[Device]{})) {
		neither += e.Error()
	}
	assert.Contains(t, neither, "a Table has none of Rows, Load and Feed")

	var both string
	for _, e := range compileErrors(t, page(webui.Table[Device]{Rows: rows, Load: load})) {
		both += e.Error()
	}
	assert.Contains(t, both, "a Table has more than one of Rows, Load and Feed")

	for _, one := range []webui.Table[Device]{{Rows: rows}, {Load: load}} {
		_, err := page(one).Compile("")
		assert.NoError(t, err)
	}
}

func TestRowActionsWorkOverRows(t *testing.T) {
	t.Parallel()

	var acted []string
	page := webui.Page[webui.NoArgs]{
		Path: "/device",
		Body: webui.Table[Device]{
			Rows: func(context.Context) ([]Device, error) {
				return []Device{{Id: "a"}, {Id: "b"}, {Id: "c"}}, nil
			},
			Key:     func(d Device) string { return d.Id },
			Columns: []webui.Accessor[Device]{webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}},
			Actions: []webui.Action[Device]{{
				Label: "Go",
				Run: func(_ context.Context, d Device) (webui.Outcome, error) {
					acted = append(acted, d.Id)
					return webui.Outcome{}, nil
				},
			}},
		},
	}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	rec := post(h, "/admin/device", map[string][]string{"_leaf": {"p"}, "_act": {"row:0:b"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, acted, []string{"b"})
}

// storageTable is a table of objects with a storage class, which nobody declares: the
// classes are whatever the rows hold.
func storageTable(rows bool) http.Handler {
	items := []Device{{Id: "a", Ip: "STANDARD"}, {Id: "b", Ip: "glacier"}, {Id: "c", Ip: "STANDARD"}, {Id: "d", Ip: "GLACIER"}}
	class := webui.Badge[Device]{Label: "Class", Load: func(d Device) string { return d.Ip }} // no Kinds
	tbl := webui.Table[Device]{
		Title:   "Objects",
		Columns: []webui.Accessor[Device]{formID, class},
	}
	if rows {
		tbl.Rows = func(context.Context) ([]Device, error) { return items, nil }
	} else {
		tbl.Load = func(context.Context, webui.Query) (webui.Window[Device], error) {
			return webui.Window[Device]{Items: items, Total: len(items)}, nil
		}
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/o", Body: tbl}}}.MustCompile("")
}

// classBoxes is the checkbox of each choice in the Class filter.
var classBoxes = regexp.MustCompile(`class="checkbox" type="checkbox" name="objects\.filter\.class" value="([^"]*)"`)

func TestABadgeOfARowsTableFiltersByTheValuesItsRowsHold(t *testing.T) {
	t.Parallel()

	h := storageTable(true)
	body := serve(h, http.MethodGet, "/o").Body.String()

	// A choice for each value present, in order without regard to case, each once.
	opts := classBoxes.FindAllStringSubmatch(body, -1)
	var got []string
	for _, m := range opts {
		got = append(got, m[1])
	}
	assert.DeepEqual(t, got, []string{"GLACIER", "glacier", "STANDARD"})

	// Choosing one keeps its rows; the choices do not shrink for it.
	chosen := serve(h, http.MethodGet, "/o?objects.filter.class=STANDARD").Body.String()
	assert.Contains(t, chosen, ">a<")
	assert.Contains(t, chosen, ">c<")
	assert.False(t, strings.Contains(chosen, ">b<"))
	assert.Equal(t, len(classBoxes.FindAllString(chosen, -1)), 3)

	// A value no row has is no hazard, and matches nothing.
	none := serve(h, http.MethodGet, "/o?objects.filter.class=NOPE").Body.String()
	assert.False(t, strings.Contains(none, ">a<"))
}

func TestABadgeOfALoadTableWithNoKindsStaysATextFilter(t *testing.T) {
	t.Parallel()

	body := serve(storageTable(false), http.MethodGet, "/o").Body.String()
	assert.Contains(t, body, `placeholder="Contains…"`)
	assert.Equal(t, len(classBoxes.FindAllString(body, -1)), 0) // no checkboxes
}

func TestSeveralValuesOfAnOpenBadgeCanBeChosenTogether(t *testing.T) {
	t.Parallel()

	body := serve(storageTable(true), http.MethodGet, "/o?objects.filter.class=STANDARD&objects.filter.class=GLACIER").Body.String()
	assert.Contains(t, body, ">a<")                // STANDARD
	assert.Contains(t, body, ">d<")                // GLACIER
	assert.False(t, strings.Contains(body, ">b<")) // glacier, which is a different value
	assert.Equal(t, strings.Count(body, " checked"), 2)
}
