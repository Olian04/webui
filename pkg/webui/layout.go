package webui

import (
	"fmt"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
)

// Stack lays out children vertically. Children may be any PageBody.
type Stack []PageBody

func (Stack) isPageBody() {}

// Split lays out children side by side. Children may be any PageBody.
type Split []PageBody

func (Split) isPageBody() {}

// Tab is one panel in Tabs. It is selected in the address by its label, in lower
// case with dashes for anything else: the "Raw events" tab is "?tabs.tab=raw-events".
type Tab struct {
	Label string
	Body  PageBody
}

func (t Tab) key() string { return args.Slug(t.Label) }

// Tabs is a labeled set of PageBody panels. The selected tab is kept in the
// address as "<ID>.tab", so it survives a reload and can be linked to; the
// library owns that parameter. ID defaults to "tabs" and must be unique within
// the page. Only the selected panel is loaded. An empty or unknown value selects the first
// tab.
type Tabs struct {
	ID     string
	Panels []Tab
}

func (Tabs) isPageBody() {}

// ASSERT: layouts implement PageBody
var (
	_ PageBody = Stack(nil)
	_ PageBody = Split(nil)
	_ PageBody = Tabs{}
)

func (s Stack) validateBody(v *bodyValidator) { v.nodes++; validateChildren(v, "Stack", s) }

func (s Split) validateBody(v *bodyValidator) { v.nodes++; validateChildren(v, "Split", s) }

func validateChildren(v *bodyValidator, what string, children []PageBody) {
	for i, c := range children {
		if c == nil {
			v.add(fmt.Sprintf("%s[%d] is nil", what, i), "Remove it or declare a body.")
			continue
		}
		c.validateBody(v)
	}
}

func (t Tabs) validateBody(v *bodyValidator) {
	v.nodes++
	v.id("Tabs", t.ID)
	v.tabs++
	tabs := v.tabs
	if len(t.Panels) == 0 {
		v.add("Tabs has no Panels", "Declare at least one Tab.")
	}
	seen := map[string]bool{}
	for i, p := range t.Panels {
		switch {
		case p.Label == "":
			v.add(fmt.Sprintf("Tabs.Panels[%d] has no Label", i), "Set Label; it is the tab text.")
		case p.key() == "":
			v.add(fmt.Sprintf("Tabs.Panels[%d]: the label %q has no letters or digits", i, p.Label),
				"A tab is named in the address by its label; use one with a letter or digit in it.")
		case seen[p.key()]:
			v.add(fmt.Sprintf("Tabs.Panels[%d]: the label %q names the same tab as another in the address", i, p.Label),
				"Reword one of them; a tab is named in the address by its label, in lower case.")
		}
		seen[p.key()] = true
		if p.Body == nil {
			v.add(fmt.Sprintf("Tabs.Panels[%d] has no Body", i), "Set Body to a Table, Form or layout.")
			continue
		}
		v.scope = append(v.scope, panel{tabs: tabs, index: i})
		p.Body.validateBody(v)
		v.scope = v.scope[:len(v.scope)-1]
	}
}

func (s Stack) lowerBody(at ir.Addr, l *bodyLowerer) ir.Node {
	*l.nodes++
	return &ir.Stack{At: at, Children: lowerChildren(at, l, s)}
}

func (s Split) lowerBody(at ir.Addr, l *bodyLowerer) ir.Node {
	*l.nodes++
	return &ir.Split{At: at, Children: lowerChildren(at, l, s)}
}

func (t Tabs) lowerBody(at ir.Addr, l *bodyLowerer) ir.Node {
	*l.nodes++
	out := &ir.Tabs{At: at, ID: effectiveID(t.ID, "tabs")}
	for i, p := range t.Panels {
		out.Tabs = append(out.Tabs, ir.Tab{Key: p.key(), Label: p.Label, Body: p.Body.lowerBody(childAddr(at, i), l)})
	}
	return out
}

func lowerChildren(at ir.Addr, l *bodyLowerer, children []PageBody) []ir.Node {
	out := make([]ir.Node, len(children))
	for i, c := range children {
		out[i] = c.lowerBody(childAddr(at, i), l)
	}
	return out
}

// childAddr is the only place an Addr grows. It is derived from position and
// nothing the user typed, so a refresh request cannot name anything else.
func childAddr(at ir.Addr, i int) ir.Addr {
	return append(append(ir.Addr(nil), at...), i)
}
