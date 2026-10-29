package runtime

import (
	"context"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// whenShown is whether a When shows its child, decided by what its Load reads. ok is false when the Load failed: it is logged, and the
// condition is not decided, which is not a condition that holds.
func (p *Program) whenShown(ctx context.Context, page *ir.Page, n *ir.When) (shown, ok bool) {
	model, ok, _ := loaded(p, ctx, page, "when", "a when", n.At, func() (any, error) { return n.Load(ctx) })
	if !ok {
		return false, false
	}
	return n.Show.Get(model) == "true", true
}

// gatesOf maps the id of each leaf and each Tabs that is under a When to the Whens above it, the outermost first. A leaf that is under none is not in it. The ids
// are positions, as leavesOf has them, so what is hidden is named by the same thing that names what is shown.
func gatesOf(root ir.Node) map[string][]*ir.When {
	out := map[string][]*ir.When{}
	var walk func(n ir.Node, above []*ir.When)
	walk = func(n ir.Node, above []*ir.When) {
		switch n := n.(type) {
		case nil:
		case *ir.Stack:
			for _, c := range n.Children {
				walk(c, above)
			}
		case *ir.Split:
			for _, c := range n.Children {
				walk(c, above)
			}
		case *ir.Columns:
			for _, c := range n.Children {
				walk(c, above)
			}
		case *ir.When:
			walk(n.Child, append(above[:len(above):len(above)], n))
		case *ir.Tabs:
			if len(above) > 0 {
				out[render.LeafID(n.Addr())] = above
			}
			for _, t := range n.Tabs {
				walk(t.Body, above)
			}
		default:
			if len(above) > 0 {
				out[render.LeafID(n.Addr())] = above
			}
		}
	}
	walk(root, nil)
	return out
}

// shown is whether every When in whens holds, asked in order and stopping at the first that does not, and a Load that fails counts as one that does not. It is what a
// request for a leaf under them is let through by: a leaf that is hidden is answered as one that is not there.
func (p *Program) shown(ctx context.Context, page *ir.Page, whens []*ir.When) bool {
	for _, w := range whens {
		if shown, ok := p.whenShown(ctx, page, w); !ok || !shown {
			return false
		}
	}
	return true
}

// reachable is the leaves that a request may name: all of them, less those that are under a When that does not hold. A page with no When has the map as it is.
func (p *Program) reachable(ctx context.Context, page *ir.Page, leaves map[string]ir.Node, gates map[string][]*ir.When) map[string]ir.Node {
	if len(gates) == 0 {
		return leaves
	}
	held := map[*ir.When]bool{} // each When is asked once, however many leaves are under it
	out := make(map[string]ir.Node, len(leaves))
	for id, leaf := range leaves {
		ok := true
		for _, w := range gates[id] {
			if _, asked := held[w]; !asked {
				shown, loaded := p.whenShown(ctx, page, w)
				held[w] = loaded && shown
			}
			if !held[w] {
				ok = false
				break
			}
		}
		if ok {
			out[id] = leaf
		}
	}
	return out
}
