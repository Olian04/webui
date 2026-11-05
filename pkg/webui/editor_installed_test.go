package webui

import (
	"context"
	"strings"
	"testing"

	"github.com/Olian04/webui/internal/render/assets"
	"github.com/Olian04/webui/test/util/assert"
)

// An Editor and a Diff are drawn with the code editor, which a program imports to have, so that one without either does not link it: this test binary has not imported
// it, and the app is refused when it is compiled, with the import as the fix.
func TestAnEditorOrADiffWithoutTheCodeEditorIsACompileError(t *testing.T) {
	t.Parallel()

	assert.False(t, assets.EditorInstalled())
	type doc struct{ Text string }
	text := String[doc]{Label: "Text", Load: func(d doc) string { return d.Text }}
	load := func(context.Context) (doc, Loaded) { return doc{}, Ready() }
	for name, body := range map[string]PageBody{
		"an Editor": Editor[doc]{Title: "Doc", Load: load, Content: text},
		"a Diff":    Diff[doc]{Title: "Doc", Load: load, Original: text, Modified: String[doc]{Label: "New", Load: text.Load}},
	} {
		_, err := App{Pages: Pages{Page[NoArgs]{Path: "/e", Body: body}}}.Compile("/admin")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), name+" is drawn with the code editor, which this program does not carry")
		assert.True(t, strings.Contains(err.Error(), `import _ "github.com/Olian04/webui/pkg/monaco"`))
	}
}
