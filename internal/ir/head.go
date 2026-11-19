package ir

// Headed is what a panel node embeds to carry the controls in its head: the buttons and links, at the right of its title, that the app declared for the panel as a whole.
// They are a toolbar's items, minus the forms: an Action or a Link, with no subject.
type Headed struct {
	HeadActions []ToolbarItem
}

// Head reports the controls of the panel's head, in order.
func (h Headed) Head() []ToolbarItem { return h.HeadActions }

// HasHead is what asks a node for its head's controls: every panel node that embeds [Headed] answers.
type HasHead interface {
	Head() []ToolbarItem
}
