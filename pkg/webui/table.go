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
	Offset int
	Limit  int
	Sort   string
	Desc   bool

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
	Min, Max *float64
}

// Rows is one window of a table's rows. Total is the count across all pages;
// when it is smaller than Offset plus len(Items) it is taken as unknown, so a
// loader that does not count can leave it zero.
type Rows[M any] struct {
	Items []M
	Total int
}

// Table is a leaf that lists rows of M.
//
// Every column header is a sort link with a filter beside it. A table keeps its
// sort, its filters, and its page when it pages (PageSize > 0), in the address
// under its ID: "devices.offset", "devices.sort", "devices.desc". The library owns
// those parameters; the page's argument struct never sees them. ID defaults to
// "table"; it must be unique within the page, so set it when a page has more than
// one table. The panels of one Tabs are never visible together, so they may share
// an ID.
//
// Where the rows come from is one of two fields, and a Table has exactly one.
//
// Rows returns every row, and the library does the rest: it filters, sorts and
// pages them by the columns' own accessors, a number as a number and text as
// text. It is what a table over a slice, a cache or a small query wants, and it
// is all that most tables need.
//
// Load is for a source that pages itself, too large to list in full. It receives
// the window, the sort and the filters as a Query and returns one window of rows,
// with the total, doing all of that itself.
//
// Search makes the table's rows findable from the global search in the top bar.
// A row is a result: its first column is the title, the other columns are the line
// beneath it, and it leads where RowClick leads, so the table needs a RowClick. For
// a table with Rows the library searches every column of every row. A table with
// Load is handed the text in Query.Search and answers as it can. The table's page
// is asked with no arguments, so it may not have path arguments, and its Guard runs
// first; each result is shown only if the page it leads to would let the visitor in.
//
// Actions render as a button per row; BulkActions render in the selection bar
// above a table that then has a checkbox column. Either needs Key, because a
// request names rows by identity, never by position.
type Table[M any] struct {
	Title       string
	Desc        string
	ID          string
	PageSize    int
	Rows        func(ctx context.Context) ([]M, error)
	Load        func(ctx context.Context, q Query) (Rows[M], error)
	Search      bool
	Key         func(M) string
	RowClick    RowClick[M]
	Actions     []Action[M]
	BulkActions []Action[[]M]
	Columns     []Accessor[M]
}

func (Table[M]) isPageBody() {}

// ASSERT: Table implements PageBody
var _ PageBody = Table[struct{}]{}

func (t Table[M]) validateBody(v *bodyValidator) {
	v.nodes++
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
		v.add("Table.PageSize is negative", "Use 0 for a table that does not page.")
	}
	v.id("Table", t.ID) // every table keeps its sort in the address

	if (len(t.Actions)+len(t.BulkActions)) > 0 && t.Key == nil {
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
		if len(args.Placeholders(v.page)) > 0 {
			v.add("a Table on a page with path arguments cannot have Search",
				"Search runs without a page's arguments, so it cannot build one. Put Search on a table of a page without placeholders, such as the list, and link to this page with RowClick.")
		}
		if len(t.Columns) == 0 {
			v.add("a Table has Search but no Columns", "A search result is made from the columns: set Columns.")
		}
	}
}

func (t Table[M]) lowerBody(at ir.Addr, l *bodyLowerer) ir.Node {
	*l.nodes++
	columns := lowerAccessors(t.Columns, "c")
	labels := make(map[string]string, len(columns)) // the address's name for a column → its label
	for _, c := range columns {
		labels[c.Key] = c.Label
	}
	out := &ir.Table{
		At: at, Title: t.Title, Desc: t.Desc, ID: effectiveID(t.ID, "table"), PageSize: t.PageSize, Search: t.Search,
		Columns: columns,
		Load:    t.loader(columns, labels),
	}
	if t.Key != nil {
		out.Key = func(row any) string { return t.Key(row.(M)) }
	}
	if t.RowClick != nil {
		out.RowClick = t.RowClick.lowerRow(l)
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
