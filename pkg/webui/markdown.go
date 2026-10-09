package webui

import (
	"context"

	"github.com/Olian04/webui/internal/ir"
)

// Markdown is a leaf that shows one text of a model, set as markdown: headings,
// emphasis, lists, links, quotes and code, and the tables, task lists, strikethrough
// and bare links of GitHub's flavour. It is for text the application has, such as a
// runbook, release notes or a README, and it only reads: the accessor has no Store.
//
// The text is not trusted any more than any other text of the model. Raw HTML in it is
// never rendered, a link goes only to an http or https address or a mailto and is
// otherwise left as its text, and a link opens in a tab of its own. An image is not
// drawn, since the page would refuse a picture from elsewhere and a picture is a way
// for a text to make the visitor's browser fetch from it: it is a link to the
// picture, named by its alt text.
//
// It needs no script. A fenced code block that names a language is coloured by the
// editor, which the page then loads, as it does for an [Editor]; a block that does
// not, and every block without script, is plain text in a monospace block.
type Markdown[M any] struct {
	// Title is the panel's heading.
	Title string

	// Desc is a description, shown in a popover from an information icon beside the
	// title.
	Desc string

	// Load returns the model that holds the text.
	Load func(ctx context.Context) (M, error)

	// Content is the markdown. Its Label names the text to a screen reader.
	Content String[M]
}

func (Markdown[M]) isPageBody() {}

// ASSERT: Markdown implements PageBody
var _ PageBody = Markdown[struct{}]{}

func (m Markdown[M]) validateBody(v *bodyValidator) {
	if m.Load == nil {
		v.add("a Markdown has no Load", "Set Load to func(ctx) (M, error).")
	}
	validateAccessors(v, []Accessor[M]{m.Content}, accessorSite{where: "Markdown.Content", uniqueLbl: true}, map[string]bool{})
	if m.Content.Store != nil {
		v.add("a Markdown has a Store", "A Markdown only reads: remove Store, or use an Editor to change the text.")
	}
}

func (m Markdown[M]) lowerBody(at ir.Addr) ir.Node {
	return &ir.Markdown{
		At: at, Title: m.Title, Desc: m.Desc,
		Load:    func(ctx context.Context) (any, error) { return m.Load(ctx) },
		Content: lowerAccessor[M](m.Content, "m"),
	}
}
