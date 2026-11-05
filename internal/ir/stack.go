package ir

// Stack arranges children vertically.
type Stack struct {
	At    Addr
	Nodes []Node
}

// Addr reports the node's position in the body tree.
func (s *Stack) Addr() Addr { return s.At }

func (s *Stack) Children() []Node { return s.Nodes }
