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

// NewMarkdownView renders the text of a markdown leaf.
func NewMarkdownView(n *ir.Markdown, source string) MarkdownView {
	return MarkdownView{Node: n, Result: markdown.Render(source)}
}

func (v MarkdownView) status() c.Tone {
	if v.Failed {
		return c.ToneCritical
	}
	return c.ToneNeutral
}
