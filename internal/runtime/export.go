package runtime

import (
	"bytes"
	"cmp"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"mime"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/convert"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

const (
	// exportBatch is how many rows an export asks a source for at a time. A source that
	// returns fewer is followed all the same: the next batch starts after what it returned.
	exportBatch = 500

	// maxExportRows bounds an export. It is a download of a table that is too large to list, so
	// there has to be an end to it; the footer of a table that may have more rows than this says so, beside its export links.
	maxExportRows = render.ExportCap

	// exportTimeout bounds the whole export, which is many requests to a source.
	exportTimeout = 60 * time.Second
)

// exportFormats are the files a table can be downloaded as, by the value of its export parameter.
var exportFormats = map[string]struct {
	ext, contentType string
	encode           func(columns []ir.Field, rows []any) []byte
}{
	"csv":  {".csv", "text/csv; charset=utf-8", exportCSV},
	"json": {".json", "application/json", exportJSON},
}

// exportOf is the table a request asks to download, and as what, if it does. A table that
// does not declare Export ignores the parameter, as any parameter for a leaf the page does
// not have is. A format that is not one is the request's mistake.
func exportOf(r *http.Request, leaves map[string]ir.Node) (n *ir.Table, format string, bad bool) {
	var tables []*ir.Table
	for _, leaf := range leaves {
		if t, ok := leaf.(*ir.Table); ok && t.Export && t.ID != "" {
			tables = append(tables, t)
		}
	}
	slices.SortFunc(tables, func(a, b *ir.Table) int { return strings.Compare(a.ID, b.ID) })
	query := r.URL.Query()
	for _, t := range tables {
		if value, asked := query[args.ViewKey(t.ID, "export")]; asked {
			_, known := exportFormats[value[0]]
			return t, value[0], !known
		}
	}
	return nil, "", false
}

// export answers a request to download a table: every row its sort and filters keep, as a file.
// The page's Guard has already run, so it is the same visitor who may see the page.
func (p *Program) export(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n *ir.Table, format string) {
	ctx, cancel := context.WithTimeout(r.Context(), exportTimeout)
	defer cancel()

	started := time.Now()
	n, err := resolve(p, page, n, func() (*ir.Table, error) { return n.Resolved(ctx) }) // a filter by a select is held to the options it has now, and a cell says what its value says
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("table options: %w", err))
		return
	}
	rows, truncated, err := p.exportRows(ctx, n, queryOf(n, req.Raw))
	p.m.Load(page.PathTemplate, "export", started, err)
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("table export: %w", err))
		return
	}
	if truncated {
		p.log.Warn("webui: a table export was cut", "page", page.PathTemplate, "table", n.ID, "rows", len(rows))
	}

	p.m.Export(page.PathTemplate, format)
	kind := exportFormats[format]
	h := w.Header()
	h.Set("Content-Type", kind.contentType)
	// The file is named for the table in the address, or "table".
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": cmp.Or(n.ID, "table") + kind.ext}))
	h.Set("Cache-Control", "no-store")
	_, _ = w.Write(kind.encode(n.Columns, rows)) //nolint:gosec // G705: rows written as CSV or JSON, with a download content type, never HTML.
}

// exportRows loads every row of a table that the view in q keeps, a batch at a time for a
// source that pages, up to maxExportRows. truncated says there were more.
func (p *Program) exportRows(ctx context.Context, n *ir.Table, q ir.Query) (rows []any, truncated bool, err error) {
	switch {
	case n.Feed:
		return exportFeed(ctx, n)
	case n.InMemory:
		// The library holds every row and does the paging, so a query with no limit is all of them.
		q.Offset, q.Limit = 0, 0
		w, err := n.Load(ctx, q)
		if err != nil {
			return nil, false, err //nolint:wrapcheck // the caller says what it was doing
		}
		if len(w.Rows) > maxExportRows {
			return w.Rows[:maxExportRows], true, nil
		}
		return w.Rows, false, nil
	}

	q.Offset, q.Limit = 0, exportBatch
	var previous []any
	for {
		w, err := n.Load(ctx, q)
		if err != nil {
			return nil, false, err //nolint:wrapcheck // the caller says what it was doing
		}
		if len(w.Rows) == 0 {
			return rows, false, nil
		}
		// A source that ignores Offset hands out its first batch again and again, and with no Total
		// nothing else says it has none left: an export would run to the cap with the same rows. A batch
		// that is the same as the one before it, row for row, is that. It is not a batch that merely
		// starts with an equal row, which any table whose rows are not unique has; and with a Total the
		// loop ends by it, so this is not asked. What was kept is flagged as possibly incomplete.
		if w.Total < 0 && reflect.DeepEqual(previous, w.Rows) {
			p.log.Warn("webui: a table export stopped at a batch that repeats the one before it: the source seems to ignore Offset", "table", n.ID, "rows", len(rows))
			return rows, true, nil
		}
		previous = w.Rows
		rows = append(rows, w.Rows...)
		q.Offset += len(w.Rows)
		done := w.Total >= 0 && q.Offset >= w.Total
		if len(rows) >= maxExportRows {
			return rows[:maxExportRows], len(rows) > maxExportRows || !done, nil
		}
		if done {
			return rows, false, nil
		}
	}
}

// exportFeed follows a feed from its first page to its last.
func exportFeed(ctx context.Context, n *ir.Table) (rows []any, truncated bool, err error) {
	seen := map[string]bool{}
	after := ""
	for {
		w, err := n.Load(ctx, ir.Query{Limit: exportBatch, After: after})
		if err != nil {
			return nil, false, err //nolint:wrapcheck // the caller says what it was doing
		}
		rows = append(rows, w.Rows...)
		if len(rows) >= maxExportRows {
			return rows[:maxExportRows], len(rows) > maxExportRows || w.Next != "", nil
		}
		// No next page is the end; so is a cursor that has been seen, which is a source that loops.
		if w.Next == "" || seen[w.Next] {
			return rows, false, nil
		}
		seen[w.Next] = true
		after = w.Next
	}
}

// isNumber is whether a column holds a number a spreadsheet should read as one: not a moment, whose
// Num is a count of seconds that is not what is shown.
func isNumber(f ir.Field) bool {
	return (f.Kind == ir.KindInt || f.Kind == ir.KindFloat) && f.Display != ir.DisplayTime
}

// exportCSV is the rows as CSV, with the columns' labels for a header. It starts with a byte order
// mark, which is what makes a spreadsheet read it as UTF-8 and not as the system's own encoding.
func exportCSV(columns []ir.Field, rows []any) []byte {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	records := [][]string{convert.Slice(columns, func(c ir.Field) string { return csvText(c.Label) })}
	for _, row := range rows {
		records = append(records, convert.Slice(columns, func(c ir.Field) string { return csvCell(c, row) }))
	}
	_ = csv.NewWriter(&buf).WriteAll(records) // a write to memory does not fail
	return buf.Bytes()
}

// csvCell is one value as a spreadsheet will read it. A number is itself, so a negative one is not
// mistaken for a formula; an address is the address and not the words that link to it; and any other
// text is safe to open.
func csvCell(f ir.Field, row any) string {
	if f.Display == ir.DisplayLink && f.Link != nil {
		if href := f.Link(row); href != "" {
			return csvText(href)
		}
	}
	if f.Display == ir.DisplaySelect {
		// What the cell says: the labels of its values, one after another. A comma would be read as
		// one more value of a list in a cell that is already quoted for it, so it is a semicolon.
		var labels []string
		for _, v := range f.Values(row) {
			labels = append(labels, f.LabelOf(v))
		}
		return csvText(strings.Join(labels, "; "))
	}
	text := f.Get(row)
	if isNumber(f) {
		return text
	}
	return csvText(text)
}

// csvText is text that a spreadsheet will not run. One that starts with a character that makes a
// formula (or, with a tab or a carriage return, one that hides it) is given a quote before it, which
// is what a spreadsheet shows as nothing and reads as "this is text".
func csvText(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// exportJSON is the rows as a list of objects keyed by the columns' labels, in their order.
func exportJSON(columns []ir.Field, rows []any) []byte {
	keys := convert.Slice(columns, func(c ir.Field) []byte { return mustJSON(c.Label) })
	var buf bytes.Buffer
	buf.WriteString("[")
	for i, row := range rows {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString("\n  {")
		for j, c := range columns {
			if j > 0 {
				buf.WriteString(", ")
			}
			buf.Write(keys[j])
			buf.WriteString(": ")
			buf.Write(jsonCell(c, row))
		}
		buf.WriteString("}")
	}
	if len(rows) > 0 {
		buf.WriteString("\n")
	}
	buf.WriteString("]\n")
	return buf.Bytes()
}

// jsonCell is one value as JSON: a moment as an RFC 3339 time in UTC, or null when there is none; a
// number as a number; a value that is not there, in a field that may have none, as null; a select as the list of its values; and anything else, an address included, as a
// string.
func jsonCell(f ir.Field, row any) []byte {
	switch {
	case f.Bool:
		// true, false, or null for neither, which is what the three of them are.
		switch f.Get(row) {
		case "true":
			return []byte("true")
		case "false":
			return []byte("false")
		}
		return []byte("null")
	case f.Nullable && f.Get(row) == "":
		return []byte("null")
	case f.Display == ir.DisplaySelect:
		// A list of the values it holds, which is what it is, and not their labels: a value is what
		// the model stores and what a program reads. A select that holds none is an empty list.
		values := f.Values(row)
		if values == nil {
			values = []string{}
		}
		return mustJSON(values)
	case f.Display == ir.DisplayTime && f.Num != nil:
		seconds := f.Num(row)
		if math.IsInf(seconds, 0) || math.IsNaN(seconds) {
			return []byte("null")
		}
		return mustJSON(time.Unix(int64(seconds), 0).UTC().Format(time.RFC3339))
	case isNumber(f):
		text := f.Get(row)
		if x, err := strconv.ParseFloat(text, 64); err == nil && !math.IsInf(x, 0) && !math.IsNaN(x) && json.Valid([]byte(text)) {
			return []byte(text)
		}
		return mustJSON(text)
	case f.Display == ir.DisplayLink && f.Link != nil:
		if href := f.Link(row); href != "" {
			return mustJSON(href)
		}
	}
	return mustJSON(f.Get(row))
}

// mustJSON is v as JSON. It is only called with strings and lists of them, which always encode.
func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
