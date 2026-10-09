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
	// A column with a fixed set of options — a Badge, whose options are the keys
	// of its Kinds — holds the options chosen, always among that set. Any other
	// column holds the one text typed, trimmed and never empty. Numeric columns
	// (Int, Float and Slider) are not here but in Ranges. Load does the
	// filtering, as it does the sorting.
	Filters map[string][]string

	// Ranges are the bounds on the numeric columns, by the same Label.
	// A column filters by a minimum and a maximum, either of which may be left
	// out, so a range is an inequality, not text. A bound is always a finite
	// number; an unreadable one never arrives.
	Ranges map[string]Range

	// Search is what the visitor typed in the global search, to find rows of this
	// table by any column, and is only ever set for that: a table has no search
	// box of its own. Match it however the source can, case-insensitively, against
	// whatever the columns show. It is empty for the table's own view. A table
	// with Rows never sees it, since the library searches the rows itself.
	Search string
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
// come from is Rows or Load, one of the two: Rows suits data that is easy to list
// in full, and Load suits data that is better paged by its source.
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
	// Rows is called on every request that needs the table's rows: each page load,
	// sort, filter, page change, row action, and search. The library keeps nothing
	// between requests, so if listing is costly, cache inside Rows, or use Load.
	// Set Rows or Load, not both.
	Rows func(ctx context.Context) ([]M, error)

	// Load returns one page of rows, and does the filtering, sorting and paging
	// itself. It receives the window, the sort and the filters as a [Query], and
	// returns a [Window] with the rows and, if it can count them, the total.
	//
	// Use Load when the source can do that work better than the library, or is too
	// large to list in full, such as a database table or a remote API. Set Rows or
	// Load, not both.
	Load func(ctx context.Context, q Query) (Window[M], error)

	// Search makes the table's rows findable from the search box in the top bar.
	// Each row is a result: its first column is the title, the other text columns
	// are the line beneath it, and it leads where RowClick leads, so a searchable
	// table needs a RowClick. With Rows the library searches every column itself;
	// with Load the typed text arrives in [Query.Search].
	//
	// The table's page is asked for with no arguments, and its Guard runs first. A
	// page with path arguments, such as "/bucket/{name}", has none to be asked with,
	// so its table is searched only while the visitor is on that page, with the
	// arguments in the address they are on: the search then covers what they are
	// looking at. A result is shown only if the page it leads to would let the
	// visitor in.
	Search bool

	// Key identifies a row, such as by its ID. It is required when the table has
	// Actions or BulkActions, because a request names rows by identity, never by
	// position.
	Key func(M) string

	// RowClick makes each row clickable: a [Link] to another page, or an [Action]
	// for what a link cannot do.
	//
	// A Link is a real anchor, so middle-click, open-in-new-tab and the keyboard
	// work, and the table can be searched. It suits a row that always leads to the
	// same kind of page. An Action is handed the row and may decide where to go from
	// it, with [Outcome.Then], such as to a folder page or an object page, or change
	// something. It is a POST, so it cannot be opened in a new tab, it needs Key, and
	// a table with one cannot be searched. A row its Guard refuses does nothing.
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

func (Table[M]) isPageBody() {}

// ASSERT: Table implements PageBody
var _ PageBody = Table[struct{}]{}

func (t Table[M]) validateBody(v *bodyValidator) {
	switch {
	case t.Rows == nil && t.Load == nil:
		v.add("a Table has neither Rows nor Load",
			"Set Rows to func(ctx) ([]M, error) to list every row and let the library filter, sort and page, or Load to do that yourself.")
	case t.Rows != nil && t.Load != nil:
		v.add("a Table has both Rows and Load", "Set one: Rows lets the library filter, sort and page, Load does it yourself.")
	}
	validateAccessors(v, t.Columns, accessorSite{where: "Table.Columns"}, map[string]bool{})

	// Every column is sortable and filterable, and is named for that in the
	// address by its label, so the labels must tell the columns apart.
	names := map[string]string{}
	for _, c := range t.Columns {
		label := accessorLabel[M](c)
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
		validateAction(v, fmt.Sprintf("Table.Actions[%d]", i), a.Label, a.Run != nil, true)
	}
	for i, a := range t.BulkActions {
		validateAction(v, fmt.Sprintf("Table.BulkActions[%d]", i), a.Label, a.Run != nil, true)
	}
	if t.RowClick != nil {
		t.RowClick.validateRow(v)
	}
	if t.Search {
		if t.RowClick == nil {
			v.add("a Table has Search but no RowClick", "A search result is a link to a row's page: set RowClick, or remove Search.")
		}
		if clickRuns {
			v.add("a Table has Search but its RowClick is an Action", "A search result is a link to a row's page: make RowClick a Link, or remove Search.")
		}
		if len(t.Columns) == 0 {
			v.add("a Table has Search but no Columns", "A search result is made from the columns: set Columns.")
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
	out := &ir.Table{
		At: at, Title: t.Title, Desc: t.Desc, PageSize: pageSizeOf(t.PageSize), Search: t.Search,
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
		out.Actions = append(out.Actions, lowerAction(a, a.Label))
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
	out := Query{Offset: q.Offset, Limit: q.Limit, Sort: labels[q.Sort], Desc: q.Desc, Search: q.Search}
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
func (t Table[M]) loader(columns []ir.Field, labels map[string]string) func(context.Context, ir.Query) ([]any, int, error) {
	if t.Rows != nil {
		return func(ctx context.Context, q ir.Query) ([]any, int, error) {
			all, err := t.Rows(ctx)
			if err != nil {
				return nil, 0, err
			}
			items := make([]any, len(all))
			for i, r := range all {
				items[i] = r
			}
			window, total := tablequery.Apply(items, columns, q)
			return window, total, nil
		}
	}
	return func(ctx context.Context, q ir.Query) ([]any, int, error) {
		rows, err := t.Load(ctx, queryOf(q, labels))
		if err != nil {
			return nil, 0, err
		}
		items := make([]any, len(rows.Items))
		for i, r := range rows.Items {
			items[i] = r
		}
		total := rows.Total
		if total < q.Offset+len(items) {
			total = -1
		}
		return items, total, nil
	}
}
