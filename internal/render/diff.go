package render

import (
	"strconv"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/textdiff"
)

// DiffView is a diff leaf with its model loaded. The runtime fills it.
type DiffView struct {
	Node *ir.Diff

	Original, Modified string

	Failed bool // Load failed: the panel says so, the page stands

	// Retry is where a failed panel leads on to try again: the address of the page.
	Retry string

	// Refusal is what the Load said the visitor may be told, when it was a refusal and not a failure.
	Refusal *ir.Refusal

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

// lineMark is how a line of the unified diff is shown: the class of its row, and the mark before it.
func lineMark(k textdiff.Kind) (class, mark string) {
	switch k {
	case textdiff.Insert:
		return "ins", "+"
	case textdiff.Delete:
		return "del", "−"
	case textdiff.Equal:
	}
	return "", " "
}

// lineNumber is a line's number in one of the two texts, and nothing on the side where it is not.
func lineNumber(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}
