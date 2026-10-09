package render

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	c "github.com/Olian04/webui/internal/render/templates/components"
)

// LeafID is the address of a leaf as it appears in the document and in the
// header of a refresh request: "p" for a body that is itself a leaf, then one
// index per level, such as "p.0.1". It comes from position only.
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

	// Options is, for each open-set column by Key, the values its rows hold.
	Options map[string][]string

	// ClickGates is, for a table whose RowClick is an Action, the reason each row
	// may not be clicked by this viewer, "" when it may.
	ClickGates []string

	// Keys is each row's identity, and Gates the reason each row's action is
	// refused to this viewer, by row then action ("" when allowed). Both are
	// empty for a table with no actions.
	Keys  []string
	Gates [][]string

	Failed bool // Load failed: the panel says so, the page stands
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
		if f.Key != "" {
			col.SortHref, col.SortDir = r.sortLink(v, f.Key)
			col.Filter = r.filterMenu(v, f)
		}
		out[i] = col
	}
	return out
}

// sortLink is the address that sorts by key. Following it from an unsorted or
// differently sorted column sorts ascending; from the ascending column,
// descending (when the table has somewhere to record that). Either way the
// table returns to its first page.
func (r *Renderer) sortLink(v TableView, key string) (string, c.SortDir) {
	id := v.Node.ID
	query := cloneQuery(v.Query)
	delete(query, args.ViewKey(id, "offset"))

	current := c.SortNone
	if v.Q.Sort == key {
		current = c.SortAsc
		if v.Q.Desc {
			current = c.SortDesc
		}
	}

	query[args.ViewKey(id, "sort")] = key
	if current == c.SortAsc {
		query[args.ViewKey(id, "desc")] = "true"
	} else {
		delete(query, args.ViewKey(id, "desc"))
	}
	return r.PageHref(v.Page, v.Path, query), current
}

// pagerLinks are the previous and next addresses; "" means there is none.
func (r *Renderer) pagerLinks(v TableView) (prev, next string) {
	size := v.Node.PageSize
	offsetKey := args.ViewKey(v.Node.ID, "offset")
	at := func(offset int) string {
		query := cloneQuery(v.Query)
		if offset > 0 {
			query[offsetKey] = strconv.Itoa(offset)
		} else {
			delete(query, offsetKey)
		}
		return r.PageHref(v.Page, v.Path, query)
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

func (r *Renderer) pager(v TableView) c.PagerProps {
	prev, next := r.pagerLinks(v)
	p := c.PagerProps{Offset: v.Q.Offset, Size: v.Node.PageSize, Total: v.Total, PrevHref: prev, NextHref: next}
	if v.Total < 0 {
		p.Size = len(v.Rows) // the range ends at the last row there is
		if len(v.Rows) == 0 {
			p.Total = 0
		}
	}
	return p
}

func cloneQuery(m map[string]string) map[string]string {
	out := make(map[string]string, len(m)+2)
	for k, v := range m {
		out[k] = v
	}
	return out
}

// gauge is a slider's value as a bar between its Min and Max. A value outside
// the range is drawn at the nearer end: the text beside it still says the truth.
func gauge(f ir.Field, text string) c.GaugeProps {
	x, _ := strconv.ParseFloat(text, 64)
	return c.GaugeProps{Value: x - f.Min, Max: f.Max - f.Min, Text: text}
}

func badgeTone(f ir.Field, value string) c.Tone {
	switch f.Kinds[value] {
	case ir.ToneOK:
		return c.ToneOK
	case ir.ToneWarning:
		return c.ToneWarning
	case ir.ToneCritical:
		return c.ToneCritical
	}
	return c.ToneNeutral
}

func (v TableView) label() string {
	if v.Node.Title != "" {
		return v.Node.Title
	}
	return "Table"
}

func (v TableView) panelStatus() c.Tone {
	if v.Failed {
		return c.ToneCritical
	}
	return c.ToneNeutral
}

// interactive reports whether the table posts: it has row actions or bulk
// actions. A table that declares neither has no form, no checkbox column and
// no bar — absence is the configuration.
func (v TableView) interactive() bool {
	return len(v.Node.Actions)+len(v.Node.Bulk) > 0 || v.Node.RowAction != nil
}

// clicks is whether row i runs the table's RowAction when clicked.
func (v TableView) clicks(i int) bool {
	return v.Node.RowAction != nil && i < len(v.Keys) && (i >= len(v.ClickGates) || v.ClickGates[i] == "")
}

func (v TableView) selectable() bool { return len(v.Node.Bulk) > 0 }

// momentAttr is a moment as shown, "2026-10-09 11:27" or a date alone, as the
// datetime attribute of a <time> element reads it.
// It is empty for text that is not a moment, which is then shown as it is.
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

func (v TableView) hasRowActions() bool { return len(v.Node.Actions) > 0 }

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

func (v TableView) gate(row, action int) string {
	if row < len(v.Gates) && action < len(v.Gates[row]) {
		return v.Gates[row][action]
	}
	return ""
}

type carried struct{ Name, Value string }

// carry is the rest of the address as hidden form fields, so applying one
// column's filter keeps everything else: the page's arguments, every sort, every
// other filter. The keys in skip are the filter being replaced, and offsets are
// left out, because a changed filter returns every table to its first page: the
// old offset may no longer exist.
func carry(query map[string]string, skip ...string) []carried {
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []carried
	for _, k := range keys {
		if slices.Contains(skip, k) || strings.HasSuffix(k, args.ViewSep+"offset") {
			continue
		}
		for _, part := range strings.Split(query[k], args.ListSep) {
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

	Numeric bool
	Moment  bool // the bounds are moments in UTC: date and time inputs
	MinKey  string
	MaxKey  string
	Min     string
	Max     string

	Action string
	Carry  []carried
	Clear  string // the address without this filter, "" when none is set
	On     bool
}

func (r *Renderer) filter(v TableView, f ir.Field) filterView {
	id := v.Node.ID
	fv := filterView{Label: f.Label, Action: r.PageHref(v.Page, v.Path, nil)}
	gone := []string{} // the address parameters this filter owns
	if f.Ranged() {
		fv.Numeric = true
		fv.Moment = f.Display == ir.DisplayTime
		fv.MinKey, fv.MaxKey = args.RangeKeys(id, f.Key)
		gone = append(gone, fv.MinKey, fv.MaxKey)
		if rng, ok := v.Q.Ranges[f.Key]; ok {
			fv.Min, fv.Max = formatBound(rng.Min, fv.Moment), formatBound(rng.Max, fv.Moment)
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
		rest := cloneQuery(v.Query)
		for _, k := range gone {
			delete(rest, k)
		}
		for k := range rest {
			if strings.HasSuffix(k, args.ViewSep+"offset") {
				delete(rest, k)
			}
		}
		fv.Clear = r.PageHref(v.Page, v.Path, rest)
	}
	return fv
}

// formatBound is a bound as the filter's input holds it: a number, or for a moment
// the date and time input's "2026-10-09T11:27", in UTC.
func formatBound(x *float64, moment bool) string {
	if x == nil {
		return ""
	}
	if moment {
		return time.Unix(int64(*x), 0).UTC().Format("2006-01-02T15:04")
	}
	return strconv.FormatFloat(*x, 'f', -1, 64)
}

func (f filterView) text() string {
	if len(f.Chosen) == 0 {
		return ""
	}
	return f.Chosen[0]
}
