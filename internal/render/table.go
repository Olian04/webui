package render

import (
	"fmt"
	"strconv"
	"strings"

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
}

// columns resolves the header: alignment from the value kind, and the sort
// link for every column that declares a key, when the table has a Sort
// argument for it to write to.
func (r *Renderer) columns(v TableView) []columnView {
	out := make([]columnView, len(v.Node.Columns))
	for i, f := range v.Node.Columns {
		col := columnView{Field: f}
		if f.Kind == ir.KindInt || f.Kind == ir.KindInt64 || f.Kind == ir.KindFloat {
			col.Align = c.AlignEnd
		}
		if f.SortKey != "" {
			col.SortHref, col.SortDir = r.sortLink(v, f.SortKey)
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
	switch f.Tones[value] {
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
func (v TableView) interactive() bool { return len(v.Node.Actions)+len(v.Node.Bulk) > 0 }

func (v TableView) selectable() bool { return len(v.Node.Bulk) > 0 }

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

// BulkActionValue is the "_act" value of a bulk action button.
func BulkActionValue(action int) string { return fmt.Sprintf("bulk:%d", action) }

func (v TableView) gate(row, action int) string {
	if row < len(v.Gates) && action < len(v.Gates[row]) {
		return v.Gates[row][action]
	}
	return ""
}
