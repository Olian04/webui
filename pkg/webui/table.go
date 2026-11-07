package webui

import (
	"fmt"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
)

// Table is a panel that lists rows of a model M, one row to a line.
//
// Every column header sorts the table and has a filter beside it, unless the column says NoSort or NoFilter, which
// leave it out of the sorting or the filters: its header has no link or no menu, and an address that asks for
// one is ignored, so [PagedQuery] never carries it. Where the rows
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

	// Loader is where the rows come from, and how much of the work the library does: a [RowsLoader] hands over every row and lets the library filter, sort and
	// page them, a [PagedLoader] pages by offset and filters and sorts itself, and a [FeedLoader] pages by a cursor, which continues one sequence and so cannot be sorted or filtered
	// by the library. A table has one, and exactly one.
	Loader Loader[M]

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
	// when it is asked for the next. An export is cut at 100,000 rows, and the footer of a table that may have more says so beside the export links
	// (when its Total is more than that, or is not known). An export also stops when the source hands out the same batch again, as one
	// that ignores Offset does when it has no Total, which is logged. It runs the page's Guard, as any
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

	// Key identifies a row, such as by its ID. It is required when the table has an
	// [Action] among its Actions or its RowClick, or BulkActions, because a request names
	// rows by identity, never by position. A table whose controls only lead somewhere, which
	// post nothing, needs none.
	Key func(M) string

	// RowClick makes each row clickable: a [Link] to another page, an [Href] to an address, or an [Action]
	// for what neither can do.
	//
	// A Link or an Href is a real anchor, so middle-click, open-in-new-tab and the keyboard
	// work (an Href opens in a tab of its own, as any link to another site does). It suits a row
	// that always leads to the same kind of place. An Action is handed the row and may decide
	// where to go from it, with [Outcome.Then], such as to a folder page or an object page, or
	// change something. It is a POST, so it cannot be opened in a new tab, and it needs Key. A
	// row its Guard refuses does nothing, and a row whose Link leads to a page that refuses the
	// visitor is a plain row, which asks that page's Guard once for each row. The Label, Role and
	// Confirm of a click on a row are not drawn, there being no button, and a Confirm is refused.
	RowClick Control[M]

	// Actions render as a control per row, in a column of its own: an [Action] is a button that
	// posts, and a [Link] or an [Href] is a button that leads somewhere, such as an "Edit" that
	// opens the row's page. Each needs a Label. A row for whom an action's Guard returns an
	// error has no button for it, and [Disabled] draws it disabled, with its message; a row
	// whose Link leads to a page that refuses the visitor has none for it. The column of controls is
	// there when a row of the page on screen has one drawn.
	Actions []Control[M]

	// BulkActions render in a selection bar above the table, which then has a
	// checkbox column. Their subject is the selected rows. When the table is drawn the
	// action's Guard is asked of each row as a selection of one, and the button is drawn if
	// it accepts any: a bulk Guard must accept a selection of one. With no bulk action
	// drawn, there is no bar and no checkbox column.
	BulkActions []Action[[]M]

	// Columns are the table's columns, in order. A column is known by its Label,
	// which the address and a [PagedQuery] use to name it, so labels must differ.
	Columns []Accessor[M]
}

// ASSERT: Table implements View
var _ View = Table[struct{}]{}

func (t Table[M]) validateView(v *viewValidator) {
	if t.Loader == nil {
		v.add("a Table has no Loader", "Set Loader to a RowsLoader, which lists every row and lets the library filter, sort and page; a PagedLoader, which does that itself; or a FeedLoader, for a source paged by a cursor.")
	} else {
		t.Loader.validateLoader(v, "Table.Loader")
	}
	validateAccessors(v, t.Columns, accessorSite{where: "Table.Columns", rowClick: t.RowClick != nil})

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

	posts := len(t.BulkActions) > 0 || (t.RowClick != nil && t.RowClick.posts())
	for i, a := range t.Actions {
		at := fmt.Sprintf("Table.Actions[%d]", i)
		if a == nil {
			v.add(at+" is nil", "Remove it or declare a control.")
			continue
		}
		posts = posts || a.posts()
		a.validateControl(v, at, slotButton)
	}
	if posts && t.Key == nil {
		v.add("a Table declares actions but no Key", "Set Key to return a stable identity for a row, such as its ID.")
	}
	for i, a := range t.BulkActions {
		validateAction(v, fmt.Sprintf("Table.BulkActions[%d]", i), a, true)
	}
	if t.RowClick != nil {
		t.RowClick.validateControl(v, "Table.RowClick", slotClick)
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

func (t Table[M]) lowerView(at ir.Pos) ir.Node {
	columns := lowerAccessors(t.Columns, "c")
	labels := make(map[string]string, len(columns)) // the address's name for a column → its label
	for _, c := range columns {
		labels[c.Key] = c.Label
	}
	loading := t.Loader.lowerLoader(columns, labels)
	if loading.inMemory {
		// The library holds every row, so a select's choices are the values the rows
		// have, whether or not its Kinds name them.
		for i := range columns {
			columns[i].OpenSet = columns[i].Display == ir.DisplaySelect
		}
	}
	out := &ir.Table{
		At: at, Title: t.Title, Desc: t.Desc, PageSize: pageSizeOf(t.PageSize), Feed: loading.feed,
		Export: t.Export, InMemory: loading.inMemory,
		Columns: columns,
		Load:    loading.load,
	}
	if t.Key != nil {
		out.Key = func(row any) string { return t.Key(row.(M)) }
	}
	if t.RowClick != nil {
		out.Click = t.RowClick.lowerControl()
	}
	for _, a := range t.Actions {
		if a != nil {
			out.Actions = append(out.Actions, a.lowerControl())
		}
	}
	for _, a := range t.BulkActions {
		out.Bulk = append(out.Bulk, lowerBulk(a))
	}
	return out
}
