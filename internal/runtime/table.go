package runtime

import (
	"context"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// queryOf is what the table's view state in the address asks of it. A sort key
// the table did not declare is dropped, so a hand-edited address cannot make a
// loader sort by something it never offered.
func queryOf(n *ir.Table, raw map[string]string) ir.Query {
	if n.Feed {
		// A feed has a cursor, and no offset or filters. It may be sorted, but only the page that
		// the source handed out: the order of the rows in a window of the sequence.
		q := ir.Query{Limit: n.PageSize, Start: 1}
		q.Sort, q.Desc = sortOf(n, raw)
		if after := raw[args.ViewKey(n.ID, "after")]; after != "" && len(after) <= maxCursor {
			q.After = after
			// Past the first page, which row it starts at and the way back are what the
			// address says; without them they are not known.
			q.Start = 0
			if row, err := strconv.Atoi(raw[args.RowKey(n.ID)]); err == nil && row > 0 {
				q.Start = row
			}
			q.Back = args.DecodeTrail(n.ID, raw[args.BackKey(n.ID)])
		}
		return q
	}
	var q ir.Query
	q.Limit = n.PageSize
	if off, err := strconv.Atoi(raw[args.ViewKey(n.ID, "offset")]); err == nil && off > 0 {
		q.Offset = off
	}
	q.Filters = filtersOf(n, raw)
	q.Ranges = rangesOf(n, raw)
	q.Sort, q.Desc = sortOf(n, raw)
	return q
}

// sortOf is the column the address sorts the table by, and whether it is descending. A
// column the table does not have is no sort.
func sortOf(n *ir.Table, raw map[string]string) (key string, desc bool) {
	key = raw[args.ViewKey(n.ID, "sort")]
	if key == "" || !slices.ContainsFunc(n.Columns, func(col ir.Field) bool { return col.Key == key && !col.NoSort }) {
		return "", false
	}
	return key, raw[args.ViewKey(n.ID, "desc")] == "true"
}

// maxCursor bounds a feed's cursor in the address. It is whatever the source made it,
// and comes back from the address, which is the visitor's to edit, so it is kept
// short; a longer one is a first page, as is one the source does not know.
const maxCursor = 2048

// maxFilterChars bounds a typed filter: it is typed by a person.
const maxFilterChars = 200

// filtersOf reads the column filters in the address. A column with a fixed set
// of options keeps only the options it offers, so a hand-edited address cannot
// make a loader filter by a value nobody could choose; any other column keeps
// the text, trimmed. Empty filters are not there at all, so a table with none
// has a nil map.
func filtersOf(n *ir.Table, raw map[string]string) map[string][]string {
	var out map[string][]string
	for _, col := range n.Columns {
		if col.Key == "" || col.NoFilter || col.Ranged() {
			continue
		}
		value := raw[args.FilterKey(n.ID, col.Key)]
		if value == "" {
			continue
		}
		var kept []string
		switch {
		case col.OpenSet:
			// The values are whatever the rows hold, which the library filters, so any
			// value is safe to keep: one that no row has matches none.
			for v := range strings.SplitSeq(value, ir.ListSep) {
				if v != "" && !slices.Contains(kept, v) {
					kept = append(kept, v)
				}
			}
		case col.Options != nil:
			for v := range strings.SplitSeq(value, ir.ListSep) {
				if slices.Contains(col.Options, v) && !slices.Contains(kept, v) {
					kept = append(kept, v)
				}
			}
		default:
			if text := strings.TrimSpace(value); text != "" {
				if runes := []rune(text); len(runes) > maxFilterChars {
					text = string(runes[:maxFilterChars])
				}
				kept = []string{text}
			}
		}
		if len(kept) == 0 {
			continue
		}
		if out == nil {
			out = map[string][]string{}
		}
		out[col.Key] = kept
	}
	return out
}

// rangesOf reads the bounds on the numeric columns, which a table filters by
// range, not by text: "5" is not a way to ask for more than 5. A bound that is not a
// finite number is dropped, so a loader never compares against NaN or infinity.
func rangesOf(n *ir.Table, raw map[string]string) map[string]ir.Range {
	var out map[string]ir.Range
	for _, col := range n.Columns {
		if col.Key == "" || col.NoFilter || !col.Ranged() {
			continue
		}
		loKey, hiKey := args.RangeKeys(n.ID, col.Key)
		read := bound
		if col.Display == ir.DisplayTime {
			read = momentBound
		}
		r := ir.Range{Min: read(raw[loKey]), Max: read(raw[hiKey])}
		if r.Min == nil && r.Max == nil {
			continue
		}
		if out == nil {
			out = map[string]ir.Range{}
		}
		out[col.Key] = r
	}
	return out
}

// momentBound reads a bound on a moment, as the filter's date and time inputs write
// it, in UTC: "2026-10-09T11:27", with or without seconds, or a date alone. Unix
// seconds are taken as they are, so an address built by hand can carry them.
func momentBound(s string) *float64 {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			x := float64(t.Unix())
			return &x
		}
	}
	return bound(s)
}

func bound(s string) *float64 {
	x, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
		return nil
	}
	return &x
}

// table loads a table leaf. A failed Load fails that panel, not the page: the
// rest of the screen is still current, and says so.
func (p *Program) table(ctx context.Context, req *Request, page *ir.Page, n *ir.Table) templ.Component {
	// A select column that reads its options has them as they are for this request: its filter offers them, and
	// keeps only them from the address. A source that fails fails the panel.
	resolved, err := resolve(p, page, n, func() (*ir.Table, error) { return n.Resolved(ctx) })
	if err != nil {
		if ctx.Err() == nil {
			p.log.Error("webui: the options of a table could not be read", "page", page.PathTemplate, "leaf", render.LeafID(n.At), "err", err)
		}
		view := render.TableView{Node: n, Page: page, Failed: true}
		view.Path, view.Query = req.encode(page)
		return p.render.Table(view)
	}
	n = resolved
	view := render.TableView{Node: n, Page: page, Q: queryOf(n, req.Raw)}
	view.Path, view.Query = req.encode(page)

	window, ok, refusal := loaded(p, ctx, page, "table", "table", n.At, func() (ir.Window, error) { return n.Load(ctx, view.Q) })
	if !ok {
		view.Failed, view.Refusal = true, refusal
		return p.render.Table(view)
	}
	rows := window.Rows
	view.Rows, view.Total, view.Options = rows, window.Total, window.Options
	if len(window.Next) > maxCursor {
		p.log.Error("webui: a feed's cursor is too long for the address, so there is no next page", "page", page.PathTemplate, "length", len(window.Next))
	} else {
		view.Next = window.Next
	}
	view.Hrefs = p.rowHrefs(ctx, req, page, n, rows)
	view.Keys, view.Gates = p.rowKeysAndGates(ctx, page, n, rows)
	view.ClickRefused = p.clickRefused(ctx, page, n, rows)
	view.BulkGates = p.bulkGates(ctx, page, n, rows)
	return p.render.Table(view)
}

// rowHrefs resolves each row's destination through Open, so it is a real href
// with the mount prefix. A row whose link cannot be built has none, and is
// logged: it renders as an ordinary row rather than a dead link. So does a row
// whose destination page refuses the visitor, since a link they cannot follow is
// not offered, and that is asked of the page's Guard once for each row.
func (p *Program) rowHrefs(ctx context.Context, req *Request, page *ir.Page, n *ir.Table, rows []any) []string {
	hrefs := make([]string, len(rows))
	link := n.RowClick
	if link == nil {
		return hrefs
	}
	for i, row := range rows {
		href, err := p.open(link.Dest, link.Args(ctx, row), req.Address)
		if err != nil {
			p.log.Error("webui: row link failed", "page", page.PathTemplate, "dest", link.Dest, "err", err)
			continue
		}
		if p.mayFollow(ctx, page, link, href) {
			hrefs[i] = href
		}
	}
	return hrefs
}

// clickRefused is, for a table whose RowClick is an Action, whether this viewer may not click each row: the Action's
// Guard, run on the row, as it will be on the POST. A row it refuses is shown but does nothing, whichever
// error says so: it has no button to hide or to disable.
func (p *Program) clickRefused(ctx context.Context, page *ir.Page, n *ir.Table, rows []any) []bool {
	if n.RowAction == nil || n.RowAction.Guard == nil {
		return nil
	}
	refused := make([]bool, len(rows))
	for i, row := range rows {
		refused[i] = !p.gateOf(page, "the RowClick of a table", func() error { return n.RowAction.Guard(ctx, row) }).Open()
	}
	return refused
}

// rowKeysAndGates names each row and says what each of its actions is for this
// viewer. The same Guard that decides the button authorises the POST, so there is no
// second authorisation rule to keep in sync. The gates are by the position that an
// action was declared at, whether it is drawn or not, since that is what its button
// posts.
func (p *Program) rowKeysAndGates(ctx context.Context, page *ir.Page, n *ir.Table, rows []any) ([]string, [][]render.Gate) {
	if n.Key == nil {
		return nil, nil
	}
	keys := make([]string, len(rows))
	gates := make([][]render.Gate, len(rows))
	for i, row := range rows {
		keys[i] = n.Key(row)
		gates[i] = make([]render.Gate, len(n.Actions))
		for a, act := range n.Actions {
			if act.Guard != nil {
				gates[i][a] = p.gateOf(page, "the action "+strconv.Quote(act.Label), func() error { return act.Guard(ctx, row) })
			}
		}
	}
	return keys, gates
}

// bulkGates says what each bulk action is for this viewer. Its Guard takes a selection, which does not exist
// until the visitor has made one, so it is asked of each row of the window as a selection of that one row, and
// the action is drawn if any of them is accepted. If every one is refused it is disabled, with the message of the
// first that said so, when one did, and is otherwise not drawn. A table with no rows has nothing to select, and none is drawn.
func (p *Program) bulkGates(ctx context.Context, page *ir.Page, n *ir.Table, rows []any) []render.Gate {
	if len(n.Bulk) == 0 {
		return nil
	}
	gates := make([]render.Gate, len(n.Bulk))
	for b, act := range n.Bulk {
		gates[b] = render.Gate{Hidden: true}
		for _, row := range rows {
			var g render.Gate
			if act.Guard != nil {
				g = p.gateOf(page, "the bulk action "+strconv.Quote(act.Label), func() error { return act.Guard(ctx, []any{row}) })
			}
			if g.Open() {
				gates[b] = g
				break
			}
			if gates[b].Hidden && g.Drawn() {
				gates[b] = g // the first that is disabled, if no row is accepted
			}
		}
	}
	return gates
}
