package webui

import (
	"context"

	"github.com/Olian04/webui/internal/ir"
)

// Diff is a leaf that shows two texts of one model and what changed between them,
// side by side in a code editor when the page is wide enough and one above the other
// when it is not, with both highlighted for the [Language]. It only reads: the two
// accessors have no Store.
//
// Like an [Editor] it is Monaco, and still works without script: the page then holds
// the changes as a unified diff, which the server makes. A Diff never loads a
// language service, so only highlighting is ever shipped for it.
type Diff[M any] struct {
	// Title is the panel's heading.
	Title string

	// Desc is a description, shown in a popover from an information icon beside the
	// title.
	Desc string

	// Load returns the model that holds both texts.
	Load func(ctx context.Context) (M, error)

	// Original is the text that was, and Modified the text that is. Their labels name
	// the two sides, and must differ.
	Original String[M]
	Modified String[M]

	// Language is the language of both texts. It is plain text by default.
	Language Language
}

// ASSERT: Diff implements PageBody
var _ PageBody = Diff[struct{}]{}

func (d Diff[M]) validateBody(v *bodyValidator) {
	needLoad(v, "a Diff", d.Load != nil)
	validateAccessors(v, []Accessor[M]{d.Original, d.Modified}, accessorSite{where: "Diff", uniqueLbl: true})
	if d.Original.Store != nil || d.Modified.Store != nil {
		v.add("a Diff has a Store", "A Diff only reads: remove Store, or use an Editor to change a text.")
	}
	validateLanguage(v, "Diff", d.Language)
}

func (d Diff[M]) lowerBody(at ir.Addr) ir.Node {
	return &ir.Diff{
		At: at, Title: d.Title, Desc: d.Desc, Language: d.Language.id(),
		Load:     eraseLoad(d.Load),
		Original: lowerAccessor[M](d.Original, "o"),
		Modified: lowerAccessor[M](d.Modified, "m"),
	}
}
