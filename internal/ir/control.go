package ir

// Control is a button or a link that belongs to a row of a table: the click on the row, or one of the controls in its trailing cell. It is one of three
// things, of which exactly one is set: an Action, which posts; a Link, which leads to a page of the app; or External, which leads to an address.
type Control struct {
	// Label and Role are of the button, where the control is drawn as one. A click on a row has no button, and ignores them.
	Label string
	Role  Role

	// Action runs on the row, as a POST to the page that names the row by Key.
	Action *Action

	// Link is a page of the app, built from the row.
	Link *Link

	// External is an address, built from the row, that has been checked: "" for a row that has none.
	External func(row any) string
}

// Posts is whether the control is a POST, which needs the rows to have an identity.
func (c *Control) Posts() bool { return c != nil && c.Action != nil }
