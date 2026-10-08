package ir

// Tabs arranges children as labeled panels. The selected tab is view state in
// the address ("<ID>.tab"), so it survives a reload and can be linked to.
type Tabs struct {
	At   Addr
	ID   string
	Tabs []Tab
}

// Kind reports the node kind.
func (t *Tabs) Kind() NodeKind { return NodeTabs }

// Addr reports the node's position in the body tree.
func (t *Tabs) Addr() Addr { return t.At }

// Tab is one labeled panel.
type Tab struct {
	Key   string // the argument value that selects this tab
	Label string
	Body  Node
}
