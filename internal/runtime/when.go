package runtime

import (
	"context"
	"sync"

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

// conditionCache is what each conditional node of a page decided, for one request, so that a condition is asked once
// however many times it is looked at: to say whether a tab is shown, and again to draw it. It is shared by the copies of the
// request that its panels are built with. A request without one (a refresh of a leaf, an export) asks each time.
type conditionCache struct {
	mu sync.Mutex
	m  map[ir.Node]*conditionResult
}

type conditionResult struct {
	once      sync.Once
	shown, ok bool
}

// conditionShown is whether the conditional node n shows its child, asked once per request when the request has a cache.
// ok is false when the condition could not be decided. A new kind of conditional node is decided in decide, and nowhere else.
func (req *Request) conditionShown(ctx context.Context, p *Program, page *ir.Page, n ir.Node) (shown, ok bool) {
	if req == nil || req.conditions == nil {
		return p.decide(ctx, page, n)
	}
	c := req.conditions
	c.mu.Lock()
	if c.m == nil {
		c.m = map[ir.Node]*conditionResult{}
	}
	r := c.m[n]
	if r == nil {
		r = &conditionResult{}
		c.m[n] = r
	}
	c.mu.Unlock()
	r.once.Do(func() { r.shown, r.ok = p.decide(ctx, page, n) })
	return r.shown, r.ok
}

// decide asks a conditional node whether it shows its child. A node that is not conditional always does.
func (p *Program) decide(ctx context.Context, page *ir.Page, n ir.Node) (shown, ok bool) {
	if w, isWhen := n.(*ir.When); isWhen {
		return p.whenShown(ctx, page, w)
	}
	return true, true
}

// gatesOf maps the id of each leaf and each Tabs that is under a When to the Whens above it, the outermost first. A leaf that is under none is not in it. The ids
// are positions, as leavesOf has them, so what is hidden is named by the same thing that names what is shown.
func gatesOf(root ir.Node) map[string][]*ir.When {
	out := map[string][]*ir.When{}
	var walk func(n ir.Node, above []*ir.When)
	walk = func(n ir.Node, above []*ir.When) {
		if n == nil {
			return
		}
		switch n := n.(type) {
		case *ir.Stack, *ir.Columns: // they only arrange: there is nothing of theirs to hide
		case *ir.When:
			above = append(above[:len(above):len(above)], n)
		default: // a leaf, or a Tabs, which is hidden as a whole
			if len(above) > 0 {
				out[render.LeafID(n.Addr())] = above
			}
		}
		for _, c := range n.Children() {
			walk(c, above)
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
