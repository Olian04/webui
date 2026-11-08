package webui_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// rowsApp is a table that only says what its rows are: 25 devices, and nothing
// about sorting, filtering or paging. The library does those from the columns.
func rowsApp(load func(context.Context) ([]Device, webui.Loaded)) http.Handler {
	if load == nil {
		load = func(context.Context) ([]Device, webui.Loaded) {
			var all []Device
			for i := range 25 {
				all = append(all, Device{Id: fmt.Sprintf("dev%02d", i), Ip: "10.0.0.1", Count: (i * 7) % 25, Duration: 1})
			}
			return all, webui.Ready()
		}
	}
	page := webui.Page[webui.NoArgs]{
		Path: "/device",
		View: webui.Table[Device]{
			Title:    "Devices",
			PageSize: 10,
			Loader:   webui.RowsLoader[Device]{Load: load},
			Columns: []webui.Accessor[Device]{
				webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }},
				webui.Select[Device]{
					Label: "Status",
					Load: func(d Device) []string {
						if d.Count%2 == 0 {
							return []string{"healthy"}
						}
						return []string{"degraded"}
					},
					Options: []webui.Option{{Value: "healthy", Tone: webui.ToneOK}, {Value: "degraded", Tone: webui.ToneWarning}},
				},
				webui.Int[Device]{Label: "Occurrences", Load: func(d Device) int { return d.Count }},
			},
		},
	}
	return webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
}

// ids are the device ids on a page, in the order shown: the first cell of each row of its table.
func ids(t *testing.T, body string) []string {
	t.Helper()

	var out []string
	for _, row := range page.One(t, "table", page.Parse(t, body).ByRole("table", "")).Rows() {
		if len(row) > 1 { // the row that says there is nothing to show is one cell wide
			out = append(out, row[0])
		}
	}
	return out
}

func TestRowsAreSortedFilteredAndPagedByTheLibrary(t *testing.T) {
	t.Parallel()

	h := rowsApp(nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Equal(t, len(ids(t, body)), 10) // paged: a Rows source never has to know
	assert.Contains(t, body, "1–10 of 25")

	second := serve(h, http.MethodGet, "/admin/device?devices.offset=20").Body.String()
	assert.Equal(t, len(ids(t, second)), 5)
	assert.Contains(t, second, "21–25 of 25")
}

func TestRowsAreSortedByTheColumnsOwnValueATextAsTextANumberAsANumber(t *testing.T) {
	t.Parallel()

	h := rowsApp(nil)

	// The occurrences are (i*7) % 25: each of 0 to 24 once, in a scrambled order.
	asc := ids(t, serve(h, http.MethodGet, "/admin/device?devices.sort=occurrences").Body.String())
	assert.Equal(t, asc[0], "dev00") // 0
	assert.Equal(t, asc[1], "dev18") // 1 (18*7 = 126)
	desc := ids(t, serve(h, http.MethodGet, "/admin/device?devices.sort=occurrences&devices.desc=true").Body.String())
	assert.Equal(t, desc[0], "dev07") // 24, which as text would sort before 3

	byID := ids(t, serve(h, http.MethodGet, "/admin/device?devices.sort=id&devices.desc=true").Body.String())
	assert.Equal(t, byID[0], "dev24")
}

func TestRowsAreFilteredByEachKindOfFilter(t *testing.T) {
	t.Parallel()

	h := rowsApp(nil)

	// Text: contains, ignoring case.
	text := ids(t, serve(h, http.MethodGet, "/admin/device?devices.filter.id=V1").Body.String())
	assert.Equal(t, len(text), 10) // dev10 to dev19

	// An option: a Badge's chosen values.
	odd := serve(h, http.MethodGet, "/admin/device?devices.filter.status=degraded").Body.String()
	for _, id := range ids(t, odd) {
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
	assert.Contains(t, body, `disabled data-in-place>Next</button>`)
}

func TestRowsThatFailFailThePanelNotThePage(t *testing.T) {
	t.Parallel()

	h := rowsApp(func(context.Context) ([]Device, webui.Loaded) {
		return nil, webui.Failed(errors.New("db password is hunter2"))
	})
	rec := serve(h, http.MethodGet, "/admin/device")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), "Could not load")
	assert.False(t, strings.Contains(rec.Body.String(), "hunter2"))
}

// A table has one source, a Loader, so "none" is the one thing to say: two cannot be written, and what a loader needs is checked where it is.
func TestATableNeedsALoaderAndEachKindOfItNeedsItsLoad(t *testing.T) {
	t.Parallel()

	page := func(table webui.Table[Device]) webui.App {
		return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", View: table}}}
	}
	rows := func(context.Context) ([]Device, webui.Loaded) { return nil, webui.Ready() }
	load := func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
		return webui.PagedWindow[Device]{}, webui.Ready()
	}

	var neither string
	for _, e := range compileErrors(t, page(webui.Table[Device]{})) {
		neither += e.Error()
	}
	assert.Contains(t, neither, "a Table has no Loader")

	for _, one := range []webui.Table[Device]{
		{Loader: webui.RowsLoader[Device]{Load: rows}},
		{Loader: webui.PagedLoader[Device]{Load: load}},
	} {
		_, err := page(one).Compile("")
		assert.NoError(t, err)
	}
}

func TestRowActionsWorkOverRows(t *testing.T) {
	t.Parallel()

	var acted []string
	page := webui.Page[webui.NoArgs]{
		Path: "/device",
		View: webui.Table[Device]{
			Loader: webui.RowsLoader[Device]{Load: func(context.Context) ([]Device, webui.Loaded) {
				return []Device{{Id: "a"}, {Id: "b"}, {Id: "c"}}, webui.Ready()
			}},
			Key:     func(d Device) string { return d.Id },
			Columns: []webui.Accessor[Device]{webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}},
			Actions: []webui.Control[Device]{webui.Action[Device]{
				Label: "Go",
				Run: func(_ context.Context, d Device) (webui.Outcome, error) {
					acted = append(acted, d.Id)
					return webui.Outcome{}, nil
				},
			}},
		},
	}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	rec := post(h, "/admin/device", map[string][]string{"_view": {"p"}, "_act": {"row:0:b"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, acted, []string{"b"})
}

// storageTable is a table of objects with a storage class, which nobody declares: the
// classes are whatever the rows hold.
func storageTable(rows bool) http.Handler {
	items := []Device{{Id: "a", Ip: "STANDARD"}, {Id: "b", Ip: "glacier"}, {Id: "c", Ip: "STANDARD"}, {Id: "d", Ip: "GLACIER"}}
	class := webui.Select[Device]{Label: "Class", Load: func(d Device) []string { return []string{d.Ip} }} // no Options
	tbl := webui.Table[Device]{
		Title:   "Objects",
		Columns: []webui.Accessor[Device]{formID, class},
	}
	if rows {
		tbl.Loader = webui.RowsLoader[Device]{Load: func(context.Context) ([]Device, webui.Loaded) { return items, webui.Ready() }}
	} else {
		tbl.Loader = webui.PagedLoader[Device]{Load: func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
			return webui.PagedWindow[Device]{Items: items, Total: len(items)}, webui.Ready()
		}}
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/o", View: tbl}}}.MustCompile("")
}

// classBoxes is the checkbox of each choice in the Class filter.
func classBoxes(t *testing.T, body string) []page.Field {
	t.Helper()

	var out []page.Field
	for _, f := range fieldsNamed(page.Parse(t, body), "objects.filter.class") {
		if f.Type == "checkbox" {
			out = append(out, f)
		}
	}
	return out
}

func TestABadgeOfARowsTableFiltersByTheValuesItsRowsHold(t *testing.T) {
	t.Parallel()

	h := storageTable(true)
	body := serve(h, http.MethodGet, "/o").Body.String()

	// A choice for each value present, in order without regard to case, each once.
	var got []string
	for _, f := range classBoxes(t, body) {
		got = append(got, f.Value)
	}
	assert.DeepEqual(t, got, []string{"GLACIER", "glacier", "STANDARD"})

	// Choosing one keeps its rows; the choices do not shrink for it.
	chosen := serve(h, http.MethodGet, "/o?objects.filter.class=STANDARD").Body.String()
	assert.DeepEqual(t, ids(t, chosen), []string{"a", "c"})
	assert.Equal(t, len(classBoxes(t, chosen)), 3)

	// A value no row has is no hazard, and matches nothing.
	none := serve(h, http.MethodGet, "/o?objects.filter.class=NOPE").Body.String()
	assert.Equal(t, len(ids(t, none)), 0)
}

func TestABadgeOfALoadTableWithNoKindsStaysATextFilter(t *testing.T) {
	t.Parallel()

	body := serve(storageTable(false), http.MethodGet, "/o").Body.String()
	assert.Equal(t, len(page.Parse(t, body).ByAttr("placeholder", "Contains…")), 2) // the ID and the Class: both ask for text
	assert.Equal(t, len(classBoxes(t, body)), 0)                                    // no checkboxes
}

func TestSeveralValuesOfAnOpenBadgeCanBeChosenTogether(t *testing.T) {
	t.Parallel()

	body := serve(storageTable(true), http.MethodGet, "/o?objects.filter.class=STANDARD&objects.filter.class=GLACIER").Body.String()
	assert.DeepEqual(t, ids(t, body), []string{"a", "c", "d"}) // STANDARD and GLACIER, and not glacier, which is a different value
	chosen := 0
	for _, f := range classBoxes(t, body) {
		if f.Checked {
			chosen++
		}
	}
	assert.Equal(t, chosen, 2)
}
