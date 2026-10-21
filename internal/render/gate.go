package render

// Gate is what an action's Guard made of its control for this visitor. The zero value is a control that is drawn as it is.
type Gate struct {
	// Hidden is a control that is not drawn: its Guard refused the visitor with an error that says nothing of how to draw it.
	Hidden bool

	// Reason is, for a control that is drawn, why it is disabled; "" for one that is not.
	Reason string
}

// Drawn is whether there is a control to draw, whether or not it can be used.
func (g Gate) Drawn() bool { return !g.Hidden }

// Open is whether the control can be used.
func (g Gate) Open() bool { return !g.Hidden && g.Reason == "" }
