package webui_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

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
		body webui.PageBody
		want string
	}{
		{editorWith(webui.LangTypeScript, ctxFiles(ts), false), "Editor.Context is set but Content has no Store"},
		{editorWith(webui.LangGo, ctxFiles(ts), true), `the language "go" takes none`},
		{editorWith(webui.LangJSON, ctxFiles(ts), true), `no "json" directory`},
		{editorWith(webui.LangTypeScript, ctxFiles(map[string]string{"typescript/a.d.ts": "", "a.d.ts": ""}), true), `"a.d.ts", which is not the directory of a language`},
		{editorWith(webui.LangTypeScript, ctxFiles(map[string]string{"typescript/tsconfig.json": `{"compilerOptions": {"outDir": "x"}}`}), true), "compilerOptions.outDir is not an option the editor reads"},
		{editorWith(webui.LangJSON, ctxFiles(map[string]string{"json/schema.json": "{"}), true), "json/schema.json is not valid JSON"},
		{editorWith(webui.LangJSON, ctxFiles(map[string]string{"json/other.json": "{}"}), true), "json/ has no schema.json"},
	}
	for _, tc := range cases {
		got := editorErrors(t, tc.body)
		assert.Contains(t, got, tc.want)
		assert.Contains(t, got, "Editor.Context") // each says where it comes from
	}
}

func TestEditorsOnOnePageShareOneProgramSoTheyMayNotDisagreeAboutIt(t *testing.T) {
	t.Parallel()

	a := editorWith(webui.LangTypeScript, ctxFiles(map[string]string{"typescript/env.d.ts": "declare const x: 1;"}), true)
	b := editorWith(webui.LangTypeScript, ctxFiles(map[string]string{"typescript/env.d.ts": "declare const x: 2;"}), true)
	b.Content = webui.String[snippet]{Label: "Other", Load: snippetText.Load, Store: snippetText.Store}
	assert.Contains(t, editorErrors(t, webui.Stack{a, b}), "give typescript/env.d.ts a different text")

	c := editorWith(webui.LangTypeScript, ctxFiles(map[string]string{"typescript/tsconfig.json": `{"compilerOptions": {"strict": true}}`}), true)
	d := editorWith(webui.LangTypeScript, ctxFiles(map[string]string{"typescript/tsconfig.json": `{"compilerOptions": {"strict": false}}`}), true)
	d.Content = b.Content
	assert.Contains(t, editorErrors(t, webui.Stack{c, d}), "different compiler options")

	// The same context twice, and a TypeScript editor beside a JavaScript one, are fine.
	same := ctxFiles(map[string]string{"typescript/env.d.ts": "declare const x: 1;", "javascript/env.d.ts": "declare const y: 2;"})
	e, f := editorWith(webui.LangTypeScript, same, true), editorWith(webui.LangJavaScript, same, true)
	f.Content = b.Content
	_, err := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", Body: webui.Stack{e, f}}}}.Compile("/admin")
	assert.NoError(t, err)
}

func TestASecretThatIsTooShortToKeepAnAddressPrivateIsACompileError(t *testing.T) {
	t.Parallel()

	errs := compileErrors(t, webui.App{Secret: []byte("short"), Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/", Body: webui.Stack{}}}})
	assert.Contains(t, errs[0].Detail, "App.Secret is 5 bytes")
}

func TestWithNoLanguageServiceImportedAnEditorsContextIsNotHandedOver(t *testing.T) {
	t.Parallel()

	// This program imports no service, so nothing would read the context.
	h := editorApp(editorWith(webui.LangTypeScript, ctxFiles(map[string]string{"typescript/env.d.ts": "declare const x: 1;"}), true))
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.False(t, strings.Contains(body, "/_webui/code/"))
	assert.False(t, strings.Contains(body, `"context"`))
}
