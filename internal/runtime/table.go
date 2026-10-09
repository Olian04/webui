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
		// A feed has a cursor and nothing else to ask: no offset, sort or filters.
		q := ir.Query{Limit: n.PageSize, Start: 1}
		if after := raw[args.ViewKey(n.ID, "after")]; after != "" && len(after) <= maxCursor {
			q.After = after
			// Past the first page, which row it starts at and the way back are what the
			// address says; without them they are not known.
			q.Start = 0
			if row, err := strconv.Atoi(raw[args.RowKey(n.ID)]); err == nil && row > 0 {
				q.Start = row
			}
			q.Back = args.DecodeTrail(raw[args.BackKey(n.ID)])
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
	if key := raw[args.ViewKey(n.ID, "sort")]; key != "" {
		for _, col := range n.Columns {
			if col.Key == key {
				q.Sort = key
				q.Desc = raw[args.ViewKey(n.ID, "desc")] == "true"
				break
			}
		}
	}
	return q
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
		if col.Key == "" || numeric(col) {
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
			for _, v := range strings.Split(value, args.ListSep) {
				if v != "" && !slices.Contains(kept, v) {
					kept = append(kept, v)
				}
			}
		case col.Options != nil:
			for _, v := range strings.Split(value, args.ListSep) {
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

// numeric reports whether a column holds numbers, which a table filters by
// range, not by text: "5" is not a way to ask for more than 5.
func numeric(col ir.Field) bool { return col.Ranged() }

// rangesOf reads the bounds on the numeric columns. A bound that is not a
// finite number is dropped, so a loader never compares against NaN or infinity.
func rangesOf(n *ir.Table, raw map[string]string) map[string]ir.Range {
	var out map[string]ir.Range
	for _, col := range n.Columns {
		if col.Key == "" || !numeric(col) {
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
	view := render.TableView{Node: n, Page: page, Q: queryOf(n, req.Raw)}
	view.Path, view.Query = req.encode(page)

	window, err := n.Load(ctx, view.Q)
	if err != nil {
		if ctx.Err() == nil {
			p.log.Error("webui: table load failed", "page", page.PathTemplate, "leaf", render.LeafID(n.At), "err", err)
		}
		view.Failed = true
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
	view.Keys, view.Gates = rowKeysAndGates(ctx, n, rows)
	view.ClickGates = clickGates(ctx, n, rows)
	return p.render.Table(view)
}

// rowHrefs resolves each row's destination through Open, so it is a real href
// with the mount prefix. A row whose link cannot be built has none, and is
// logged: it renders as an ordinary row rather than a dead link.
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
		hrefs[i] = href
	}
	return hrefs
}

// clickGates is, for a table whose RowClick is an Action, why each row may not be
// clicked by this viewer: the Action's Guard, run on the row, as it will be on the
// POST. A row it refuses is shown but does nothing.
func clickGates(ctx context.Context, n *ir.Table, rows []any) []string {
	if n.RowAction == nil || n.RowAction.Guard == nil {
		return nil
	}
	gates := make([]string, len(rows))
	for i, row := range rows {
		if err := n.RowAction.Guard(ctx, row); err != nil {
			gates[i] = err.Error()
		}
	}
	return gates
}

// rowKeysAndGates names each row and says which of its actions this viewer may
// not use, and why. The same Guard that gates the button authorises the POST,
// so there is no second authorisation rule to keep in sync. Bulk actions are
// not gated here: their Guard takes the selection, which does not exist yet,
// so it runs at execution.
func rowKeysAndGates(ctx context.Context, n *ir.Table, rows []any) ([]string, [][]string) {
	if n.Key == nil {
		return nil, nil
	}
	keys := make([]string, len(rows))
	gates := make([][]string, len(rows))
	for i, row := range rows {
		keys[i] = n.Key(row)
		gates[i] = make([]string, len(n.Actions))
		for a, act := range n.Actions {
			if act.Guard == nil {
				continue
			}
			if err := act.Guard(ctx, row); err != nil {
				gates[i][a] = err.Error()
			}
		}
	}
	return keys, gates
}
