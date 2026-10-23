package webui_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type stock struct {
	Name  string
	Qty   int
	Price float64
	Made  int
	Page  string
}

var (
	stockName  = webui.String[stock]{Label: "Name", Load: func(s stock) string { return s.Name }}
	stockQty   = webui.Int[stock]{Label: "Qty", Load: func(s stock) int { return s.Qty }}
	stockPrice = webui.Float[stock]{Label: "Price", Precision: 2, Load: func(s stock) float64 { return s.Price }}
	stockMade  = webui.Timestamp[stock]{Label: "Made", Load: func(s stock) int { return s.Made }}
	stockPage  = webui.URL[stock]{Label: "Page", Text: "Open", Load: func(s stock) string { return s.Page }}
	stockCols  = []webui.Accessor[stock]{stockName, stockQty, stockPrice, stockMade, stockPage}
)

var stocks = []stock{
	{Name: "bolt", Qty: 40, Price: 0.25, Made: 1_700_000_000, Page: "https://example.com/bolt"},
	{Name: "nut", Qty: 5, Price: 0.1, Made: 0},
	{Name: "washer", Qty: -3, Price: 1.5, Made: 1_800_000_000, Page: "https://example.com/washer"},
}

func stockApp(t webui.Table[stock]) http.Handler {
	t.Title = "Items"
	t.Columns = stockCols
	t.Export = true
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", Body: t}}}.MustCompile("/admin")
}

func rowsTable(rows []stock) webui.Table[stock] {
	return webui.Table[stock]{Rows: func(context.Context) ([]stock, error) { return rows, nil }}
}

func records(t *testing.T, body string) [][]string {
	t.Helper()

	assert.True(t, strings.HasPrefix(body, "\xEF\xBB\xBF")) // read as UTF-8 by a spreadsheet
	out, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(body, "\xEF\xBB\xBF"))).ReadAll()
	assert.NoError(t, err)
	return out
}

func TestATableThatExportsDownloadsEveryRowItsViewKeepsAsCSV(t *testing.T) {
	t.Parallel()

	h := stockApp(rowsTable(stocks))
	rec := serve(h, http.MethodGet, "/admin/s?items.export=csv")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, rec.Header().Get("Content-Type"), "text/csv; charset=utf-8")
	assert.Equal(t, rec.Header().Get("Content-Disposition"), `attachment; filename=items.csv`)
	assert.Equal(t, rec.Header().Get("Cache-Control"), "no-store")
	assert.Equal(t, rec.Header().Get("X-Content-Type-Options"), "nosniff")
	assert.Equal(t, rec.Header().Get("X-Webui-Export-Truncated"), "")

	got := records(t, rec.Body.String())
	assert.DeepEqual(t, got[0], []string{"Name", "Qty", "Price", "Made", "Page"}) // the labels, in the columns' order
	assert.DeepEqual(t, got[1], []string{"bolt", "40", "0.25", "2023-11-14 22:13", "https://example.com/bolt"})
	assert.DeepEqual(t, got[2], []string{"nut", "5", "0.10", "", ""})
	assert.Equal(t, len(got), 4)
}

func TestAnExportHasTheTablesSortAndFiltersAndNotItsPage(t *testing.T) {
	t.Parallel()

	many := make([]stock, 0, 60)
	for i := range 60 {
		many = append(many, stock{Name: fmt.Sprintf("part%02d", i), Qty: i})
	}
	h := stockApp(rowsTable(many)) // 25 to a page, and the export is not a page

	all := records(t, serve(h, http.MethodGet, "/admin/s?items.export=csv").Body.String())
	assert.Equal(t, len(all), 61)

	// Sorted, filtered and on another page, the export is every row that the filter keeps, in the order.
	got := records(t, serve(h, http.MethodGet, "/admin/s?items.export=csv&items.sort=qty&items.desc=true&items.min.qty=10&items.max.qty=14&items.offset=25").Body.String())
	var names []string
	for _, r := range got[1:] {
		names = append(names, r[0])
	}
	assert.DeepEqual(t, names, []string{"part14", "part13", "part12", "part11", "part10"})

	byText := records(t, serve(h, http.MethodGet, "/admin/s?items.export=csv&items.filter.name=part5").Body.String())
	assert.Equal(t, len(byText), 11) // part5 and part50 to part59, and the header
}

func TestACSVCellThatASpreadsheetWouldRunAsAFormulaIsWrittenAsText(t *testing.T) {
	t.Parallel()

	h := stockApp(rowsTable([]stock{
		{Name: "=1+1", Qty: -3}, {Name: "+cmd", Qty: 1}, {Name: "-5 apples"}, {Name: "@home"}, {Name: "\tx"}, {Name: "\rx"},
		{Name: `a, "b"`}, {Name: "line\nbreak"}, {Name: "plain"},
	}))
	got := records(t, serve(h, http.MethodGet, "/admin/s?items.export=csv").Body.String())
	var names []string
	for _, r := range got[1:] {
		names = append(names, r[0])
	}
	assert.DeepEqual(t, names, []string{"'=1+1", "'+cmd", "'-5 apples", "'@home", "'\tx", "'\rx", `a, "b"`, "line\nbreak", "plain"})
	assert.Equal(t, got[1][1], "-3") // a number is a number: a negative one is not a formula
}

func TestAnExportOfJSONIsAListOfObjectsWithTypedValues(t *testing.T) {
	t.Parallel()

	h := stockApp(rowsTable(stocks))
	rec := serve(h, http.MethodGet, "/admin/s?items.export=json")
	assert.Equal(t, rec.Header().Get("Content-Type"), "application/json")
	assert.Equal(t, rec.Header().Get("Content-Disposition"), `attachment; filename=items.json`)

	var got []map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, len(got), 3)
	assert.DeepEqual(t, got[0], map[string]any{"Name": "bolt", "Qty": float64(40), "Price": 0.25, "Made": "2023-11-14T22:13:20Z", "Page": "https://example.com/bolt"})
	assert.True(t, got[1]["Made"] == nil) // no moment is null, not a time in 1970
	assert.True(t, got[1]["Page"] == "")  // no link is an empty string
	assert.True(t, got[2]["Qty"] == float64(-3))
	// The keys are in the columns' order, which a map does not keep.
	assert.True(t, strings.Index(rec.Body.String(), `"Name"`) < strings.Index(rec.Body.String(), `"Qty"`))
	assert.True(t, strings.Index(rec.Body.String(), `"Qty"`) < strings.Index(rec.Body.String(), `"Page"`))

	empty := stockApp(rowsTable(nil))
	assert.Equal(t, strings.TrimSpace(serve(empty, http.MethodGet, "/admin/s?items.export=json").Body.String()), "[]")
	assert.Equal(t, len(records(t, serve(empty, http.MethodGet, "/admin/s?items.export=csv").Body.String())), 1) // the header
}

func TestALoadTableIsAskedForEveryRowInBatchesFromTheStart(t *testing.T) {
	t.Parallel()

	var asked []webui.Query
	h := stockApp(webui.Table[stock]{Load: func(_ context.Context, q webui.Query) (webui.Window[stock], error) {
		asked = append(asked, q)
		var items []stock
		for i := q.Offset; i < min(q.Offset+q.Limit, 1200); i++ {
			items = append(items, stock{Name: fmt.Sprintf("part%04d", i)})
		}
		return webui.Window[stock]{Items: items, Total: 1200}, nil
	}})
	got := records(t, serve(h, http.MethodGet, "/admin/s?items.export=csv&items.offset=300&items.sort=name").Body.String())
	assert.Equal(t, len(got), 1201)
	assert.Equal(t, asked[0].Offset, 0)    // not where the page was
	assert.Equal(t, asked[0].Sort, "Name") // the table's own sort, in the labels the source knows
	assert.Equal(t, len(asked), 3)         // 500, 500 and 200
	assert.Equal(t, got[1200][0], "part1199")
}

func TestALoadTableThatIgnoresOffsetOrNeverEndsDoesNotLoopForEver(t *testing.T) {
	t.Parallel()

	calls := 0
	h := stockApp(webui.Table[stock]{Load: func(context.Context, webui.Query) (webui.Window[stock], error) {
		calls++
		return webui.Window[stock]{Items: []stock{{Name: "same"}, {Name: "rows"}}, Total: -1}, nil // always the first two
	}})
	rec := serve(h, http.MethodGet, "/admin/s?items.export=csv")
	assert.Equal(t, len(records(t, rec.Body.String())), 3)
	assert.Equal(t, calls, 2)
	assert.Equal(t, rec.Header().Get("X-Webui-Export-Truncated"), "") // the footer says it, in the page: there is no header for it
}

func TestRowsThatAreNotUniqueAreNotTakenForASourceThatIgnoresOffset(t *testing.T) {
	t.Parallel()

	// Every batch starts with the same row, as any table whose rows repeat has, and has no Total.
	h := stockApp(webui.Table[stock]{Load: func(_ context.Context, q webui.Query) (webui.Window[stock], error) {
		var items []stock
		for i := q.Offset; i < min(q.Offset+q.Limit, 1100); i++ {
			name := fmt.Sprintf("part%04d", i)
			if i%q.Limit == 0 {
				name = "same first row"
			}
			items = append(items, stock{Name: name})
		}
		return webui.Window[stock]{Items: items, Total: -1}, nil
	}})
	rec := serve(h, http.MethodGet, "/admin/s?items.export=csv")
	assert.Equal(t, len(records(t, rec.Body.String())), 1101) // all of them
	assert.Equal(t, rec.Header().Get("X-Webui-Export-Truncated"), "")
}

func TestAnExportIsCutAtTheLimitAndSaysSo(t *testing.T) {
	t.Parallel()

	h := stockApp(webui.Table[stock]{Load: func(_ context.Context, q webui.Query) (webui.Window[stock], error) {
		items := make([]stock, q.Limit)
		for i := range items {
			items[i] = stock{Name: fmt.Sprintf("part%d", q.Offset+i)}
		}
		return webui.Window[stock]{Items: items, Total: 250_000}, nil
	}})
	rec := serve(h, http.MethodGet, "/admin/s?items.export=csv")
	assert.Equal(t, rec.Header().Get("X-Webui-Export-Truncated"), "")
	assert.Equal(t, len(records(t, rec.Body.String())), 100_001)
}

// A download that may not have every row says so in the footer, beside the links that download it: when the table has more rows than an export
// holds, and when it does not say how many it has. One that fits does not.
func TestTheFooterSaysWhenAnExportMayBeCut(t *testing.T) {
	t.Parallel()

	note := func(total int) bool {
		h := stockApp(webui.Table[stock]{Load: func(context.Context, webui.Query) (webui.Window[stock], error) {
			return webui.Window[stock]{Items: []stock{{Name: "bolt"}}, Total: total}, nil
		}})
		body := serve(h, http.MethodGet, "/admin/s").Body.String()
		assert.Contains(t, body, "Download the rows as CSV")
		return strings.Contains(body, `<span class="pager-export-note">cut at 100,000 rows</span>`)
	}
	assert.True(t, note(250_000))
	assert.True(t, note(-1))
	assert.False(t, note(100_000))
	assert.False(t, note(1))
}

func TestAFeedIsFollowedFromItsFirstPageToItsLast(t *testing.T) {
	t.Parallel()

	var cursors []string
	h := stockApp(webui.Table[stock]{Feed: func(_ context.Context, after string, _ int) ([]stock, string, error) {
		cursors = append(cursors, after)
		switch after {
		case "":
			return []stock{{Name: "a"}, {Name: "b"}}, "c1", nil
		case "c1":
			return []stock{{Name: "c"}}, "c2", nil
		}
		return []stock{{Name: "d"}}, "", nil
	}})
	// Wherever the address has the feed, the export is the whole feed.
	got := records(t, serve(h, http.MethodGet, "/admin/s?items.export=csv&items.after=c2").Body.String())
	assert.Equal(t, len(got), 5)
	assert.DeepEqual(t, cursors, []string{"", "c1", "c2"})

	loop := stockApp(webui.Table[stock]{Feed: func(context.Context, string, int) ([]stock, string, error) {
		return []stock{{Name: "x"}}, "again", nil // a cursor that comes back
	}})
	assert.Equal(t, len(records(t, serve(loop, http.MethodGet, "/admin/s?items.export=csv").Body.String())), 3)
}

func TestATableThatDoesNotExportIgnoresTheParameter(t *testing.T) {
	t.Parallel()

	table := rowsTable(stocks)
	table.Title, table.Columns = "Items", stockCols
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", Body: table}}}.MustCompile("/admin")
	rec := serve(h, http.MethodGet, "/admin/s?items.export=csv")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.False(t, strings.Contains(rec.Body.String(), "pager-export")) // and offers no link
}

func TestAnExportOfAFormatThatIsNotOneIsABadRequest(t *testing.T) {
	t.Parallel()

	h := stockApp(rowsTable(stocks))
	for _, format := range []string{"xml", "CSV", "", "csv,json", "../x"} {
		assert.Equal(t, serve(h, http.MethodGet, "/admin/s?items.export="+format).Code, http.StatusBadRequest)
	}
}

func TestAnExportRunsThePagesGuardAndLoadsNothingForAVisitorItRefuses(t *testing.T) {
	t.Parallel()

	loaded := false
	table := webui.Table[stock]{Title: "Items", Columns: stockCols, Export: true, Rows: func(context.Context) ([]stock, error) {
		loaded = true
		return stocks, nil
	}}
	page := webui.Page[webui.NoArgs]{Path: "/s", Body: table, Guard: func(context.Context, webui.NoArgs) error { return errors.New("members only") }}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	rec := serve(h, http.MethodGet, "/admin/s?items.export=csv")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.False(t, loaded)
	assert.False(t, strings.Contains(rec.Body.String(), "bolt"))
}

func TestAFailingExportSaysSoAndNotWhy(t *testing.T) {
	t.Parallel()

	h := stockApp(webui.Table[stock]{Rows: func(context.Context) ([]stock, error) { return nil, errors.New("db secret-host is down") }})
	rec := serve(h, http.MethodGet, "/admin/s?items.export=csv")
	assert.Equal(t, rec.Code, http.StatusInternalServerError)
	assert.False(t, strings.Contains(rec.Body.String(), "secret-host"))
	assert.False(t, strings.Contains(rec.Header().Get("Content-Disposition"), "attachment"))
}

func TestOnlyTheTableThatWasAskedForIsExported(t *testing.T) {
	t.Parallel()

	a := rowsTable(stocks)
	a.Title, a.Columns, a.Export = "First", stockCols, true
	b := rowsTable([]stock{{Name: "only"}})
	b.Title, b.Columns, b.Export = "Second", []webui.Accessor[stock]{stockName}, true
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", Body: webui.Stack{a, b}}}}.MustCompile("/admin")
	got := records(t, serve(h, http.MethodGet, "/admin/s?second.export=csv").Body.String())
	assert.DeepEqual(t, got, [][]string{{"Name"}, {"only"}})
	assert.Equal(t, serve(h, http.MethodGet, "/admin/s?first.export=csv").Header().Get("Content-Disposition"), `attachment; filename=first.csv`)
}

func TestTheFooterOffersBothFilesAsPlainLinksThatKeepTheViewAndNotThePage(t *testing.T) {
	t.Parallel()

	many := make([]stock, 0, 60)
	for i := range 60 {
		many = append(many, stock{Name: fmt.Sprintf("part%02d", i), Qty: i})
	}
	h := stockApp(rowsTable(many))
	body := serve(h, http.MethodGet, "/admin/s?items.sort=qty&items.desc=true&items.offset=25").Body.String()
	assert.Contains(t, body, `<div class="pager-export">`)
	assert.Contains(t, body, `href="/admin/s?items.desc=true&amp;items.export=csv&amp;items.sort=qty"`)
	assert.Contains(t, body, `href="/admin/s?items.desc=true&amp;items.export=json&amp;items.sort=qty"`)
	assert.Contains(t, body, `download=""`) // left to the browser, not fetched into the panel
	assert.Contains(t, body, `aria-label="Download the rows as CSV"`)
	assert.False(t, strings.Contains(body, `items.export=csv&amp;items.offset`)) // every row, not the page
}

func TestAFeedsFooterHasTheLinksToo(t *testing.T) {
	t.Parallel()

	h := stockApp(webui.Table[stock]{Feed: func(context.Context, string, int) ([]stock, string, error) {
		return []stock{{Name: "a"}}, "next", nil
	}})
	body := serve(h, http.MethodGet, "/admin/s?items.after=next&items.row=2").Body.String()
	assert.Contains(t, body, `href="/admin/s?items.export=csv"`) // from its first page, with no cursor
}

func TestAFailedTableOffersNoExport(t *testing.T) {
	t.Parallel()

	h := stockApp(webui.Table[stock]{Rows: func(context.Context) ([]stock, error) { return nil, errors.New("down") }})
	body := serve(h, http.MethodGet, "/admin/s").Body.String()
	assert.Contains(t, body, "Could not load")
	assert.False(t, strings.Contains(body, "pager-export"))
}

func TestAnExportIsCompressedWhenTheBrowserAccepts(t *testing.T) {
	t.Parallel()

	h := stockApp(rowsTable(stocks))
	req := httptest.NewRequest(http.MethodGet, "/admin/s?items.export=csv", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, rec.Header().Get("Content-Encoding"), "gzip")
	assert.Contains(t, unzip(t, rec.Body.Bytes()), "bolt")
}

type ticket struct {
	Name   string
	Status []string
	Labels []string
}

var (
	ticketName   = webui.String[ticket]{Label: "Name", Load: func(t ticket) string { return t.Name }}
	ticketStatus = webui.Select[ticket]{
		Label: "Status", Load: func(t ticket) []string { return t.Status },
		Options: []webui.Option{{Value: "open", Label: "Open"}, {Value: "done", Label: "Done"}},
	}
	ticketLabels = webui.Select[ticket]{
		Label: "Labels", Load: func(t ticket) []string { return t.Labels }, Multi: true,
		Options: []webui.Option{{Value: "ui", Label: "UI"}, {Value: "bug"}},
	}
)

func ticketApp() http.Handler {
	table := webui.Table[ticket]{
		Title: "Tickets", Export: true, Columns: []webui.Accessor[ticket]{ticketName, ticketStatus, ticketLabels},
		Rows: func(context.Context) ([]ticket, error) {
			return []ticket{
				{Name: "a", Status: []string{"open"}, Labels: []string{"ui", "bug"}},
				{Name: "b", Status: []string{"done"}},
				{Name: "c", Status: []string{"gone"}, Labels: []string{"=cmd"}}, // a value that is no option
			}, nil
		},
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", Body: table}}}.MustCompile("/admin")
}

func TestASelectInACSVIsTheLabelsOfItsValuesAndNeverTheInternalSeparator(t *testing.T) {
	t.Parallel()

	rec := serve(ticketApp(), http.MethodGet, "/admin/t?tickets.export=csv")
	assert.False(t, strings.Contains(rec.Body.String(), "\x1f"))
	got := records(t, rec.Body.String())
	assert.DeepEqual(t, got[1], []string{"a", "Open", "UI; bug"})
	assert.DeepEqual(t, got[2], []string{"b", "Done", ""})
	assert.DeepEqual(t, got[3], []string{"c", "gone", "'=cmd"}) // an unknown value says itself, and a formula is text
}

func TestASelectInJSONIsTheListOfItsValues(t *testing.T) {
	t.Parallel()

	rec := serve(ticketApp(), http.MethodGet, "/admin/t?tickets.export=json")
	assert.False(t, strings.Contains(rec.Body.String(), "\x1f"))
	var got []map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.DeepEqual(t, got[0]["Status"], any([]any{"open"}))
	assert.DeepEqual(t, got[0]["Labels"], any([]any{"ui", "bug"})) // the values, not the labels "UI"
	assert.DeepEqual(t, got[1]["Labels"], any([]any{}))            // none is an empty list, not null or ""
}
