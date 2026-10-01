package ir

// Stack arranges children vertically.
type Stack struct {
	At       Addr
	Children []Node
}

// Kind reports the node kind.
func (s *Stack) Kind() NodeKind { return NodeStack }

// Addr reports the node's position in the body tree.
func (s *Stack) Addr() Addr { return s.At }
