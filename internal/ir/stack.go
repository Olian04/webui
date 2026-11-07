package ir

// Stack arranges children vertically.
type Stack struct {
	At    Pos
	Nodes []Node
}

// Pos reports the node's position in the view tree.
func (s *Stack) Pos() Pos { return s.At }

func (s *Stack) Children() []Node { return s.Nodes }
