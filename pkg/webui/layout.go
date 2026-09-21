package webui

// Stack lays out children vertically. Children may be any PageBody.
type Stack []PageBody

func (Stack) isPageBody() {}

// Split lays out children side by side. Children may be any PageBody.
type Split []PageBody

func (Split) isPageBody() {}

// Tab is one panel in Tabs.
type Tab struct {
	Label string
	Body  PageBody
}

// Tabs is a labeled stack of PageBody panels.
type Tabs []Tab

func (Tabs) isPageBody() {}

// ASSERT: layouts implement PageBody
var (
	_ PageBody = Stack(nil)
	_ PageBody = Split(nil)
	_ PageBody = Tabs(nil)
)
