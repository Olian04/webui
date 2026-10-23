package render

import (
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/markdown"
)

// MarkdownView is a markdown leaf with its model loaded. The runtime fills it.
type MarkdownView struct {
	Node *ir.Markdown

	// Result is the rendered text.
	Result markdown.Result

	Failed bool // Load failed: the panel says so, the page stands

	// Retry is where a failed panel leads on to try again: the address of the page.
	Retry string

	// Refusal is what the Load said the visitor may be told, when it was a refusal and not a failure.
	Refusal *ir.Refusal
}

// MarkdownView renders the text of a markdown leaf. A link in it to a path in the app is
// given the prefix the app is mounted at.
func (r *Renderer) MarkdownView(n *ir.Markdown, source string) MarkdownView {
	return MarkdownView{Node: n, Result: markdown.Render(source, markdown.Options{Prefix: r.prefix, TrustedOrigins: n.TrustedOrigins})}
}
