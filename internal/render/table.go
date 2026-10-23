package render

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	c "github.com/Olian04/webui/internal/render/templates/components"
)

// LeafID is the address of a leaf as it appears in the document and in the
// header of a refresh request: "p" for a body that is itself a leaf, then one
// index per level, such as "p.0.1". It comes from position only.
// confirmID is the id of the dialog that a button of a leaf asks in: the leaf, the kind of place the button is in, and its place
// there. It is a position and never a key, so it holds nothing that a row says.
func confirmID(at ir.Addr, kind string, n int) string {
	return "confirm-" + LeafID(at) + "-" + kind + strconv.Itoa(n)
}

func LeafID(at ir.Addr) string {
	var b strings.Builder
	b.WriteString("p")
	for _, i := range at {
		b.WriteByte('.')
		b.WriteString(strconv.Itoa(i))
	}
	return b.String()
}

// TableView is a table with its data loaded. The runtime fills it; nothing
// here loads or decides.
type TableView struct {
	Node *ir.Table
	Page *ir.Page

	// Path and Query are the page's current arguments, encoded by name, so a
	// sort or pager link can change one and keep the rest.
	Path, Query map[string]string

	Rows  []any
	Total int
	Q     ir.Query // the window and order actually loaded

	// Hrefs is each row's destination, "" for a row that has none.
	Hrefs []string

	// Next is, for a feed, the cursor of the page after this one, "" for none.
	Next string

	// Options is, for each open-set column by Key, the values its rows hold.
	Options map[string][]string

	// ClickRefused is, for a table whose RowClick is an Action, whether each row
	// may not be clicked by this viewer.
	ClickRefused []bool

	// Keys is each row's identity, and Gates what each row's action is for this
	// viewer, by row then action, as the declared actions number them. Both are
	// empty for a table with no actions.
	Keys  []string
	Gates [][]Gate

	// BulkGates is what each bulk action is for this viewer, as the declared actions number them.
	BulkGates []Gate

	Failed bool // Load failed: the panel says so, the page stands

	// Refusal is what the Load said the visitor may be told, when it was a refusal and not a failure.
	Refusal *ir.Refusal
}

type columnView struct {
	Field    ir.Field
	Align    c.Align
	SortHref string
	SortDir  c.SortDir
	Filter   templ.Component
}

// columns resolves the header: alignment from the value kind, and the sort
// link for every column that declares a key, when the table has a Sort
// argument for it to write to.
func (r *Renderer) columns(v TableView) []columnView {
	out := make([]columnView, len(v.Node.Columns))
	for i, f := range v.Node.Columns {
		col := columnView{Field: f}
		if (f.Kind == ir.KindInt || f.Kind == ir.KindFloat) && f.Display != ir.DisplayTime {
			col.Align = c.AlignEnd
		}
		if f.Key != "" && !f.NoSort {
			col.SortHref, col.SortDir = r.sortLink(v, f.Key)
		}
		if f.Key != "" && !f.NoFilter && !v.Node.Feed { // a feed is one sequence, which there is nothing to filter
			col.Filter = r.filterMenu(v, f)
		}
		out[i] = col
	}
	return out
}

// sortLink is the address that sorts by key. Following it from an unsorted or
// differently sorted column sorts ascending; from the ascending column,
// descending (when the table has somewhere to record that). Either way the
// table returns to its first page, except a feed, which stays where it is: it
// orders the page on screen, and its cursor is the way back to it.
func (r *Renderer) sortLink(v TableView, key string) (string, c.SortDir) {
	id := v.Node.ID
	current := c.SortNone
	if v.Q.Sort == key {
		current = c.SortAsc
		if v.Q.Desc {
			current = c.SortDesc
		}
	}

	set, drop := map[string]string{args.ViewKey(id, "sort"): key}, []string{args.ViewKey(id, "offset")} // a feed has no offset
	if current == c.SortAsc {
		set[args.ViewKey(id, "desc")] = "true"
	} else {
		drop = append(drop, args.ViewKey(id, "desc"))
	}
	return r.tableHref(v, set, drop...), current
}

// tableHref is this table's address with some of its view state changed: the keys in drop are removed
// and those in set are written, and the rest, the page's arguments and every other table's state, stays.
func (r *Renderer) tableHref(v TableView, set map[string]string, drop ...string) string {
	query := maps.Clone(v.Query)
	if query == nil {
		query = map[string]string{}
	}
	for _, k := range drop {
		delete(query, k)
	}
	maps.Copy(query, set)
	return r.PageHref(v.Page, v.Path, query)
}

// emptyLinks are the ways on from a table that shows nothing: to its first page when it is past the end of what there is, and to the table without its filters when
// some are set, "" for each that does not apply. The empty state says why there is nothing, and these are what the visitor can do about it.
func (r *Renderer) emptyLinks(v TableView) (first, unfiltered string) {
	id := v.Node.ID
	pages := []string{args.ViewKey(id, "offset"), args.ViewKey(id, "after"), args.RowKey(id), args.BackKey(id)}
	if v.Q.Offset > 0 || v.Query[args.ViewKey(id, "after")] != "" {
		first = r.tableHref(v, nil, pages...)
	}
	if len(v.Q.Filters)+len(v.Q.Ranges) > 0 {
		// The filters are in the address under the column's name, which is not the key that the query holds them by, so they are found by what they begin with.
		lo, hi := args.RangeKeys(id, "")
		prefixes := []string{args.FilterKey(id, ""), lo, hi}
		drop := slices.Clone(pages)
		for key := range v.Query {
			if slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(key, prefix) }) {
				drop = append(drop, key)
			}
		}
		unfiltered = r.tableHref(v, nil, drop...)
	}
	return first, unfiltered
}

// pagerLinks are the previous and next addresses; "" means there is none.
func (r *Renderer) pagerLinks(v TableView) (prev, next string) {
	size := v.Node.PageSize
	offsetKey := args.ViewKey(v.Node.ID, "offset")
	at := func(offset int) string {
		if offset > 0 {
			return r.tableHref(v, map[string]string{offsetKey: strconv.Itoa(offset)})
		}
		return r.tableHref(v, nil, offsetKey)
	}
	if v.Q.Offset > 0 {
		prev = at(max(v.Q.Offset-size, 0))
	}
	more := len(v.Rows) >= size // total unknown: a full page might have a successor
	if v.Total >= 0 {
		more = v.Q.Offset+len(v.Rows) < v.Total
	}
	if more && len(v.Rows) > 0 {
		next = at(v.Q.Offset + size)
	}
	return prev, next
}

// feedLinks are the addresses of a feed's first page, of the one before this, and of
// the one after. The cursor is a parameter of the address, with the number of the
// page's first row and the pages before it, so the first page has none of them.
func (r *Renderer) feedLinks(v TableView) (first, prev, next string) {
	id := v.Node.ID
	after, row, back := args.ViewKey(id, "after"), args.RowKey(id), args.BackKey(id)
	// at is this table's address at a page, with the pages before it.
	at := func(m ir.Mark, trail []ir.Mark) string {
		set := map[string]string{}
		if m.After != "" {
			set[after] = m.After
			if m.Start > 0 {
				set[row] = strconv.Itoa(m.Start)
			}
			if len(trail) > 0 {
				set[back] = args.EncodeTrail(trail)
			}
		}
		return r.tableHref(v, set, after, row, back)
	}
	if v.Q.After != "" {
		first = at(ir.Mark{}, nil)
	}
	if n := len(v.Q.Back); n > 0 {
		prev = at(v.Q.Back[n-1], v.Q.Back[:n-1])
	}
	if v.Next != "" {
		start := 0
		if v.Q.Start > 0 {
			start = v.Q.Start + len(v.Rows)
		}
		next = at(ir.Mark{Start: start, After: v.Next}, args.PushMark(id, v.Q.Back, ir.Mark{Start: v.Q.Start, After: v.Q.After}))
	}
	return first, prev, next
}

func (r *Renderer) pager(v TableView) c.PagerProps {
	prev, next := r.pagerLinks(v)
	p := c.PagerProps{Offset: v.Q.Offset, Size: v.Node.PageSize, Total: v.Total, PrevHref: prev, NextHref: next, Export: r.exportLinks(v), ExportNote: exportNote(v)}
	if v.Total < 0 {
		p.Size = len(v.Rows) // the range ends at the last row there is
		if len(v.Rows) == 0 {
			p.Total = 0
		}
	}
	return p
}

// ExportCap is how many rows an export has at most: a download is built in memory, so there is an end to it.
const ExportCap = 100_000

// exportNote is what the footer says beside the export links when the download may not have every row: the table has more than
// ExportCap, or does not say how many it has. It is "" when the whole table fits.
func exportNote(v TableView) string {
	if v.Total >= 0 && v.Total <= ExportCap {
		return ""
	}
	return "cut at " + thousands(ExportCap) + " rows"
}

// thousands is n with a comma between each three digits: 100000 is "100,000".
func thousands(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// exportLinks are the links that download a table, for its footer: its sort and filters are kept,
// and where it is in its pages is not, since a download is every row. A table that does not
// export, or has failed, has none.
func (r *Renderer) exportLinks(v TableView) []c.ExportLink {
	if !v.Node.Export || v.Failed {
		return nil
	}
	id := v.Node.ID
	at := func(format string) string {
		return r.tableHref(v, map[string]string{args.ViewKey(id, "export"): format},
			args.ViewKey(id, "offset"), args.ViewKey(id, "after"), args.RowKey(id), args.BackKey(id))
	}
	return []c.ExportLink{{Label: "CSV", Format: "CSV", Href: at("csv")}, {Label: "JSON", Format: "JSON", Href: at("json")}}
}

// gauge is a slider's value as a bar between its Min and Max. A value outside
// the range is drawn at the nearer end: the text beside it still says the truth.
func gauge(f ir.Field, text string) c.GaugeProps {
	x, _ := strconv.ParseFloat(text, 64)
	return c.GaugeProps{Value: x - f.Min, Max: f.Max - f.Min, Text: text}
}

func (v TableView) label() string {
	if v.Node.Title != "" {
		return v.Node.Title
	}
	return "Table"
}

// interactive reports whether the table posts: a row action or a bulk action is drawn for
// this viewer, or a click on a row is an action. A table with none has no form, no checkbox column and
// no bar: absence is the configuration, and for a viewer whose Guards refuse every action, so it is for them.
func (v TableView) interactive() bool {
	return v.selectable() || v.hasRowActions() || v.Node.RowAction != nil
}

// clicks is whether row i runs the table's RowAction when clicked.
func (v TableView) clicks(i int) bool {
	return v.Node.RowAction != nil && i < len(v.Keys) && (i >= len(v.ClickRefused) || !v.ClickRefused[i])
}

// selectable is whether a bulk action is drawn, and so a column to select the rows with and a bar to act on them from.
func (v TableView) selectable() bool {
	return slices.ContainsFunc(v.BulkGates, Gate.Drawn)
}

// linkOf is the address a DisplayLink field has for a row, "" for any other field or
// a value that may not be a link.
func linkOf(f ir.Field, row any) string {
	if f.Link == nil {
		return ""
	}
	return f.Link(row)
}

// momentAttr is a moment as shown, "2026-10-09 11:27" or a date alone, as the
// datetime attribute of a <time> element reads it.
// It is empty for text that is not a moment, which is then shown as it is.
// momentOf is the machine-readable moment of a time field in a row, for its <time> element: the
// value's own text, which is what it holds, and not the text it is shown as, which a Format may
// have made something other than a moment. "" for any other field.
func momentOf(f ir.Field, row any) string {
	if f.Display != ir.DisplayTime || f.Get == nil {
		return ""
	}
	return momentAttr(f.Get(row))
}

func momentAttr(text string) string {
	if _, err := time.Parse("2006-01-02 15:04", text); err == nil {
		return text[:10] + "T" + text[11:] + ":00Z"
	}
	if _, err := time.Parse("2006-01-02", text); err == nil {
		return text
	}
	return ""
}

func (v TableView) rowClickValue(i int) string {
	if i < len(v.Keys) {
		return RowClickValue(v.Keys[i])
	}
	return ""
}

// hasRowActions is whether a row of this window has a row action drawn, and so a column to draw it in: it comes and goes
// with the rows that the window holds, as the Guards say.
func (v TableView) hasRowActions() bool {
	return slices.ContainsFunc(v.Gates, func(row []Gate) bool { return slices.ContainsFunc(row, Gate.Drawn) })
}

// width is the column count including the ones the table adds itself.
func (v TableView) width() int {
	n := len(v.Node.Columns)
	if v.selectable() {
		n++
	}
	if v.hasRowActions() {
		n++
	}
	return n
}

func variant(role ir.Role) c.Variant {
	switch role {
	case ir.RolePrimary:
		return c.VariantPrimary
	case ir.RoleDestructive:
		return c.VariantDanger
	case ir.RoleSecondary:
		return c.VariantSecondary
	}
	return c.VariantSecondary
}

// RowActionValue and BulkActionValue are what an action button submits as
// "_act". The runtime parses them back; they are defined once, here.
func RowActionValue(action int, key string) string { return fmt.Sprintf("row:%d:%s", action, key) }

// RowClickValue is the "_act" value of the button that covers a row whose click is
// an Action.
func RowClickValue(key string) string { return "click:0:" + key }

// BulkActionValue is the "_act" value of a bulk action button.
func BulkActionValue(action int) string { return fmt.Sprintf("bulk:%d", action) }

// gate is what row's action is for this viewer: not drawn, disabled, or as it is.
func (v TableView) gate(row, action int) Gate {
	if row < len(v.Gates) && action < len(v.Gates[row]) {
		return v.Gates[row][action]
	}
	return Gate{}
}

// bulkGate is what a bulk action is for this viewer.
func (v TableView) bulkGate(action int) Gate {
	if action < len(v.BulkGates) {
		return v.BulkGates[action]
	}
	return Gate{}
}

type carried struct{ Name, Value string }

// without is the query without the keys in skip, and without every table's offset: a changed filter
// returns every table to its first page, and the old offset may no longer exist.
func without(query map[string]string, skip ...string) map[string]string {
	out := make(map[string]string, len(query))
	for k, v := range query {
		if !slices.Contains(skip, k) && !strings.HasSuffix(k, args.ViewSep+"offset") {
			out[k] = v
		}
	}
	return out
}

// carry is the rest of the address as hidden form fields, so applying one
// column's filter keeps everything else: the page's arguments, every sort, every
// other filter. The keys in skip are the filter being replaced, and offsets are
// left out, because a changed filter returns every table to its first page: the
// old offset may no longer exist.
func carry(query map[string]string, skip ...string) []carried {
	rest := without(query, skip...)
	var out []carried
	for _, k := range slices.Sorted(maps.Keys(rest)) {
		for _, part := range strings.Split(rest[k], ir.ListSep) {
			if part != "" {
				out = append(out, carried{k, part})
			}
		}
	}
	return out
}

// filterView is one column's filter, resolved. A column with fixed Options is a
// multi-select, a numeric column a minimum and a maximum, any other a text box.
type filterView struct {
	Label   string
	Key     string // the address parameter of a text or multi-select filter
	Chosen  []string
	Options []string
	Field   ir.Field // for what an option says, when it says more than its value

	Numeric bool
	Moment  bool // the bounds are moments in UTC: date and time inputs
	Day     bool // ... or dates, for a column that holds days: date inputs
	MinKey  string
	MaxKey  string
	Min     string
	Max     string

	Action string
	Carry  []carried
	Clear  string // the address without this filter, "" when none is set
	On     bool
}

// name is what an option of the filter says: for a select, its label, and otherwise the value.
func (f filterView) name(option string) string { return f.Field.LabelOf(option) }

func (r *Renderer) filter(v TableView, f ir.Field) filterView {
	id := v.Node.ID
	fv := filterView{Label: f.Label, Field: f, Action: r.PageHref(v.Page, v.Path, nil)}
	gone := []string{} // the address parameters this filter owns
	if f.Ranged() {
		fv.Numeric = true
		fv.Moment = f.Display == ir.DisplayTime
		fv.Day = fv.Moment && f.DateOnly
		fv.MinKey, fv.MaxKey = args.RangeKeys(id, f.Key)
		gone = append(gone, fv.MinKey, fv.MaxKey)
		if rng, ok := v.Q.Ranges[f.Key]; ok {
			fv.Min, fv.Max = formatBound(rng.Min, fv.Moment, fv.Day), formatBound(rng.Max, fv.Moment, fv.Day)
		}
		fv.On = fv.Min != "" || fv.Max != ""
	} else {
		fv.Key = args.FilterKey(id, f.Key)
		gone = append(gone, fv.Key)
		fv.Chosen, fv.Options = v.Q.Filters[f.Key], f.Options
		if f.OpenSet {
			// The values the rows hold, and any chosen one the rows no longer do, so it
			// can still be unticked.
			fv.Options = []string{}
			fv.Options = append(fv.Options, v.Options[f.Key]...)
			for _, c := range fv.Chosen {
				if !slices.Contains(fv.Options, c) {
					fv.Options = append(fv.Options, c)
				}
			}
		}
		fv.On = len(fv.Chosen) > 0
	}
	fv.Carry = carry(v.Query, gone...)
	if fv.On {
		fv.Clear = r.PageHref(v.Page, v.Path, without(v.Query, gone...))
	}
	return fv
}

// formatBound is a bound as the filter's input holds it: a number, or for a moment
// the date and time input's "2026-10-09T11:27", in UTC.
func formatBound(x *float64, moment, day bool) string {
	if x == nil {
		return ""
	}
	if moment && day {
		return time.Unix(int64(*x), 0).UTC().Format("2006-01-02")
	}
	if moment {
		return time.Unix(int64(*x), 0).UTC().Format("2006-01-02T15:04")
	}
	return strconv.FormatFloat(*x, 'f', -1, 64)
}

// boundType is the input a moment's bounds are written in: a date for a column that holds days, and a date and time for the rest.
func (f filterView) boundType() string {
	if f.Day {
		return "date"
	}
	return "datetime-local"
}

func (f filterView) text() string {
	if len(f.Chosen) == 0 {
		return ""
	}
	return f.Chosen[0]
}

// columnWidths is the width of each column of a table, as a share of the table, from what the columns are and not from
// what is in them. A table sizes its columns by its rows, so two tables of the same columns, one above the other, would
// have them in different places as their rows differ. Each column is given the room that it needs, worked out from its
// declaration (its header, the choices of a select and whether it holds several, a bar, a moment), and the table is shared
// between them in those proportions, so a column with badges is wider than one with a number, at any width of the table.
func columnWidths(cols []columnView) []string {
	need := make([]float64, len(cols))
	total := 0.0
	for i, col := range cols {
		need[i] = columnNeed(col.Field)
		total += need[i]
	}
	out := make([]string, len(cols))
	for i := range cols {
		out[i] = strconv.FormatFloat(need[i]/total*100, 'f', 2, 64) + "%"
	}
	return out
}

// The room a column needs, in pixels of a 13px table: a letter of text, the padding of a cell, the room for the sort link
// and the filter beside a header, and what is in the cell that no row can say.
const (
	letterPx  = 7.2
	cellPx    = 20 // the padding of a cell
	headerPx  = 24 // the sort caret and the filter button, which a header with a key makes room for
	badgePx   = 34 // the dot and the padding of a badge
	badgeGap  = 6
	textPx    = 120 // a text or a link: about sixteen letters
	numberPx  = 90  // a number as it is formatted, such as "520.6 KiB"
	momentPx  = 110
	barPx     = 140 // the bar and the number beside it
	unknownPx = 110 // a select whose values are whatever its rows hold
)

// columnNeed is what one column needs: the more of what its header takes and what its cell takes.
func columnNeed(f ir.Field) float64 {
	header := float64(utf8.RuneCountInString(f.Label))*letterPx + cellPx
	if f.Key != "" && (!f.NoSort || !f.NoFilter) {
		header += headerPx
	}
	cell := float64(textPx)
	switch {
	case f.Display == ir.DisplaySelect:
		cell = unknownPx
		if len(f.Choices) > 0 {
			longest, second := 0, 0
			for _, c := range f.Choices {
				n := utf8.RuneCountInString(f.LabelOf(c.Value))
				if n > longest {
					longest, second = n, longest
				} else if n > second {
					second = n
				}
			}
			cell = float64(longest)*letterPx + badgePx + cellPx
			if f.Multi && len(f.Choices) > 1 {
				// room for two badges side by side, which is what a value of several looks like before it wraps
				cell += float64(second)*letterPx + badgePx + badgeGap
			}
		}
	case f.Display == ir.DisplaySlider:
		cell = barPx
	case f.Display == ir.DisplayTime:
		cell = momentPx
	case f.Kind == ir.KindInt || f.Kind == ir.KindFloat:
		cell = numberPx
	}
	return max(header, cell)
}

// inPlace marks a link that replaces the panel it is in, with the panel as the server draws it at the address that the link leads to: the sort and pager links of a table,
// and the ways on from an empty or failed one. Script follows such a link by asking the server for that panel alone, and any other link is a navigation, which is
// what a link is: the markup says which, and script does not guess from where the link leads.
var inPlace = templ.Attributes{"data-in-place": true}
