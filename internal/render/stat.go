package render

import "github.com/Olian04/webui/internal/ir"

// StatView is a stat leaf with its model loaded. The runtime fills it.
type StatView struct {
	Node  *ir.Stat
	Model any

	Failed bool // Load failed: the panel says so, the page stands

	// Retry is where a failed panel leads on to try again: the address of the page.
	Retry string

	// Refusal is what the Load said the visitor may be told, when it was a refusal and not a failure.
	Refusal *ir.Refusal
}
