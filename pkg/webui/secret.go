package webui

import "github.com/Olian04/webui/internal/ir"

// Secret projects a value of M that is typed and never read back, such as an API key or a password, as a password input. The stored value is not in
// the page, in a file or in a search, and not in the form that answers a refused submission: the library is never given it. What it is given is
// whether there is one, with Load, and the input then shows a row of bullets, the same whatever the value is, in place of the Placeholder.
//
// A submission that leaves it empty leaves the stored value as it is, since the page cannot show the value and the visitor cannot be asked to type it again
// to change something else. So only a value that is typed is stored, and removing one is not a thing that a form does: it is a choice of its own, which
// the app offers as an [Action] if it wants one. For the same reason a Secret has no Required.
//
// It is for a form: a table, a toolbar and an editor have nothing to show of it.
type Secret[M any] struct {
	// Label is the input's label.
	Label string

	// Load says whether the model has a value: true shows the bullets. The value itself is not asked for.
	Load func(M) bool

	// Store writes the value that was typed to the model, and is not called for an empty submission. A Secret must have one: without it
	// there is nothing to type for.
	Store func(*M, string)

	// Rules are checked in the browser and again on the server, on what is typed. Required is not among them: an empty submission leaves the value
	// as it is, and so cannot be refused. Multiline is not a rule either, since a secret is one line.
	Rules StringRules

	// Placeholder is shown in the input while it is empty and there is no value stored; with one, the bullets are shown.
	Placeholder string

	// Help is a line or two that tells the visitor what the field is for, or what to put in it, where its label and its rules cannot: it is in the popover
	// beside the label that lists the rules, under a heading of its own and above them, on hover and on keyboard focus, and is part of the icon's
	// accessible name. It is text, not markup.
	Help string
}

func (a Secret[M]) label() string { return a.Label }

func (a Secret[M]) meta() accessorMeta {
	return accessorMeta{NoSort: true, NoFilter: true, Help: a.Help}
}

// ASSERT: Secret implements Accessor
var _ Accessor[struct{}] = Secret[struct{}]{}

// validateSecret checks what a secret needs besides what every accessor does: a Store, and a place that can hold an input.
func validateSecret[M any](v *bodyValidator, at string, a Secret[M], where string) {
	validateTextRules(v, at, a.Rules)
	switch {
	case a.Store == nil:
		v.add(at+" has no Store", "A secret is a value to be typed and stored, never read back: set Store to func(*M, string).")
	case where == "Table.Columns":
		v.add(at+" is a Secret in a table", "A secret is never shown, and a column would be empty: put it in a form.")
	case where == "Toolbar":
		v.add(at+" is a Secret in a toolbar", "A toolbar shows values and takes no input, and a secret is never shown: put it in a Form.")
	}
}

// lowerSecret is a Secret as the IR holds it. Everything that reads a field reads Get, so the stored value is nowhere: not in the input, a cell, a
// file, a search or a sort. What is typed is stored, and nothing typed leaves it as it is.
func lowerSecret[M any](a Secret[M], name string) ir.Field {
	f := head(name, a.Label, ir.KindString)
	f.Rules, f.Placeholder = lowerStringRules(a.Rules, false), a.Placeholder
	f.Secret = true
	f.Get = func(any) string { return "" }
	f.Stored = func(m any) bool { return a.Load(m.(M)) }
	if store := a.Store; store != nil {
		f.Set = func(m any, raw string) error {
			if raw != "" {
				store(m.(*M), raw)
			}
			return nil
		}
		f.Unset = func(any) {}
	}
	return f
}
