package webui_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Olian04/webui/pkg/lang"
	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

func ctxFiles(m map[string]string) fstest.MapFS {
	out := fstest.MapFS{}
	for name, text := range m {
		out[name] = &fstest.MapFile{Data: []byte(text)}
	}
	return out
}

func editorWith(language webui.Language, ctx fstest.MapFS, store bool) webui.Editor[snippet] {
	content := snippetView
	submit := webui.Action[snippet]{}
	if store {
		content = snippetText
		submit = webui.Action[snippet]{Run: func(context.Context, snippet) (webui.Outcome, error) { return webui.Success("Saved"), nil }}
	}
	return webui.Editor[snippet]{Title: "E", Load: loadSnippet, Content: content, Language: language, Submit: submit, Context: ctx}
}

func TestAnEditorContextThatCannotBeUsedIsACompileErrorThatSaysWhy(t *testing.T) {
	t.Parallel()

	ts := map[string]string{"typescript/env.d.ts": "declare const x: 1;"}
	cases := []struct {
		body  webui.View
		names []string
	}{
		{editorWith(lang.TypeScript, ctxFiles(ts), false), []string{"Editor.Context", "Store"}},
		{editorWith(lang.Go, ctxFiles(ts), true), []string{`"go"`}},
		{editorWith(lang.JSON, ctxFiles(ts), true), []string{`"json"`}},
		{editorWith(lang.TypeScript, ctxFiles(map[string]string{"typescript/a.d.ts": "", "a.d.ts": ""}), true), []string{`"a.d.ts"`}},
		{editorWith(lang.TypeScript, ctxFiles(map[string]string{"typescript/tsconfig.json": `{"compilerOptions": {"outDir": "x"}}`}), true), []string{"compilerOptions.outDir"}},
		{editorWith(lang.JSON, ctxFiles(map[string]string{"json/schema.json": "{"}), true), []string{"json/schema.json", "JSON"}},
		{editorWith(lang.JSON, ctxFiles(map[string]string{"json/other.json": "{}"}), true), []string{"json/", "schema.json"}},
	}
	for _, tc := range cases {
		got := editorErrors(t, tc.body)
		refuses(t, got, append([]string{"Editor.Context"}, tc.names...)...) // each says where it comes from, and what
	}
}

func TestEditorsOnOnePageShareOneProgramSoTheyMayNotDisagreeAboutIt(t *testing.T) {
	t.Parallel()

	a := editorWith(lang.TypeScript, ctxFiles(map[string]string{"typescript/env.d.ts": "declare const x: 1;"}), true)
	b := editorWith(lang.TypeScript, ctxFiles(map[string]string{"typescript/env.d.ts": "declare const x: 2;"}), true)
	b.Content = webui.String[snippet]{Label: "Other", Load: snippetText.Load, Store: snippetText.Store}
	refuses(t, editorErrors(t, webui.Stack{a, b}), "typescript/env.d.ts")

	c := editorWith(lang.TypeScript, ctxFiles(map[string]string{"typescript/tsconfig.json": `{"compilerOptions": {"strict": true}}`}), true)
	d := editorWith(lang.TypeScript, ctxFiles(map[string]string{"typescript/tsconfig.json": `{"compilerOptions": {"strict": false}}`}), true)
	d.Content = b.Content
	refuses(t, editorErrors(t, webui.Stack{c, d}), "compiler options")

	// The same context twice, and a TypeScript editor beside a JavaScript one, are fine.
	same := ctxFiles(map[string]string{"typescript/env.d.ts": "declare const x: 1;", "javascript/env.d.ts": "declare const y: 2;"})
	e, f := editorWith(lang.TypeScript, same, true), editorWith(lang.JavaScript, same, true)
	f.Content = b.Content
	_, err := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", View: webui.Stack{e, f}}}}.Compile("/admin")
	assert.NoError(t, err)
}

func TestASecretThatIsTooShortToKeepAnAddressPrivateIsACompileError(t *testing.T) {
	t.Parallel()

	errs := compileErrors(t, webui.App{Secret: []byte("short"), Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/", View: webui.Stack{}}}})
	refuses(t, errs[0].Error(), "App.Secret")
}

func TestWithNoLanguageServiceImportedAnEditorsContextIsNotHandedOver(t *testing.T) {
	t.Parallel()

	// This program imports no service, so nothing would read the context.
	h := editorApp(editorWith(lang.TypeScript, ctxFiles(map[string]string{"typescript/env.d.ts": "declare const x: 1;"}), true))
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.False(t, strings.Contains(body, "/_webui/code/"))
	assert.False(t, strings.Contains(body, `"context"`))
}
