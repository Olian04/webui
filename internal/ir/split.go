package ir

// Split arranges children side by side.
type Split struct {
	At       Addr
	Children []Node
}

// Kind reports the node kind.
func (s *Split) Kind() NodeKind { return NodeSplit }

// Addr reports the node's position in the body tree.
func (s *Split) Addr() Addr { return s.At }
