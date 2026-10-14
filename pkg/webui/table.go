package webui

import (
	"context"
	"fmt"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/tablequery"
)

// Query is what the address asks of a table: a window and an order. Sort is the
// Label of the column the table is sorted by, or empty. It is always one of the
// table's columns.
type Query struct {
	// Offset is how many rows to skip: the start of the page.
	Offset int

	// Limit is how many rows the page shows: the table's PageSize.
	Limit int

	// Sort is the Label of the column to sort by, or empty for the source's own
	// order.
	Sort string

	// Desc sorts descending.
	Desc bool

	// Filters are the column filters in force, by the column's Label.
	// A column with a fixed set of options, a Select, whose options are those it
	// lists, holds the options chosen, always among that set. Any other
	// column holds the one text typed, trimmed and never empty. Numeric columns
	// (Int, Float and Slider) and moments (Datetime and Timestamp) are not here but
	// in Ranges. Load does the filtering, as it does the sorting.
	Filters map[string][]string

	// Ranges are the bounds on the numeric columns and the moments, by the same
	// Label. A column filters by a minimum and a maximum, either of which may be left
	// out, so a range is an inequality, not text. A bound is always a finite number;
	// an unreadable one never arrives. The bounds of a Datetime or a Timestamp are
	// Unix seconds, whatever the column holds, and the Datetime's own value, an ISO
	// 8601 string, is for the source to read the same way.
	// Sort is by the same number, so moments order as moments and not as text.
	Ranges map[string]Range
}

// Range bounds a number, both ends inclusive. A nil end is unbounded.
type Range struct {
	// Min is the lower bound, or nil for none.
	Min *float64
	// Max is the upper bound, or nil for none.
	Max *float64
}

// Window is one window of a table's rows, as Load returns it. Total is the count across all pages;
// when it is smaller than Offset plus len(Items) it is taken as unknown, so a
// loader that does not count can leave it zero.
type Window[M any] struct {
	// Items are the rows of this window.
	Items []M

	// Total is how many rows match across all pages. Leave it zero when the source
	// does not count.
	Total int
}

// Table is a leaf that lists rows of a model M, one row to a line.
//
// Every column header sorts the table and has a filter beside it. Where the rows
// come from is Rows, Load or Feed, one of the three. Rows suits a set small enough to
// list in full. Load and Feed suit a set too large for that, which the source pages:
// Load when it pages by offset and can sort and filter on request, and Feed when it
// pages by a cursor, which continues one sequence and so cannot be sorted or filtered
// by the library.
//
// A table keeps its sort, filters and page in the address, so a copied link shows
// the same view. Each of those parameters starts with the table's name, which is
// its Title in lower case with dashes: a table titled "Devices" uses
// "devices.sort", "devices.desc", "devices.offset" and so on, and a table with no
// Title is called "table". When two tables on one page would have the same name, the
// later one gets a number, as in "devices-2". The panels of a [Tabs] are never shown
// together, so tables in different panels may share a name, and then share their
// state. The library owns these parameters; the page's argument struct never sees
// them.
type Table[M any] struct {
	// Title is the panel's heading.
	Title string

	// Desc is a description, shown in a popover from an information icon beside
	// the title.
	Desc string

	// PageSize is how many rows a page shows, or 25 when it is left zero. A table
	// always pages, because one that listed every row would grow without bound.
	PageSize int

	// Rows returns every row of the table, and the library does the rest: it
	// filters, sorts and pages them by the columns' own values, comparing numbers
	// as numbers and text as text.
	//
	// Use Rows when the rows are all at hand or cheap to list in full, such as a
	// slice in memory or a small query. It is all that most tables need.
	//
	// If Rows, Load or Feed returns an error the panel says "Could not load", the cause is
	// logged and the rest of the page stands. The error's text is not shown to the
	// visitor, since it may name things they should not see. Return no rows, not an
	// error, for a table that is empty.
	//
	// Rows is called on every request that needs the table's rows: each page load,
	// sort, filter, page change, row action, and search. The library keeps nothing
	// between requests, so if listing is costly, cache inside Rows, or use Load.
	// Set exactly one of Rows, Load and Feed.
	Rows func(ctx context.Context) ([]M, error)

	// Load returns one page of rows, and does the filtering, sorting and paging
	// itself. It receives the window, the sort and the filters as a [Query], and
	// returns a [Window] with the rows and, if it can count them, the total.
	//
	// Use Load when the source can do that work better than the library, or is too
	// large to list in full, such as a database table or a remote API. Set exactly one of
	// Rows, Load and Feed.
	Load func(ctx context.Context, q Query) (Window[M], error)

	// Feed is for a source that pages by a cursor, also called a continuation token,
	// such as an object store's listing or a feed of events. The rows come in the order
	// the source gives them, and each page continues where the last ended. Feed is
	// handed the cursor of the page asked for, "" for the first, and how many rows a
	// page holds, and returns them with the cursor of the page after, "" when there is
	// none.
	//
	// Use Feed when the source pages by cursor, and Load when it pages by offset and
	// can sort and filter on request. A cursor continues one sequence, so there is
	// nothing for the library to filter, and a feed table has no filters. Its columns
	// can be sorted, but only the page on screen: the library orders the rows Feed
	// returned, by the column's own value, and Feed is not told of it. The page after
	// continues the sequence and is ordered within itself, so a sort never moves a row
	// to another page, and the footer says that it is of this page. Its pager is First
	// page, Previous and Next, with "Showing rows 10032–10054" for a count, since a
	// cursor has no place in a total.
	//
	// The address holds the cursor, the number of the page's first row, and the pages
	// before it, which is all that Previous and the row numbers are made from: nothing
	// is kept on the server, and the source is handed only the cursor and the size.
	// So cursors must be short enough to be in an address, and Previous goes back as
	// far as the address can remember, which is further for short cursors; the oldest
	// pages are forgotten first, and First page is always there. Set exactly one of
	// Rows, Load and Feed.
	Feed func(ctx context.Context, after string, limit int) (rows []M, next string, err error)

	// Export puts two links in the table's footer, CSV and JSON, that download its rows.
	// The file holds every row the table's sort and filters keep (a feed has no filters, and
	// its sort is of a page, so a feed's file is in the source's own order), not only the page on
	// screen, and the columns it shows, with their Labels as the header: no row actions
	// and no row links. A table does not export unless it says so, since it is a way to take
	// all of the rows at once.
	//
	// Rows exports every row. A Load table is asked for them a batch at a time, from
	// Offset 0, until it returns an empty window or the Total is reached, and a Feed is
	// followed from its first page to its last, so a source that pages must not repeat rows
	// when it is asked for the next. An export is cut at 100,000 rows, and the response
	// says so in the header X-Webui-Export-Truncated, with the number of rows in the file; so does an
	// export that stopped because the source hands out the same batch again, as one that ignores Offset
	// does when it has no Total. It runs the page's Guard, as any
	// request for the page does.
	//
	// A CSV cell is the text the table shows, except that a number is written as a number,
	// a [Select] as the labels of its values joined by "; ", and an address as the address, and a text that a spreadsheet would run as a formula,
	// one that starts with =, +, -, @, a tab or a carriage return, is written with a quote
	// before it. JSON is a list with an object for each row, keyed by the Labels in the
	// columns' order, where a number is a number, a moment is an RFC 3339 time in UTC (or
	// null when there is none), a [Select] is the list of its values, and anything else is a
	// string.
	Export bool

	// Key identifies a row, such as by its ID. It is required when the table has
	// Actions or BulkActions, because a request names rows by identity, never by
	// position.
	Key func(M) string

	// RowClick makes each row clickable: a [Link] to another page, or an [Action]
	// for what a link cannot do.
	//
	// A Link is a real anchor, so middle-click, open-in-new-tab and the keyboard
	// work. It suits a row that always leads to the same kind of page. An Action is
	// handed the row and may decide where to go from it, with [Outcome.Then], such as
	// to a folder page or an object page, or change something. It is a POST, so it
	// cannot be opened in a new tab, and it needs Key. A row its Guard refuses does
	// nothing.
	RowClick RowClick[M]

	// Actions render as a button per row.
	Actions []Action[M]

	// BulkActions render in a selection bar above the table, which then has a
	// checkbox column. Their subject is the selected rows.
	BulkActions []Action[[]M]

	// Columns are the table's columns, in order. A column is known by its Label,
	// which the address and a [Query] use to name it, so labels must differ.
	Columns []Accessor[M]
}

// ASSERT: Table implements PageBody
var _ PageBody = Table[struct{}]{}

func (t Table[M]) validateBody(v *bodyValidator) {
	sources := 0
	for _, set := range []bool{t.Rows != nil, t.Load != nil, t.Feed != nil} {
		if set {
			sources++
		}
	}
	switch {
	case sources == 0:
		v.add("a Table has none of Rows, Load and Feed",
			"Set Rows to func(ctx) ([]M, error) to list every row and let the library filter, sort and page; Load to do that yourself; or Feed for a source paged by a cursor.")
	case sources > 1:
		v.add("a Table has more than one of Rows, Load and Feed",
			"Set one: Rows lets the library filter, sort and page, Load does it yourself, and Feed pages by a cursor, sorting only the page on screen and with no filters.")
	}
	validateAccessors(v, t.Columns, accessorSite{where: "Table.Columns"})

	// Every column is sortable and filterable, and is named for that in the
	// address by its label, so the labels must tell the columns apart.
	names := map[string]string{}
	for _, c := range t.Columns {
		label := labelOf[M](c)
		if label == "" {
			continue // Compile said so already, or it is a Group
		}
		name := args.Slug(label)
		switch other, taken := names[name]; {
		case name == "":
			v.add(fmt.Sprintf("Table.Columns: the label %q has no letters or digits", label),
				"A column is named in the address by its label; use one with a letter or digit in it.")
		case taken:
			v.add(fmt.Sprintf("Table.Columns: the labels %q and %q name the same column in the address", other, label),
				"Reword one of them; a column is named in the address by its label, in lower case.")
		default:
			names[name] = label
		}
	}
	if t.PageSize < 0 {
		v.add("Table.PageSize is negative", "Use a number of rows, or leave it zero for the default of 25.")
	}

	_, clickRuns := t.RowClick.(Action[M])
	if (len(t.Actions)+len(t.BulkActions) > 0 || clickRuns) && t.Key == nil {
		v.add("a Table declares actions but no Key", "Set Key to return a stable identity for a row, such as its ID.")
	}
	for i, a := range t.Actions {
		validateAction(v, fmt.Sprintf("Table.Actions[%d]", i), a, true)
	}
	for i, a := range t.BulkActions {
		validateAction(v, fmt.Sprintf("Table.BulkActions[%d]", i), a, true)
	}
	if t.RowClick != nil {
		t.RowClick.validateRow(v)
		if len(t.Columns) > 0 {
			if _, isLink := t.Columns[0].(URL[M]); isLink {
				v.add("a Table's first column is a URL, but its RowClick is there",
					"The row's own link or button covers the first column, and a link cannot hold a link: put the URL in another column.")
			}
		}
	}
}

// defaultPageSize is how many rows a page shows when the table does not say.
const defaultPageSize = 25

// pageSizeOf is the rows a page shows: what the table asked for, or the default.
func pageSizeOf(size int) int {
	if size == 0 {
		return defaultPageSize
	}
	return size
}

func (t Table[M]) lowerBody(at ir.Addr) ir.Node {
	columns := lowerAccessors(t.Columns, "c")
	labels := make(map[string]string, len(columns)) // the address's name for a column → its label
	for _, c := range columns {
		labels[c.Key] = c.Label
	}
	if t.Rows != nil {
		// The library holds every row, so a select's choices are the values the rows
		// have, whether or not its Kinds name them.
		for i := range columns {
			columns[i].OpenSet = columns[i].Display == ir.DisplaySelect
		}
	}
	out := &ir.Table{
		At: at, Title: t.Title, Desc: t.Desc, PageSize: pageSizeOf(t.PageSize), Feed: t.Feed != nil,
		Export: t.Export, InMemory: t.Rows != nil,
		Columns: columns,
		Load:    t.loader(columns, labels),
	}
	if t.Key != nil {
		out.Key = func(row any) string { return t.Key(row.(M)) }
	}
	if t.RowClick != nil {
		out.RowClick, out.RowAction = t.RowClick.lowerRow()
	}
	for _, a := range t.Actions {
		out.Actions = append(out.Actions, lowerAction(a))
	}
	for _, a := range t.BulkActions {
		out.Bulk = append(out.Bulk, lowerBulk(a))
	}
	return out
}

// queryOf is the IR's Query as the user's Load receives it. The IR names a column
// as the address does, and the user knows it by its label, so each is renamed;
// the two otherwise differ only in the type of a Range, which the user sees as
// theirs.
func queryOf(q ir.Query, labels map[string]string) Query {
	out := Query{Offset: q.Offset, Limit: q.Limit, Sort: labels[q.Sort], Desc: q.Desc}
	if q.Filters != nil {
		out.Filters = make(map[string][]string, len(q.Filters))
		for name, values := range q.Filters {
			out.Filters[labels[name]] = values
		}
	}
	if q.Ranges != nil {
		out.Ranges = make(map[string]Range, len(q.Ranges))
		for name, r := range q.Ranges {
			out.Ranges[labels[name]] = Range(r)
		}
	}
	return out
}

// loader is the table's rows as the IR asks for them, by a Query.
func (t Table[M]) loader(columns []ir.Field, labels map[string]string) func(context.Context, ir.Query) (ir.Window, error) {
	if t.Rows != nil {
		return func(ctx context.Context, q ir.Query) (ir.Window, error) {
			all, err := t.Rows(ctx)
			if err != nil {
				return ir.Window{}, err
			}
			items := anys(all)
			window, total := tablequery.Apply(items, columns, q)
			return ir.Window{Rows: window, Total: total, Options: tablequery.Options(items, columns)}, nil
		}
	}
	if t.Feed != nil {
		return func(ctx context.Context, q ir.Query) (ir.Window, error) {
			rows, next, err := t.Feed(ctx, q.After, q.Limit)
			if err != nil {
				return ir.Window{}, err
			}
			items := anys(rows)
			if q.Sort != "" {
				// The source's order is the sequence's, so only the page in hand is ordered: the
				// next page continues the sequence, and is ordered within itself.
				items, _ = tablequery.Apply(items, columns, ir.Query{Sort: q.Sort, Desc: q.Desc})
			}
			return ir.Window{Rows: items, Total: -1, Next: next}, nil
		}
	}
	return func(ctx context.Context, q ir.Query) (ir.Window, error) {
		rows, err := t.Load(ctx, queryOf(q, labels))
		if err != nil {
			return ir.Window{}, err
		}
		items := anys(rows.Items)
		total := rows.Total
		if total < q.Offset+len(items) {
			total = -1
		}
		return ir.Window{Rows: items, Total: total}, nil
	}
}
