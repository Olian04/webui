package webui

import (
	"context"
	"fmt"

	"github.com/Olian04/webui/internal/ir"
)

// Editor is a leaf that shows one text of a model in a code editor: with syntax
// highlighting for its [Language], line numbers, search, and what a code editor has.
// With a Store on its Content it edits the text and saves it, like a [Form]; without
// one it is a viewer of the text, with the same highlighting and no way to change it.
//
// The editor is Monaco, which is the editor of VS Code, and a page that has one loads
// it. Everything still works without script: an editor is then a plain text box with a
// Save button, and a viewer is the text in a monospace block.
//
// A language service (diagnostics, completion and the like) is available for a few
// languages, and only to an editor that can be edited: a viewer never loads one. Each
// is an optional import, since each is a worker of its own that most applications do
// not want in the program: pkg/monaco/json, pkg/monaco/css, pkg/monaco/html and
// pkg/monaco/typescript. Without the import the editor highlights and nothing more.
type Editor[M any] struct {
	// Title is the panel's heading.
	Title string

	// Desc is a description, shown in a popover from an information icon beside the
	// title.
	Desc string

	// Load returns the model the editor shows. On submit it is loaded again, and the
	// text is applied to it, as with a [Form].
	Load func(ctx context.Context) (M, error)

	// Content is the text: Load reads it, and Store, if there is one, writes what was
	// saved. Without Store the editor is a viewer. Its Label names the editor to a
	// screen reader and to a [FieldError], and its Rules apply to what is saved: use
	// MaxLen to bound it.
	Content String[M]

	// Language is the language of the text, such as [LangJSON]. It is plain text by
	// default.
	Language Language

	// Submit is what saving does, as a [Form]'s is: its Guard decides who may save, and
	// a visitor it refuses sees the text as a viewer does. It is required with a Store
	// and an error without one.
	Submit Action[M]
}

func (Editor[M]) isPageBody() {}

// ASSERT: Editor implements PageBody
var _ PageBody = Editor[struct{}]{}

func (e Editor[M]) editable() bool { return e.Content.Store != nil }

func (e Editor[M]) validateBody(v *bodyValidator) {
	if e.Load == nil {
		v.add("an Editor has no Load", "Set Load to func(ctx) (M, error).")
	}
	validateAccessors(v, []Accessor[M]{e.Content}, accessorSite{where: "Editor.Content", uniqueLbl: true}, map[string]bool{})
	if !e.Language.known() {
		v.add(fmt.Sprintf("Editor.Language %q is not a language the editor highlights", string(e.Language)),
			"Use one of the Lang constants, such as LangJSON or LangGo; there is one for each language the editor knows.")
	}
	switch {
	case e.editable() && e.Submit.Run == nil:
		v.add("an Editor with a Store has no Submit.Run", "Set Submit.Run to say what saving does, or remove Content.Store to make it a viewer.")
	case !e.editable() && (e.Submit.Run != nil || e.Submit.Guard != nil || e.Submit.Label != ""):
		v.add("Editor.Submit is set but Content has no Store", "A viewer saves nothing: set Content.Store to make it an editor, or remove Submit.")
	case e.editable():
		validateAction(v, "Editor.Submit", e.Submit.Label, true, false)
	}
}

func (e Editor[M]) lowerBody(at ir.Addr) ir.Node {
	f := Form[M]{Title: e.Title, Desc: e.Desc, Load: e.Load, Fields: []Accessor[M]{e.Content}, Submit: e.Submit}
	out := f.lowerBody(at).(*ir.Form)
	out.Editor = &ir.Editor{Language: e.Language.id(), Service: e.Language.service()}
	return out
}
