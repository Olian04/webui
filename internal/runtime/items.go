package runtime

import (
	"context"
	"strconv"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// maxItems bounds how many a list of items can be asked to show. The address says how many, so a hand-edited one must not ask a source for a million.
const maxItems = 200

// limitOf is how many items the address asks a list to show: what it shows at first, or more when its Show more has been followed, and no more than maxItems.
func limitOf(n *ir.Items, raw map[string]string) int {
	limit := n.PageSize
	if v, err := strconv.Atoi(raw[args.ViewKey(n.ID, "limit")]); err == nil && v > limit {
		limit = v
	}
	return min(limit, max(n.PageSize, maxItems))
}

// items loads a list of items and builds its view. The source is asked for one more than is shown, which is how the leaf knows there is more without
// counting. A failed Load fails that panel, not the page.
func (p *Program) items(ctx context.Context, req *Request, page *ir.Page, n *ir.Items) templ.Component {
	kind := "list"
	if n.Cards {
		kind = "cards"
	}
	view := render.ItemsView{Node: n, Page: page}
	view.Path, view.Query = req.encode(page)
	limit := limitOf(n, req.Raw)
	rows, ok, refusal := loaded(p, ctx, page, kind, kind, n.At, func() ([]any, error) { return firstItems(ctx, n, limit+1) })
	if !ok {
		view.Failed, view.Refusal = true, refusal
		return p.render.Items(view)
	}
	view.More = len(rows) > limit && limit < maxItems
	if len(rows) > limit {
		rows = rows[:limit]
	}
	view.Rows, view.Limit = rows, limit
	view.Hrefs = p.controlHrefs(ctx, req, page, n.Click, rows)
	if n.Click.Posts() && n.Key != nil {
		view.Keys = make([]string, len(rows))
		for i, row := range rows {
			view.Keys[i] = n.Key(row)
		}
		view.ClickRefused = p.clickRefused(ctx, page, n.Click, "an item", rows)
	}
	return p.render.Items(view)
}

// firstItems is the first n items of a list, from its loader, in the order it gives them.
func firstItems(ctx context.Context, list *ir.Items, n int) ([]any, error) {
	window, err := list.Load(ctx, ir.Query{Limit: n})
	return window.Rows, err
}
