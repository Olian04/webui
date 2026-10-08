package runtime

import (
	"context"
	"strconv"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// queryOf is what the table's view state in the address asks of it. A sort key
// the table did not declare is dropped, so a hand-edited address cannot make a
// loader sort by something it never offered.
func queryOf(n *ir.Table, raw map[string]string) ir.Query {
	var q ir.Query
	if n.PageSize > 0 {
		q.Limit = n.PageSize
		if off, err := strconv.Atoi(raw[args.ViewKey(n.ID, "offset")]); err == nil && off > 0 {
			q.Offset = off
		}
	}
	if key := raw[args.ViewKey(n.ID, "sort")]; key != "" {
		for _, col := range n.Columns {
			if col.SortKey == key {
				q.Sort = key
				q.Desc = raw[args.ViewKey(n.ID, "desc")] == "true"
				break
			}
		}
	}
	return q
}

// table loads a table leaf. A failed Load fails that panel, not the page: the
// rest of the screen is still current, and says so.
func (p *Program) table(ctx context.Context, req *Request, page *ir.Page, n *ir.Table) templ.Component {
	view := render.TableView{Node: n, Page: page, Q: queryOf(n, req.Raw)}
	view.Path, view.Query = req.encode(page)

	rows, total, err := n.Load(ctx, view.Q)
	if err != nil {
		if ctx.Err() == nil {
			p.log.Error("webui: table load failed", "page", page.PathTemplate, "leaf", render.LeafID(n.At), "err", err)
		}
		view.Failed = true
		return p.render.Table(view)
	}
	view.Rows, view.Total = rows, total
	view.Hrefs = p.rowHrefs(ctx, page, n, rows)
	view.Keys, view.Gates = rowKeysAndGates(ctx, n, rows)
	return p.render.Table(view)
}

// rowHrefs resolves each row's destination through Open, so it is a real href
// with the mount prefix. A row whose link cannot be built has none, and is
// logged: it renders as an ordinary row rather than a dead link.
func (p *Program) rowHrefs(ctx context.Context, page *ir.Page, n *ir.Table, rows []any) []string {
	hrefs := make([]string, len(rows))
	link, ok := n.RowClick.(*ir.Link)
	if !ok {
		return hrefs
	}
	for i, row := range rows {
		href, err := p.Open(link.Dest, link.Args(ctx, row))
		if err != nil {
			p.log.Error("webui: row link failed", "page", page.PathTemplate, "dest", link.Dest, "err", err)
			continue
		}
		hrefs[i] = href
	}
	return hrefs
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
