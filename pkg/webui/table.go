package webui

import (
	"context"
	"fmt"

	"github.com/Olian04/webui/internal/ir"
)

// Query is what the address asks of a table: a window and an order. Sort is the
// Key (or the Label, when the accessor has no Key) of the column the table is
// sorted by, or empty. It is always one of the table's columns.
type Query struct {
	Offset int
	Limit  int
	Sort   string
	Desc   bool

	// Filters are the column filters in force, by the column's Key (or Label).
	// A column with a fixed set of options — a Badge, whose options are the keys
	// of its Kinds — holds the options chosen, always among that set. Any other
	// column holds the one text typed, trimmed and never empty. Numeric columns
	// (Int, Float and Slider) are not here but in Ranges. Load does the
	// filtering, as it does the sorting.
	Filters map[string][]string

	// Ranges are the bounds on the numeric columns, by the same Key (or Label).
	// A column filters by a minimum and a maximum, either of which may be left
	// out, so a range is an inequality, not text. A bound is always a finite
	// number; an unreadable one never arrives.
	Ranges map[string]Range
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
// Every column header is a sort link. A table keeps its sort, and its page when
// it pages (PageSize > 0), in the address under its ID: "devices.offset",
// "devices.sort", "devices.desc". The library owns those parameters; the page's
// argument struct never sees them, and Load receives the result as a Query and
// does the sorting. ID defaults to "table"; it must be unique within the page,
// so set it when a page has more than one table. The panels of one Tabs are
// never visible together, so they may share an ID.
//
// Actions render as a button per row; BulkActions render in the selection bar
// above a table that then has a checkbox column. Either needs Key, because a
// request names rows by identity, never by position.
type Table[M any] struct {
	Title       string
	Desc        string
	ID          string
	PageSize    int
	Load        func(ctx context.Context, q Query) (Rows[M], error)
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
	if t.Load == nil {
		v.add("a Table has no Load", "Set Load to func(ctx, Query) (Rows[M], error).")
	}
	validateAccessors(v, t.Columns, accessorSite{where: "Table.Columns"}, map[string]bool{})

	// Every column is sortable, by its Key or its Label, so those must tell the
	// columns apart: Load receives one in Query.Sort and has to know which.
	keys := map[string]bool{}
	for _, c := range t.Columns {
		if k := columnKey[M](c); k != "" {
			if keys[k] {
				v.add(fmt.Sprintf("Table.Columns: two columns sort by %q", k), "Give one of them a distinct Key.")
			}
			keys[k] = true
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
}

func (t Table[M]) lowerBody(at ir.Addr, l *bodyLowerer) ir.Node {
	*l.nodes++
	out := &ir.Table{
		At: at, Title: t.Title, Desc: t.Desc, ID: effectiveID(t.ID, "table"), PageSize: t.PageSize,
		Columns: lowerAccessors(t.Columns, "c"),
		Load: func(ctx context.Context, q ir.Query) ([]any, int, error) {
			rows, err := t.Load(ctx, queryOf(q))
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
		},
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

// queryOf is the IR's Query as the user's Load receives it. The two differ only
// in the type of a Range, which the user sees as theirs.
func queryOf(q ir.Query) Query {
	out := Query{Offset: q.Offset, Limit: q.Limit, Sort: q.Sort, Desc: q.Desc, Filters: q.Filters}
	if q.Ranges != nil {
		out.Ranges = make(map[string]Range, len(q.Ranges))
		for k, r := range q.Ranges {
			out.Ranges[k] = Range(r)
		}
	}
	return out
}
