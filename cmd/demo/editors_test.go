package main

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestTheConfigPageEditsATextAndShowsWhatChangedSinceTheDeploy(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/config").Body.String()
	assert.Contains(t, body, `<textarea class="code-text"`)
	assert.Contains(t, body, `data-language="json"`)
	assert.Contains(t, body, "No differences")

	rec := post(h, "/admin/config", url.Values{"_leaf": {"p.0"}, "f0": {"{\n  \"collector\": \"eu-west-2\"\n}\n"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	body = get(h, "/admin/config").Body.String()
	assert.False(t, strings.Contains(body, "No differences"))
	assert.Contains(t, body, "eu-west-2")

	// A viewer is shown the text and is not let save it.
	v := handlerAs(t, Viewer)
	body = get(v, "/admin/config").Body.String()
	assert.Contains(t, body, `<pre class="code-view"`)
	assert.False(t, strings.Contains(body, "<textarea"))
	refused := post(v, "/admin/config", url.Values{"_leaf": {"p.0"}, "f0": {"{}"}})
	assert.Equal(t, refused.Code, http.StatusSeeOther)
	assert.Contains(t, follow(v, refused).Body.String(), `class="toast err"`)
}

func TestThePagingRuleIsTypeScriptWithItsLanguageServiceForAnEditorAndNotForAViewer(t *testing.T) {
	workers := func(h http.Handler) string {
		body := get(h, "/admin/rule").Body.String()
		i := strings.Index(body, `<script id="webui-code"`)
		assert.True(t, i >= 0)
		return body[i : i+strings.Index(body[i:], "</script>")]
	}
	editor := handler(t)
	body := get(editor, "/admin/rule").Body.String()
	assert.Contains(t, body, `data-language="typescript"`)
	assert.Contains(t, body, `<textarea class="code-text"`)
	assert.Contains(t, workers(editor), `"ts":"/admin/_webui/monaco/workers/ts.worker-`) // the service the program imported

	// The same text is read by a viewer, who is sent no service: nothing there can be changed.
	viewer := handlerAs(t, Viewer)
	assert.Contains(t, get(viewer, "/admin/rule").Body.String(), `<pre class="code-view"`)
	assert.False(t, strings.Contains(workers(viewer), `"ts"`))

	// The text has type errors on purpose, and saving it is the editor's.
	assert.Contains(t, body, "pageOnCall(signal, ")
	rec := post(editor, "/admin/rule", url.Values{"_leaf": {"p"}, "f0": {"export const ok: number = 1;\n"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Contains(t, get(editor, "/admin/rule").Body.String(), "export const ok: number = 1;")
	refused := post(viewer, "/admin/rule", url.Values{"_leaf": {"p"}, "f0": {"x"}})
	assert.Equal(t, refused.Code, http.StatusSeeOther)
	assert.Contains(t, follow(viewer, refused).Body.String(), `class="toast err"`)
}

func TestTheEditorsAreGivenTheFolderOfContextTheProgramEmbeds(t *testing.T) {
	// One folder serves both editors, each reading the directory named for its own language.
	editor := handler(t)
	rule := get(editor, "/admin/rule").Body.String()
	m := regexp.MustCompile(`"libs":\{"typescript":\[\{"path":"/rule\.d\.ts","url":"([^"]+)"\}\]`).FindStringSubmatch(rule)
	assert.NotNil(t, m)
	assert.Contains(t, rule, `"compiler":{"typescript":{`)
	file := get(editor, m[1])
	assert.Equal(t, file.Code, http.StatusOK)
	assert.Contains(t, file.Body.String(), "declare function pageOnCall(")

	config := get(editor, "/admin/config").Body.String()
	assert.Contains(t, config, `"schemas":{`)
	assert.Contains(t, config, "data-context=")

	// A viewer is shown the text and handed none of it.
	viewer := handlerAs(t, Viewer)
	for _, page := range []string{"/admin/rule", "/admin/config"} {
		body := get(viewer, page).Body.String()
		assert.False(t, strings.Contains(body, "/_webui/code/"))
	}
}
