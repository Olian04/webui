package main

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func TestTheConfigPageEditsATextAndShowsWhatChangedSinceTheDeploy(t *testing.T) {
	h := handler(t)
	p := pageAt(t, h, "/admin/config")
	box, _ := p.Field("f0")
	assert.Equal(t, box.Type, "textarea")
	assert.True(t, len(p.ByAttr("data-language", "json")) > 0)
	assert.True(t, shown(p, "No differences"))

	rec := post(h, "/admin/config", url.Values{"_view": {"p.0"}, "f0": {"{\n  \"collector\": \"eu-west-2\"\n}\n"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	changed := pageAt(t, h, "/admin/config")
	assert.False(t, shown(changed, "No differences"))
	assert.True(t, strings.Contains(changed.Root().AllText(), "eu-west-2"))

	// A viewer is shown the text and is not let save it.
	v := handlerAs(t, Viewer)
	read := pageAt(t, v, "/admin/config")
	_, editable := read.Field("f0")
	assert.False(t, editable)
	assert.True(t, strings.Contains(read.Root().AllText(), "collector"))
	refused := post(v, "/admin/config", url.Values{"_view": {"p.0"}, "f0": {"{}"}})
	assert.Equal(t, refused.Code, http.StatusSeeOther)
	told := toasts(t, follow(v, refused).Body.String())
	assert.Equal(t, len(told), 1)
	assert.Equal(t, told[0].Role, "alert")
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
	rule := page.Parse(t, body)
	assert.Equal(t, len(rule.ByAttr("data-language", "typescript")), 1)
	box, _ := rule.Field("f0")
	assert.Equal(t, box.Type, "textarea")
	assert.Contains(t, workers(editor), `"ts":"/admin/_webui/monaco/workers/ts.worker-`) // the service the program imported

	// The same text is read by a viewer, who is sent no service: nothing there can be changed.
	viewer := handlerAs(t, Viewer)
	_, editable := pageAt(t, viewer, "/admin/rule").Field("f0")
	assert.False(t, editable)
	assert.False(t, strings.Contains(workers(viewer), `"ts"`))

	// The text has type errors on purpose, and saving it is the editor's.
	assert.Contains(t, body, "pageOnCall(signal, ")
	rec := post(editor, "/admin/rule", url.Values{"_view": {"p"}, "f0": {"export const ok: number = 1;\n"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	saved, _ := pageAt(t, editor, "/admin/rule").Field("f0")
	assert.Equal(t, saved.Value, "export const ok: number = 1;\n")
	refused := post(viewer, "/admin/rule", url.Values{"_view": {"p"}, "f0": {"x"}})
	assert.Equal(t, refused.Code, http.StatusSeeOther)
	told := toasts(t, follow(viewer, refused).Body.String())
	assert.Equal(t, len(told), 1)
	assert.Equal(t, told[0].Role, "alert")
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
