package render

import (
	"strconv"

	"github.com/Olian04/webui/internal/ir"
	c "github.com/Olian04/webui/internal/render/templates/components"
	"github.com/Olian04/webui/internal/textdiff"
)

// DiffView is a diff leaf with its model loaded. The runtime fills it.
type DiffView struct {
	Node *ir.Diff

	Original, Modified string

	Failed bool // Load failed: the panel says so, the page stands

	// Hunks, Added and Removed are the changes between the two texts, and Compared is
	// false when they were too large or too different to compare here.
	Hunks          []textdiff.Hunk
	Added, Removed int
	Compared       bool
}

// NewDiffView compares the two texts of a diff.
func NewDiffView(n *ir.Diff, original, modified string) DiffView {
	v := DiffView{Node: n, Original: original, Modified: modified}
	v.Hunks, v.Added, v.Removed, v.Compared = textdiff.Unified(original, modified, 3)
	return v
}

func (v DiffView) status() c.Tone {
	if v.Failed {
		return c.ToneCritical
	}
	return c.ToneNeutral
}

// unifiedLine is a line of the unified diff, with how it is shown.
type unifiedLine textdiff.Line

func (l unifiedLine) class() string {
	switch l.Kind {
	case textdiff.Insert:
		return "ins"
	case textdiff.Delete:
		return "del"
	case textdiff.Equal:
	}
	return ""
}

func (l unifiedLine) mark() string {
	switch l.Kind {
	case textdiff.Insert:
		return "+"
	case textdiff.Delete:
		return "−"
	case textdiff.Equal:
	}
	return " "
}

func (l unifiedLine) old() string {
	if l.Old == 0 {
		return ""
	}
	return strconv.Itoa(l.Old)
}

func (l unifiedLine) new() string {
	if l.New == 0 {
		return ""
	}
	return strconv.Itoa(l.New)
}
