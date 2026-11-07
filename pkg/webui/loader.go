package webui

import (
	"context"

	"github.com/Olian04/webui/internal/convert"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/tablequery"
)

// Loader is where the rows of a [Table], the items of a [List] or [Cards] come from, and how much of the work of showing them the library does: the three kinds are
// [RowsLoader], which hands over every row and lets the library filter, sort and page, [PagedLoader], which pages by offset and filters and sorts itself, and
// [FeedLoader], which pages by a cursor. A source is one of them, so a table that has none, or two, cannot be written, and each kind says how it is checked and how it is
// loaded, so nothing that takes a Loader looks at which one it was given.
//
// If a Loader's Load returns [Failed] the panel says "Could not load", the cause is logged and the rest of the page stands. The error's text is not shown to
// the visitor, since it may name things they should not see. Return no rows, not a failure, for a panel that is empty. A panel that shows a job that is still going
// returns [Loaded.RefreshAfter]. A list or cards asks for the first rows only, one more than it shows (to know there are more), and asks again for more when its Show more
// is followed; a loader that is asked for them by a Query or a cursor is given a window of the first rows, with no sort and no filter.
type Loader[M any] interface {
	// validateLoader checks the loader where it is, which at names.
	validateLoader(v *viewValidator, at string)

	// lowerLoader is the loader as the IR asks for rows, by a Query, for columns that the panel has (none, for a list), named for the address by labels.
	lowerLoader(columns []ir.Field, labels map[string]string) loading
}

// loading is a loader as the IR holds it: how to ask for a window of rows, and what kind of source it is.
type loading struct {
	load func(context.Context, ir.Query) (ir.Window, error)

	// feed says the rows come from a cursor, and inMemory that the library holds all of them.
	feed, inMemory bool
}

// RowsLoader returns every row, and the library does the rest: it filters, sorts and pages them by the columns' own values, comparing numbers as numbers and text
// as text.
//
// Use it when the rows are all at hand or cheap to list in full, such as a slice in memory or a small query. It is all that most tables need.
//
// Load is called on every request that needs the rows: each page load, sort, filter, page change, row action, and search. The library keeps nothing
// between requests, so if listing is costly, cache it in your app (see the package documentation on a table's sources, and Example_caching), or use a [PagedLoader].
type RowsLoader[M any] struct {
	// Load returns every row.
	Load func(ctx context.Context) ([]M, Loaded)
}

// PagedLoader returns one page of rows, and does the filtering, sorting and paging itself. Its Load receives the window, the sort and the filters as a [PagedQuery], and
// returns a [PagedWindow] with the rows and, if it can count them, the total.
//
// Use it when the source can do that work better than the library, or is too large to list in full, such as a database table or a remote API.
type PagedLoader[M any] struct {
	// Load returns the rows of the window that the query asks for, in the order it asks for, with the filters applied.
	Load func(ctx context.Context, q PagedQuery) (PagedWindow[M], Loaded)
}

// FeedLoader is for a source that pages by a cursor, also called a continuation token, such as an object store's listing or a feed of events. The rows come in the order
// the source gives them, and each page continues where the last ended. Its Load is handed the cursor of the page asked for, "" for the first, and how many rows a
// page holds, and returns them with the cursor of the page after, "" when there is none.
//
// Use it when the source pages by cursor, and a [PagedLoader] when it pages by offset and can sort and filter on request. A cursor continues one sequence, so there is
// nothing for the library to filter, and a feed table has no filters. Its columns can be sorted, but only the page on screen: the library orders the rows the loader
// returned, by the column's own value, and the loader is not told of it. The page after continues the sequence and is ordered within itself, so a sort never moves a row
// to another page, and the footer says that it is of this page. Its pager is First page, Previous and Next, with "Showing rows 10032–10054" for a count, since a
// cursor has no place in a total.
//
// The address holds the cursor, the number of the page's first row, and the pages before it, which is all that Previous and the row numbers are made from: nothing
// is kept on the server, and the source is handed only the cursor and the size. So cursors must be short enough to be in an address, and Previous goes back as
// far as the address can remember, which is further for short cursors; the oldest pages are forgotten first, and First page is always there.
type FeedLoader[M any] struct {
	// Load returns the rows of the page that starts at the cursor, at most limit of them, and the cursor of the page after.
	Load func(ctx context.Context, after string, limit int) (rows []M, next string, loaded Loaded)
}

// ASSERT: RowsLoader, PagedLoader and FeedLoader implement Loader
var (
	_ Loader[struct{}] = RowsLoader[struct{}]{}
	_ Loader[struct{}] = PagedLoader[struct{}]{}
	_ Loader[struct{}] = FeedLoader[struct{}]{}
)

func (l RowsLoader[M]) validateLoader(v *viewValidator, at string) {
	if l.Load == nil {
		v.add(at+" is a RowsLoader with no Load", "Set Load to func(ctx) ([]M, Loaded), returning every row.")
	}
}

func (l PagedLoader[M]) validateLoader(v *viewValidator, at string) {
	if l.Load == nil {
		v.add(at+" is a PagedLoader with no Load", "Set Load to func(ctx, PagedQuery) (PagedWindow[M], Loaded), returning the rows of the window asked for.")
	}
}

func (l FeedLoader[M]) validateLoader(v *viewValidator, at string) {
	if l.Load == nil {
		v.add(at+" is a FeedLoader with no Load", "Set Load to func(ctx, after string, limit int) (rows []M, next string, loaded Loaded), returning the page that starts at the cursor.")
	}
}

func (l RowsLoader[M]) lowerLoader(columns []ir.Field, _ map[string]string) loading {
	return loading{inMemory: true, load: func(ctx context.Context, q ir.Query) (ir.Window, error) {
		all, loaded := l.Load(ctx)
		if err := loaded.apply(ctx); err != nil {
			return ir.Window{}, err
		}
		items := convert.Any(all)
		window, total := tablequery.Apply(items, columns, q)
		return ir.Window{Rows: window, Total: total, Options: tablequery.Options(items, columns)}, nil
	}}
}

func (l PagedLoader[M]) lowerLoader(_ []ir.Field, labels map[string]string) loading {
	return loading{load: func(ctx context.Context, q ir.Query) (ir.Window, error) {
		rows, loaded := l.Load(ctx, queryOf(q, labels))
		if err := loaded.apply(ctx); err != nil {
			return ir.Window{}, err
		}
		items := convert.Any(rows.Items)
		total := rows.Total
		if total < q.Offset+len(items) {
			total = -1
		}
		return ir.Window{Rows: items, Total: total}, nil
	}}
}

func (l FeedLoader[M]) lowerLoader(columns []ir.Field, _ map[string]string) loading {
	return loading{feed: true, load: func(ctx context.Context, q ir.Query) (ir.Window, error) {
		rows, next, loaded := l.Load(ctx, q.After, q.Limit)
		if err := loaded.apply(ctx); err != nil {
			return ir.Window{}, err
		}
		items := convert.Any(rows)
		if q.Sort != "" {
			// The source's order is the sequence's, so only the page in hand is ordered: the
			// next page continues the sequence, and is ordered within itself.
			items, _ = tablequery.Apply(items, columns, ir.Query{Sort: q.Sort, Desc: q.Desc})
		}
		return ir.Window{Rows: items, Total: -1, Next: next}, nil
	}}
}

// PagedQuery is what the address asks of a table: a window and an order. Sort is the
// Label of the column the table is sorted by, or empty. It is always one of the
// table's columns.
type PagedQuery struct {
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
	// (Int, Float and Slider) and moments (Time and OptionalTime) are not here but
	// in Ranges. Load does the filtering, as it does the sorting.
	Filters map[string][]string

	// Ranges are the bounds on the numeric columns and the moments, by the same
	// Label. A column filters by a minimum and a maximum, either of which may be left
	// out, so a range is an inequality, not text. A bound is always a finite number;
	// an unreadable one never arrives. The bounds of a Time or an OptionalTime are
	// Unix seconds, which the source reads the same way whatever it holds the moment as.
	// Sort is by the same number, so moments order as moments and not as text.
	Ranges map[string]PagedRange
}

// PagedRange bounds a number, both ends inclusive. A nil end is unbounded.
type PagedRange struct {
	// Min is the lower bound, or nil for none.
	Min *float64
	// Max is the upper bound, or nil for none.
	Max *float64
}

// PagedWindow is one window of a table's rows, as Load returns it. Total is the count across all pages;
// when it is smaller than Offset plus len(Items) it is taken as unknown, so a
// loader that does not count can leave it zero.
type PagedWindow[M any] struct {
	// Items are the rows of this window.
	Items []M

	// Total is how many rows match across all pages. Leave it zero when the source
	// does not count.
	Total int
}

// queryOf is the IR's PagedQuery as the user's Load receives it. The IR names a column
// as the address does, and the user knows it by its label, so each is renamed;
// the two otherwise differ only in the type of a PagedRange, which the user sees as
// theirs.
func queryOf(q ir.Query, labels map[string]string) PagedQuery {
	out := PagedQuery{Offset: q.Offset, Limit: q.Limit, Sort: labels[q.Sort], Desc: q.Desc}
	if q.Filters != nil {
		out.Filters = make(map[string][]string, len(q.Filters))
		for name, values := range q.Filters {
			out.Filters[labels[name]] = values
		}
	}
	if q.Ranges != nil {
		out.Ranges = make(map[string]PagedRange, len(q.Ranges))
		for name, r := range q.Ranges {
			out.Ranges[labels[name]] = PagedRange(r)
		}
	}
	return out
}
