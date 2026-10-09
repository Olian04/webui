package ir

// Tables is every table under n, in the order the page shows them. A table in a
// tab that is not selected is still one of them: this is about what a page
// declares, not what it is showing.
func Tables(n Node) []*Table {
	var out []*Table
	var walk func(Node)
	walk = func(n Node) {
		switch n := n.(type) {
		case *Stack:
			for _, c := range n.Children {
				walk(c)
			}
		case *Split:
			for _, c := range n.Children {
				walk(c)
			}
		case *Tabs:
			for _, t := range n.Tabs {
				walk(t.Body)
			}
		case *Table:
			out = append(out, n)
		}
	}
	if n != nil {
		walk(n)
	}
	return out
}
