package ir

// Tabs arranges children as labeled panels. The selected tab is view state in
// the address ("<ID>.tab"), so it survives a reload and can be linked to.
type Tabs struct {
	At   Addr
	ID   string
	Tabs []Tab
}

// Addr reports the node's position in the body tree.
func (t *Tabs) Addr() Addr { return t.At }

func (t *Tabs) Children() []Node {
	out := make([]Node, len(t.Tabs))
	for i, tab := range t.Tabs {
		out[i] = tab.Body
	}
	return out
}

// Tab is one labeled panel.
type Tab struct {
	Key   string // the argument value that selects this tab
	Label string
	Body  Node
}
