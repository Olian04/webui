package render

import (
	"github.com/Olian04/webui/internal/ir"
)

// LoadedEmbed is an embedded page's panel with the address of its page. The runtime fills it.
type LoadedEmbed struct {
	Node *ir.Embed

	// Src is the path of the page that the frame shows.
	Src string

	Failed bool // the address could not be had: the panel says so, the page stands

	// Retry is where a failed panel leads on to try again: the address of the page.
	Retry string

	// Refusal is what the Load said the visitor may be told, when it was a refusal and not a failure.
	Refusal *ir.Refusal
}
