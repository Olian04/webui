package ir

// Tabs arranges children as labeled panels.
type Tabs struct {
	At   Addr
	Tabs []Tab
}

// Kind reports the node kind.
func (t *Tabs) Kind() NodeKind { return NodeTabs }

// Addr reports the node's position in the body tree.
func (t *Tabs) Addr() Addr { return t.At }

// Tab is one labeled panel.
type Tab struct {
	Label string
	Body  Node
}
