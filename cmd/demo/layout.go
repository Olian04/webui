package main

import (
	"context"
	"time"

	"github.com/Olian04/webui/pkg/webui"
)

// extra is a field of a form with its own label, so that a form can be as tall as it is told to be.
func extra(label string) webui.String[Note] {
	return webui.String[Note]{Label: label, Load: func(n Note) string { return n.Text }, Store: func(n *Note, v string) { n.Text = v }}
}

// editable is a form that has a foot (its Save button), and as many fields as it is given.
func editable(title string, fields ...string) webui.Form[Note] {
	accessors := make([]webui.Accessor[Note], len(fields))
	for i, f := range fields {
		accessors[i] = extra(f)
	}
	return webui.Form[Note]{
		Title:  title,
		Load:   func(context.Context) (Note, error) { return Note{Text: "text"}, nil },
		Fields: accessors,
		Submit: webui.Action[Note]{Run: func(context.Context, Note) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
	}
}

// The panels of a Split side by side are as tall as the tallest of them, and the rows of a Split are independent: five
// panels in two columns make three rows, each as tall as its tallest panel and not as the rows around it. A panel's
// foot (a pager, a Save button) stays at the bottom of its box.
var LayoutSplit = webui.Page[webui.NoArgs]{
	Path: "/layout/split",
	Nav:  webui.Nav{Label: "Split rows", Icon: "table-columns", Section: "Layout"},
	Body: webui.Stack{
		intro("Five panels in one Split make three rows of two columns. In each row the panels are as tall as the tallest one, with their borders lined up, and a pager or a Save button at the bottom of its box. The last row has one panel, which is as tall as it is and is not stretched to the rows above. The Split after it is its own: its rows do not follow these. Narrow the window below 1100 px and the columns become one, each panel its natural height."),
		webui.Split{
			editable("Row 1, left: a tall form", "Name", "Address", "Owner", "Region", "Notes"),
			backend("Row 1, right: two rows", 0, 2),
			backend("Row 2, left: a table with a pager", 0, 12),
			editable("Row 2, right: a short form", "Name"),
			backend("Row 3, alone: not stretched", 0, 3),
		},
		webui.Split{
			backend("Second Split, left (1 row)", 0, 1),
			backend("Second Split, right (6 rows)", 0, 6),
		},
		backend("A panel outside any Split, as tall as it is (20 ms)", 20*time.Millisecond, 2),
	},
}
