package webui

import (
	"context"
	"fmt"
	"maps"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
)

// Stack lays out bodies vertically, in a column of panels. Its children may be any
// [PageBody], including other layouts.
type Stack []PageBody

// Split lays out bodies side by side, across the width of the page. It collapses to
// a column on a narrow screen. Its children may be any [PageBody].
type Split []PageBody

// Column is one column of a [Columns]: a body, and its share of the width.
type Column struct {
	// Weight is the column's share of the row, relative to the others: Columns{{Weight: 25, Body: a}, {Weight: 50, Body: b}, {Weight: 25, Body: c}}
	// is 25/50/25, and a Weight of 1 and one of 3 is 25/75. Zero is 1, so a Column with no Weight is as wide as any other.
	Weight uint8

	// Body is what the column shows. It may be any [PageBody], including another layout.
	Body PageBody
}

// Columns lays out bodies side by side in a row, each as wide as its Weight says: 25/75, 75/25 and 25/50/25 are
// {1, 3}, {3, 1} and {1, 2, 1}. Where [Split] is two equal columns that wrap into rows of two, Columns is one row that is
// divided by weight.
//
// A column is never narrower than 280px. When a row cannot hold all of its columns at that width, the last of them
// move to a row of their own, and each row shares its width by weight in the same way: 25/50/25 on a narrow screen is a
// row of 25/50 above one of 25, and on a narrower one three rows of one. On a screen too narrow for two columns it is
// one column, in the order declared.
type Columns []Column

// When shows its Body only while Show says so of what Load reads, and leaves no gap when it does not. Load reads the model the condition is decided from, as a leaf's
// Load does, and Show is a [BoolLike] of that model, a [Bool] or an [OptionalBool], the same accessor that a form or a table shows it with; "show when false" is a Bool whose Load returns the negation, and a value that is not set is not shown. The Body
// loads its own data: the model is not passed to it, so a body that needs what Load read asks for it too, and an app that cares caches it, as it does for any two panels.
//
// A When is a layout that loads, and loads for the condition alone. It runs Load, then Show, then builds its Body, so its Load is in front of the Body's: keep it cheap. If Load fails the
// panel says "Could not load", the cause is logged and the Body is not shown, since a condition that cannot be decided does not hold. A [Loaded.RefreshAfter] from it does nothing, since a
// layout has nowhere to be refreshed from; the Body's own leaves refresh themselves.
//
// What a When hides cannot be reached. A refresh, a form post or an export that names a leaf under a When that does not hold is answered as it would be for a leaf that is not there, and
// a failed Load is as good as a condition that does not hold. The condition is asked again for each such request. That is how a panel that is shown only to some stays out of reach of the others, but
// a When is for what a page shows: the page's Guard is what lets a visitor open it, and an Action's Guard is what lets them act.
//
// Its position is that of one child, so a leaf's address is the same whether it is shown or not. A When in a [Columns] that does not hold leaves an empty column, which is not drawn,
// so the row wraps as if it were not declared; in a [Tabs] it leaves a blank tab, so put it outside.
type When[M any] struct {
	// Load reads what the condition is decided from. It is called on every request that draws the page, and on every request for a leaf under it.
	Load func(ctx context.Context) (M, Loaded)

	// Show says whether the Body is shown, of the model Load read: a [Bool], or an [OptionalBool], which is not shown when it is not set. Its Label, Store, Help, NoSort and NoFilter do nothing here,
	// as they do nothing where an accessor is only read.
	Show BoolLike[M]

	// Body is what is shown. It may be any [PageBody], including another When, which is "and".
	Body PageBody
}

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

// ASSERT: layouts implement PageBody
var (
	_ PageBody = Stack(nil)
	_ PageBody = Split(nil)
	_ PageBody = Columns(nil)
	_ PageBody = When[struct{}]{}
	_ PageBody = Tabs{}
)

func (s Stack) validateBody(v *bodyValidator) { validateChildren(v, "Stack", s) }

func (s Split) validateBody(v *bodyValidator) { validateChildren(v, "Split", s) }

func (c Columns) validateBody(v *bodyValidator) {
	if len(c) == 0 {
		v.add("Columns has no Column", "Declare at least one Column, with the Body it shows and, if it is not as wide as the others, a Weight.")
	}
	for i, col := range c {
		if col.Body == nil {
			v.add(fmt.Sprintf("Columns[%d].Body is nil", i), "Set Body to a Table, Form or layout.")
			continue
		}
		col.Body.validateBody(v)
	}
}

func (w When[M]) validateBody(v *bodyValidator) {
	if w.Load == nil {
		v.add("a When has no Load", "Set Load to func(ctx) (M, Loaded), reading what the condition is decided from.")
	}
	if w.Show == nil || !w.Show.hasLoad() {
		v.add("When.Show has no Load", "Set Show to a Bool, or an OptionalBool, with a Load that says whether the Body is shown of the model.")
	}
	if w.Body == nil {
		v.add("When.Body is nil", "Set Body to a Table, Form or layout.")
		return
	}
	w.Body.validateBody(v)
}

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

func (c Columns) lowerBody(at ir.Addr) ir.Node {
	out := &ir.Columns{At: at}
	for i, col := range c {
		out.Children = append(out.Children, col.Body.lowerBody(childAddr(at, i)))
		out.Weights = append(out.Weights, max(int(col.Weight), 1))
	}
	return out
}

func (w When[M]) lowerBody(at ir.Addr) ir.Node {
	return &ir.When{At: at, Load: eraseLoad(w.Load), Show: lowerAccessor[M](w.Show, "w"), Child: w.Body.lowerBody(childAddr(at, 0))}
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

// assignViewIDs names the view state of every table, list of items, calendar and tabs on a page: the part
// before the dot in "devices.sort", "latest.limit", "releases.month" and "tabs.tab". A table is named by its title,
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
		case *ir.Columns:
			for _, c := range n.Children {
				walk(c, taken)
			}
		case *ir.When:
			walk(n.Child, taken)
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
		case *ir.Items:
			n.ID = claim(itemsViewID(n), taken)
		case *ir.Calendar:
			n.ID = claim(calendarViewID(n), taken)
		}
	}
	walk(body, map[string]bool{})
}
