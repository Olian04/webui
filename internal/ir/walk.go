package ir

// Walk calls visit on n and then on every node under it, a parent before its children, in the
// order a page shows them. A tab that is not selected is still visited: this is about what a page
// declares, not what it is showing. A nil n visits nothing.
func Walk(n Node, visit func(Node)) {
	if n == nil {
		return
	}
	visit(n)
	switch n := n.(type) {
	case *Stack:
		for _, c := range n.Children {
			Walk(c, visit)
		}
	case *Split:
		for _, c := range n.Children {
			Walk(c, visit)
		}
	case *Tabs:
		for _, t := range n.Tabs {
			Walk(t.Body, visit)
		}
	}
}

// Markdowns is every markdown leaf under n, in the order the page shows them.
func Markdowns(n Node) []*Markdown {
	var out []*Markdown
	Walk(n, func(n Node) {
		if m, ok := n.(*Markdown); ok {
			out = append(out, m)
		}
	})
	return out
}

// Itemses is every list of items under n, in the order the page shows them.
func Itemses(n Node) []*Items {
	var out []*Items
	Walk(n, func(n Node) {
		if i, ok := n.(*Items); ok {
			out = append(out, i)
		}
	})
	return out
}

// Editors is every form under n that is a code editor, in the order the page shows them.
func Editors(n Node) []*Form {
	var out []*Form
	Walk(n, func(n Node) {
		if f, ok := n.(*Form); ok && f.Editor != nil {
			out = append(out, f)
		}
	})
	return out
}
