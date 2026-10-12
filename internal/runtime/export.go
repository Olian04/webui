package runtime

import (
	"bytes"
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
	"github.com/Olian04/webui/internal/ir"
)

const (
	// exportBatch is how many rows an export asks a source for at a time. A source that
	// returns fewer is followed all the same: the next batch starts after what it returned.
	exportBatch = 500

	// maxExportRows bounds an export. It is a download of a table that is too large to list, so
	// there has to be an end to it; the response says when it was cut.
	maxExportRows = 100_000

	// exportTimeout bounds the whole export, which is many requests to a source.
	exportTimeout = 60 * time.Second

	// exportTruncatedHeader is set on an export that may be incomplete, to the number of rows in it: it
	// was cut at maxExportRows, or stopped at a source that repeats itself.
	exportTruncatedHeader = "X-Webui-Export-Truncated"
)

// exportFormats are the files a table can be downloaded as, by the value of its export parameter.
var exportFormats = map[string]struct{ ext, contentType string }{
	"csv":  {".csv", "text/csv; charset=utf-8"},
	"json": {".json", "application/json"},
}

// exportOf is the table a request asks to download, and as what, if it does. A table that
// does not declare Export ignores the parameter, as any parameter for a leaf the page does
// not have is. A format that is not one is the request's mistake.
func exportOf(r *http.Request, leaves map[string]ir.Node) (n *ir.Table, format string, bad bool) {
	query := r.URL.Query()
	ids := make([]string, 0, len(leaves))
	for _, leaf := range leaves {
		if t, ok := leaf.(*ir.Table); ok && t.Export && t.ID != "" {
			ids = append(ids, t.ID)
		}
	}
	slices.Sort(ids)
	for _, id := range ids {
		value, asked := query[args.ViewKey(id, "export")]
		if !asked {
			continue
		}
		if _, known := exportFormats[value[0]]; !known {
			return nil, "", true
		}
		for _, leaf := range leaves {
			if t, ok := leaf.(*ir.Table); ok && t.ID == id {
				return t, value[0], false
			}
		}
	}
	return nil, "", false
}

// export answers a request to download a table: every row its sort and filters keep, as a file.
// The page's Guard has already run, so it is the same visitor who may see the page.
func (p *Program) export(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n *ir.Table, format string) {
	ctx, cancel := context.WithTimeout(r.Context(), exportTimeout)
	defer cancel()

	rows, truncated, err := p.exportRows(ctx, n, queryOf(n, req.Raw))
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("table export: %w", err))
		return
	}
	if truncated {
		p.log.Warn("webui: a table export was cut", "page", page.PathTemplate, "table", n.ID, "rows", maxExportRows)
	}

	var body []byte
	switch format {
	case "csv":
		body, err = exportCSV(n.Columns, rows)
	default:
		body, err = exportJSON(n.Columns, rows)
	}
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("table export: %w", err))
		return
	}

	kind := exportFormats[format]
	h := w.Header()
	h.Set("Content-Type", kind.contentType)
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": exportName(n) + kind.ext}))
	h.Set("Cache-Control", "no-store")
	if truncated {
		h.Set(exportTruncatedHeader, strconv.Itoa(len(rows)))
	}
	_, _ = w.Write(body) //nolint:gosec // G705: rows written as CSV or JSON, with a download content type, never HTML.
}

// exportName is the file's name without its extension: the table's name in the address, or
// "table".
func exportName(n *ir.Table) string {
	if n.ID != "" {
		return n.ID
	}
	return "table"
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
func exportCSV(columns []ir.Field, rows []any) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	header := make([]string, len(columns))
	for i, c := range columns {
		header[i] = csvText(c.Label)
	}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("csv: %w", err)
	}
	record := make([]string, len(columns))
	for _, row := range rows {
		for i, c := range columns {
			record[i] = csvCell(c, row)
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("csv: %w", err)
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error() //nolint:wrapcheck // a write to memory
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
func exportJSON(columns []ir.Field, rows []any) ([]byte, error) {
	keys := make([][]byte, len(columns))
	for i, c := range columns {
		k, err := json.Marshal(c.Label)
		if err != nil {
			return nil, fmt.Errorf("json: %w", err)
		}
		keys[i] = k
	}
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
			v, err := jsonCell(c, row)
			if err != nil {
				return nil, err
			}
			buf.Write(keys[j])
			buf.WriteString(": ")
			buf.Write(v)
		}
		buf.WriteString("}")
	}
	if len(rows) > 0 {
		buf.WriteString("\n")
	}
	buf.WriteString("]\n")
	return buf.Bytes(), nil
}

// jsonCell is one value as JSON: a moment as an RFC 3339 time in UTC, or null when there is none; a
// number as a number; a select as the list of its values; and anything else, an address included, as a
// string.
func jsonCell(f ir.Field, row any) ([]byte, error) {
	switch {
	case f.Display == ir.DisplaySelect:
		// A list of the values it holds, which is what it is, and not their labels: a value is what
		// the model stores and what a program reads. A select that holds none is an empty list.
		values := f.Values(row)
		if values == nil {
			values = []string{}
		}
		b, err := json.Marshal(values)
		if err != nil {
			return nil, fmt.Errorf("json: %w", err)
		}
		return b, nil
	case f.Display == ir.DisplayTime && f.Num != nil:
		seconds := f.Num(row)
		if math.IsInf(seconds, 0) || math.IsNaN(seconds) {
			return []byte("null"), nil
		}
		return marshalString(time.Unix(int64(seconds), 0).UTC().Format(time.RFC3339))
	case isNumber(f):
		text := f.Get(row)
		if x, err := strconv.ParseFloat(text, 64); err == nil && !math.IsInf(x, 0) && !math.IsNaN(x) && json.Valid([]byte(text)) {
			return []byte(text), nil
		}
		return marshalString(text)
	case f.Display == ir.DisplayLink && f.Link != nil:
		if href := f.Link(row); href != "" {
			return marshalString(href)
		}
	}
	return marshalString(f.Get(row))
}

func marshalString(s string) ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}
	return b, nil
}
