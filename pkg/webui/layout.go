package webui

import (
	"fmt"
	"maps"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
)

// Stack lays out bodies vertically, in a column of panels. Its children may be any
// [PageBody], including other layouts.
type Stack []PageBody

func (Stack) isPageBody() {}

// Split lays out bodies side by side, across the width of the page. It collapses to
// a column on a narrow screen. Its children may be any [PageBody].
type Split []PageBody

func (Split) isPageBody() {}

// Tab is one panel in Tabs. It is selected in the address by its label, in lower
// case with dashes for anything else: the "Raw events" tab is "?tabs.tab=raw-events".
type Tab struct {
	// Label is the text of the tab, and what names it in the address.
	Label string

	// Body is what the tab shows, loaded only while the tab is selected.
	Body PageBody
}

func (t Tab) key() string { return args.Slug(t.Label) }

// Tabs is a set of labelled bodies, of which one is shown at a time. The selected
// tab is kept in the address as "tabs.tab" ("tabs-2.tab" for a second Tabs on the
// page), so it survives a reload and can be linked to. The library owns that
// parameter. Only the selected panel is loaded, and an empty or unknown value
// selects the first.
type Tabs struct {
	// Panels are the tabs, in order.
	Panels []Tab
}

func (Tabs) isPageBody() {}

// ASSERT: layouts implement PageBody
var (
	_ PageBody = Stack(nil)
	_ PageBody = Split(nil)
	_ PageBody = Tabs{}
)

func (s Stack) validateBody(v *bodyValidator) { validateChildren(v, "Stack", s) }

func (s Split) validateBody(v *bodyValidator) { validateChildren(v, "Split", s) }

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
		p.Body.validateBody(v)
	}
}

func (s Stack) lowerBody(at ir.Addr) ir.Node {
	return &ir.Stack{At: at, Children: lowerChildren(at, s)}
}

func (s Split) lowerBody(at ir.Addr) ir.Node {
	return &ir.Split{At: at, Children: lowerChildren(at, s)}
}

func (t Tabs) lowerBody(at ir.Addr) ir.Node {
	out := &ir.Tabs{At: at}
	for i, p := range t.Panels {
		out.Tabs = append(out.Tabs, ir.Tab{Key: p.key(), Label: p.Label, Body: p.Body.lowerBody(childAddr(at, i))})
	}
	return out
}

func lowerChildren(at ir.Addr, children []PageBody) []ir.Node {
	out := make([]ir.Node, len(children))
	for i, c := range children {
		out[i] = c.lowerBody(childAddr(at, i))
	}
	return out
}

// childAddr is the only place an Addr grows. It is derived from position and
// nothing the user typed, so a refresh request cannot name anything else.
func childAddr(at ir.Addr, i int) ir.Addr {
	return append(append(ir.Addr(nil), at...), i)
}

// assignViewIDs names the view state of every table and tabs on a page: the part
// before the dot in "devices.sort" and "tabs.tab". A table is named by its title,
// spelled as the address spells any label, and a Tabs is "tabs". A name that
// something else on screen already has gets "-2", then "-3". The panels of one Tabs
// are never on screen together, so each starts from the names taken before the Tabs
// and may repeat another panel's: the same table in two panels keeps its sort and
// filters in both.
func assignViewIDs(body ir.Node) {
	var walk func(n ir.Node, taken map[string]bool)
	claim := func(base string, taken map[string]bool) string {
		id := base
		for n := 2; taken[id]; n++ {
			id = fmt.Sprintf("%s-%d", base, n)
		}
		taken[id] = true
		return id
	}
	walk = func(n ir.Node, taken map[string]bool) {
		switch n := n.(type) {
		case *ir.Stack:
			for _, c := range n.Children {
				walk(c, taken)
			}
		case *ir.Split:
			for _, c := range n.Children {
				walk(c, taken)
			}
		case *ir.Tabs:
			n.ID = claim("tabs", taken)
			all := maps.Clone(taken)
			for _, t := range n.Tabs {
				panel := maps.Clone(taken)
				walk(t.Body, panel)
				maps.Copy(all, panel)
			}
			maps.Copy(taken, all)
		case *ir.Table:
			base := args.Slug(n.Title)
			if base == "" {
				base = "table"
			}
			n.ID = claim(base, taken)
		}
	}
	walk(body, map[string]bool{})
}
