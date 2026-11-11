package webui_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

type blob struct {
	Name string
	Size int
	Pct  float64
	Made time.Time
	Seen time.Time
}

// human is the application's own formatter: the library ships none.
func human(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%d MiB", n>>20)
	case n >= 1<<10:
		return fmt.Sprintf("%d KiB", n>>10)
	}
	return fmt.Sprintf("%d B", n)
}

var (
	blobName = webui.String[blob]{Label: "Name", Load: func(b blob) string { return b.Name }}
	blobSize = webui.Int[blob]{Label: "Size", Load: func(b blob) int { return b.Size }, Format: human}
	blobPct  = webui.Float[blob]{Label: "Share", Load: func(b blob) float64 { return b.Pct }, Format: func(x float64) string { return fmt.Sprintf("%.0f%%", x*100) }}
	blobMade = webui.Time[blob]{Label: "Made", Load: func(b blob) time.Time { return b.Made }, Format: func(t time.Time) string { return t.Format("Jan 2") }}
	blobSeen = webui.Time[blob]{Label: "Seen", Load: func(b blob) time.Time { return b.Seen }, Format: func(t time.Time) string { return t.Format("15h04") }}
)

// blobs sort one way as numbers and another as text: "1024" and "1048576" come before "9".
func blobs() []blob {
	return []blob{
		{Name: "a", Size: 1 << 20, Pct: 0.5, Made: time.Unix(1791540000, 0).UTC(), Seen: time.Date(2026, 10, 9, 11, 27, 0, 0, time.UTC)},
		{Name: "b", Size: 9, Pct: 0.05},
		{Name: "c", Size: 1 << 10, Pct: 0.25, Made: time.Unix(1791543600, 0).UTC()},
	}
}

func blobApp() http.Handler {
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{
		Path: "/b",
		Search: webui.SearchRows(
			func(context.Context) ([]blob, webui.Loaded) { return blobs(), webui.Ready() },
			func(b blob) webui.Hit[webui.NoArgs] { return webui.Hit[webui.NoArgs]{Title: b.Name} },
			blobName, blobSize, blobPct,
		),
		View: webui.Table[blob]{
			Title:   "Blobs",
			Loader:  webui.RowsLoader[blob]{Load: func(context.Context) ([]blob, webui.Loaded) { return blobs(), webui.Ready() }},
			Key:     func(b blob) string { return b.Name },
			Columns: []webui.Accessor[blob]{blobName, blobSize, blobPct, blobMade, blobSeen},
			Export:  true,
		},
	}}}.MustCompile("/admin")
}

// order is whether the markers appear in the body in this order.
func order(body string, markers ...string) bool {
	last := -1
	for _, m := range markers {
		i := strings.Index(body, m)
		if i <= last {
			return false
		}
		last = i
	}
	return true
}

func TestAFormattedColumnReadsAsItsFormatAndSortsByItsValue(t *testing.T) {
	t.Parallel()

	h := blobApp()
	body := serve(h, http.MethodGet, "/admin/b").Body.String()
	for _, shown := range []string{">9 B<", ">1 KiB<", ">1 MiB<", ">50%<", ">5%<"} {
		assert.Contains(t, body, shown)
	}
	assert.False(t, strings.Contains(body, ">1048576<")) // the value is not what is read

	asc := serve(h, http.MethodGet, "/admin/b?blobs.sort=size").Body.String()
	assert.True(t, order(asc, ">9 B<", ">1 KiB<", ">1 MiB<")) // by the number: as text, "1024" would come before "9"
	desc := serve(h, http.MethodGet, "/admin/b?blobs.sort=size&blobs.desc=true").Body.String()
	assert.True(t, order(desc, ">1 MiB<", ">1 KiB<", ">9 B<"))
	share := serve(h, http.MethodGet, "/admin/b?blobs.sort=share").Body.String()
	assert.True(t, order(share, ">5%<", ">25%<", ">50%<"))
}

func TestARangeFilterIsTypedInTheValueAndKeepsTheRowsAboveIt(t *testing.T) {
	t.Parallel()

	body := serve(blobApp(), http.MethodGet, "/admin/b?blobs.min.size=1000").Body.String()
	assert.Contains(t, body, ">1 KiB<")
	assert.Contains(t, body, ">1 MiB<")
	assert.False(t, strings.Contains(body, ">9 B<"))
}

func TestTheSearchFindsAFormattedColumnByWhatIsShownAndByTheValue(t *testing.T) {
	t.Parallel()

	h := blobApp()
	titles := func(q string) []string {
		var out []string
		for _, r := range search(t, h, url.QueryEscape(q)).Results {
			out = append(out, r.Title)
		}
		return out
	}
	assert.DeepEqual(t, titles("KiB"), []string{"c"})
	assert.DeepEqual(t, titles("1024"), []string{"c"})
	assert.DeepEqual(t, titles("50%"), []string{"a"})
}

func TestTheExportHoldsTheValuesAndNotTheirFormat(t *testing.T) {
	t.Parallel()

	h := blobApp()
	csv := serve(h, http.MethodGet, "/admin/b?blobs.export=csv").Body.String()
	assert.Contains(t, csv, "a,1048576,0.5,")
	assert.False(t, strings.Contains(csv, "MiB"))

	var rows []map[string]any
	assert.NoError(t, json.Unmarshal(serve(h, http.MethodGet, "/admin/b?blobs.export=json").Body.Bytes(), &rows))
	assert.Equal(t, rows[0]["Size"].(float64), float64(1<<20)) // a number, not "1 MiB"
	assert.Equal(t, rows[0]["Share"].(float64), 0.5)
}

func TestAMomentReadsAsItsFormatAndStillHoldsAndSortsByTheMoment(t *testing.T) {
	t.Parallel()

	h := blobApp()
	// Read as the format, held as the moment.
	want := []moment{{"2026-10-09T10:00:00Z", "Oct 9"}, {"2026-10-09T11:27:00Z", "11h27"}, {"2026-10-09T11:00:00Z", "Oct 9"}}
	got := moments(t, pageOf(t, h, "/admin/b"))
	assert.DeepEqual(t, got, want) // two Made and one Seen: the zero time is not set, so it is not handed to Format and shows as nothing
	for _, m := range got {
		assert.False(t, strings.HasPrefix(m.At, "0001"))
	}
	assert.Equal(t, len(pageOf(t, h, "/admin/b").ByText("Jan 1")), 0)

	// What the app wrote is left as it is, and the moment is in the title in UTC, with its zone, so a reader of "Oct 9" can have the rest.
	for _, el := range page.One(t, "main", pageOf(t, h, "/admin/b").ByRole("main", "")).ByAttr("datetime", "") {
		assert.Equal(t, el.Attr("title"), strings.Replace(strings.Replace(el.Attr("datetime"), "T", " ", 1), ":00Z", " UTC", 1))
	}

	// A sort is by the moment, not by what it reads as: the two Made both read "Oct 9".
	made := func(target string) []string {
		var at []string
		for _, m := range moments(t, pageOf(t, h, target)) {
			if m.Text == "Oct 9" {
				at = append(at, m.At)
			}
		}
		return at
	}
	assert.DeepEqual(t, made("/admin/b?blobs.sort=made"), []string{"2026-10-09T10:00:00Z", "2026-10-09T11:00:00Z"})
	assert.DeepEqual(t, made("/admin/b?blobs.sort=made&blobs.desc=true"), []string{"2026-10-09T11:00:00Z", "2026-10-09T10:00:00Z"})
}

type sized struct{ Size int }

func TestAFieldThatCanBeEditedHoldsTheValueAndOneThatCannotReadsAsItsFormat(t *testing.T) {
	t.Parallel()

	var saved sized
	editable := webui.Int[sized]{Label: "Size", Load: func(s sized) int { return s.Size }, Store: func(s *sized, v int) { s.Size = v }, Format: human}
	readOnly := webui.Int[sized]{Label: "Was", Load: func(s sized) int { return s.Size }, Format: human}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", View: webui.Form[sized]{
		Load:   func(context.Context) (sized, webui.Loaded) { return sized{Size: 1 << 20}, webui.Ready() },
		Fields: []webui.Accessor[sized]{editable, readOnly},
		Submit: webui.Action[sized]{Run: func(_ context.Context, s sized) (webui.Outcome, error) {
			saved = s
			return webui.Success("Saved"), nil
		}},
	}}}}.MustCompile("/admin")

	body := serve(h, http.MethodGet, "/admin/s").Body.String()
	assert.Contains(t, body, `value="1048576"`) // what is edited is the number, so what is typed is what is stored
	assert.Contains(t, body, "1 MiB")           // what is only read is the format
	assert.Equal(t, post(h, "/admin/s", url.Values{"_view": {"p"}, "f0": {"2048"}}).Code, http.StatusSeeOther)
	assert.Equal(t, saved.Size, 2048)
}

func TestALoadTableShowsTheFormatAndIsHandedTheValuesSort(t *testing.T) {
	t.Parallel()

	var asked webui.PagedQuery
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/l", View: webui.Table[blob]{
		Title: "Loaded",
		Loader: webui.PagedLoader[blob]{Load: func(_ context.Context, q webui.PagedQuery) (webui.PagedWindow[blob], webui.Loaded) {
			asked = q
			return webui.PagedWindow[blob]{Items: blobs(), Total: 3}, webui.Ready()
		}},
		Key:     func(b blob) string { return b.Name },
		Columns: []webui.Accessor[blob]{blobName, blobSize},
	}}}}.MustCompile("/admin")

	body := serve(h, http.MethodGet, "/admin/l?loaded.sort=size&loaded.desc=true").Body.String()
	assert.Contains(t, body, ">1 MiB<")
	assert.Equal(t, asked.Sort, "Size") // the column, and it is the source that orders it by the number
	assert.True(t, asked.Desc)
}

func TestAFormatThatPanicsIsLoggedAndTheValueIsShownAsItIs(t *testing.T) {
	t.Parallel()

	var logged bytes.Buffer
	broken := webui.Int[blob]{Label: "Size", Load: func(b blob) int { return b.Size }, Format: func(int) string { panic("boom") }}
	h := webui.App{Logger: slog.New(slog.NewTextHandler(&logged, nil)), Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/p", View: webui.Table[blob]{
		Title:   "Broken",
		Loader:  webui.RowsLoader[blob]{Load: func(context.Context) ([]blob, webui.Loaded) { return blobs(), webui.Ready() }},
		Key:     func(b blob) string { return b.Name },
		Columns: []webui.Accessor[blob]{blobName, broken},
	}}}}.MustCompile("/admin")

	rec := serve(h, http.MethodGet, "/admin/p")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), ">1048576<") // the number, plainly
	assert.Contains(t, logged.String(), "a Format panicked")
	assert.Contains(t, logged.String(), "boom")
}
