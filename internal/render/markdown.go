package render

import (
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/markdown"
	c "github.com/Olian04/webui/internal/render/templates/components"
)

// MarkdownView is a markdown leaf with its model loaded. The runtime fills it.
type MarkdownView struct {
	Node *ir.Markdown

	// Result is the rendered text.
	Result markdown.Result

	Failed bool // Load failed: the panel says so, the page stands
}

// MarkdownView renders the text of a markdown leaf. A link in it to a path in the app is
// given the prefix the app is mounted at.
func (r *Renderer) MarkdownView(n *ir.Markdown, source string) MarkdownView {
	return MarkdownView{Node: n, Result: markdown.Render(source, r.prefix)}
}

func (v MarkdownView) status() c.Tone {
	if v.Failed {
		return c.ToneCritical
	}
	return c.ToneNeutral
}
