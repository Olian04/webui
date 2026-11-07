package ir

// Tabs arranges children as labeled panels. The selected tab is view state in
// the address ("<ID>.tab"), so it survives a reload and can be linked to.
type Tabs struct {
	At   Pos
	ID   string
	Tabs []Tab
}

// Pos reports the node's position in the view tree.
func (t *Tabs) Pos() Pos { return t.At }

func (t *Tabs) Children() []Node {
	out := make([]Node, len(t.Tabs))
	for i, tab := range t.Tabs {
		out[i] = tab.Child
	}
	return out
}

// Tab is one labeled child of Tabs.
type Tab struct {
	Key   string // the argument value that selects this tab
	Label string
	Child Node
}
