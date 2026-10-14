package webui

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/Olian04/webui/internal/editorctx"
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

	// Context is what the editor's language service is given besides the text, for the
	// languages that can use it: TypeScript and JavaScript, and JSON. It is one folder, such
	// as an embedded one, with a directory for each language it gives context to, and an
	// editor reads only the directory named for its own Language:
	//
	//	typescript/tsconfig.json   compiler options, as a tsconfig.json has them
	//	typescript/*.d.ts          declarations, at any depth: globals the text may use, and
	//	                           the types of libraries, such as
	//	                           typescript/node_modules/@types/foo/index.d.ts
	//	javascript/...             the same, for a JavaScript editor
	//	json/schema.json           the schema the text must satisfy
	//	json/*.json                other schemas, for schema.json to refer to
	//
	// The service then checks the text against them: a name an environment file declares
	// is known, a type from a library is completed, and a value that breaks the schema is
	// marked. Only compilerOptions of tsconfig.json is read, and only the options that
	// change what is an error; anything the editor would ignore is a compile error, and so
	// is any file it would not read. A folder embedded with //go:embed is the usual source,
	// and fs.Sub narrows a larger one to what an editor takes.
	//
	// Like the service itself, Context goes only to an editor that can be edited, and only
	// to a visitor who may edit it. The files are not in the page: it names them by
	// address, which carries a key that only such a page gives out (see [App.Secret]), so
	// a browser keeps them, and a visitor who cannot edit is not handed them. The page
	// fetches every file of its language's directory when an editor mounts, so a folder
	// of a hundred libraries costs a slow editor.
	//
	// A page's TypeScript service is one program, so the editors of one page share their
	// Context: two of them may not give one file two texts. JSON schemas are each their
	// own editor's.
	Context fs.FS

	// Submit is what saving does, as a [Form]'s is: its Guard decides who may save, and
	// a visitor it refuses sees the text as a viewer does. It is required with a Store
	// and an error without one.
	Submit Action[M]
}

// ASSERT: Editor implements PageBody
var _ PageBody = Editor[struct{}]{}

func (e Editor[M]) editable() bool { return e.Content.Store != nil }

func (e Editor[M]) validateBody(v *bodyValidator) {
	needLoad(v, "an Editor", e.Load != nil)
	validateAccessors(v, []Accessor[M]{e.Content}, accessorSite{where: "Editor.Content", uniqueLbl: true})
	validateLanguage(v, "Editor", e.Language)
	e.validateContext(v)
	switch {
	case e.editable() && e.Submit.Run == nil:
		v.add("an Editor with a Store has no Submit.Run", "Set Submit.Run to say what saving does, or remove Content.Store to make it a viewer.")
	case !e.editable() && (e.Submit.Run != nil || e.Submit.Guard != nil || e.Submit.Label != ""):
		v.add("Editor.Submit is set but Content has no Store", "A viewer saves nothing: set Content.Store to make it an editor, or remove Submit.")
	case e.editable():
		validateAction(v, "Editor.Submit", e.Submit, false)
	}
}

func (e Editor[M]) lowerBody(at ir.Addr) ir.Node {
	f := Form[M]{Title: e.Title, Desc: e.Desc, Load: e.Load, Fields: []Accessor[M]{e.Content}, Submit: e.Submit}
	out := f.lowerBody(at).(*ir.Form)
	out.Editor = &ir.Editor{Language: e.Language.id(), Service: e.Language.service(), Context: lowerContext(e.Context, e.Language)}
	return out
}

func (e Editor[M]) validateContext(v *bodyValidator) {
	if e.Context == nil {
		return
	}
	dir := editorctx.Dir(e.Language.id())
	switch {
	case !e.editable():
		v.add("Editor.Context is set but Content has no Store",
			"A viewer loads no language service, so nothing would read the context: set Content.Store to make it an editor, or remove Context.")
		return
	case dir == "":
		v.add(fmt.Sprintf("Editor.Context is set but the language %q takes none", e.Language.id()),
			"Context is for TypeScript, JavaScript and JSON editors; remove it, or change Language.")
		return
	}
	ctx, problems := editorctx.Read(e.Context, dir)
	for _, p := range problems {
		v.add("Editor.Context: "+p.Detail, p.Fix)
	}
	if ctx == nil {
		return
	}
	if v.contexts == nil {
		v.contexts = map[string]string{}
	}
	if dir == "json" {
		return // a schema is matched to its own editor, so editors never share one
	}
	if ctx.Compiler != nil {
		if v.compilers == nil {
			v.compilers = map[string]string{}
		}
		opts := fmt.Sprint(ctx.Compiler)
		if had, ok := v.compilers[dir]; ok && had != opts {
			v.add(fmt.Sprintf("two editors on this page give the %s service different compiler options in their Context", dir),
				"A page has one service for a language, so its editors share one set of options: use the same tsconfig.json in both, or put the editors on different pages.")
		}
		v.compilers[dir] = opts
	}
	for _, f := range ctx.Files {
		key := dir + f.Path
		if had, ok := v.contexts[key]; ok && had != f.Name {
			v.add(fmt.Sprintf("two editors on this page give %s/%s a different text in their Context", dir, f.Path[1:]),
				"A page has one TypeScript service, so its editors share their context: use the same file in both, or put the editors on different pages.")
		}
		v.contexts[key] = f.Name
	}
}

func lowerContext(fsys fs.FS, language Language) *editorctx.Context {
	if fsys == nil {
		return nil
	}
	ctx, _ := editorctx.Read(fsys, editorctx.Dir(language.id())) // checked by validate
	return ctx
}
