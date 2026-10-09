package monaco_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	_ "github.com/Olian04/webui/pkg/monaco/css"
	_ "github.com/Olian04/webui/pkg/monaco/html"
	_ "github.com/Olian04/webui/pkg/monaco/json"
	_ "github.com/Olian04/webui/pkg/monaco/typescript"
	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type doc struct{ Text string }

func editor(language webui.Language) http.Handler {
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", Body: webui.Editor[doc]{
		Title: "Doc", Language: language,
		Content: webui.String[doc]{
			Label: "Text", Load: func(d doc) string { return d.Text }, Store: func(d *doc, v string) { d.Text = v },
		},
		Submit: webui.Action[doc]{Run: func(context.Context, doc) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
		Load:   func(context.Context) (doc, error) { return doc{}, nil },
	}}}}.MustCompile("/admin")
}

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestEachLanguageIsGivenTheServiceThatCoversItAndNoOther(t *testing.T) {
	t.Parallel()

	cases := map[webui.Language]string{
		webui.LangJSON:       "json",
		webui.LangCSS:        "css",
		webui.LangSCSS:       "css",
		webui.LangLess:       "css",
		webui.LangHTML:       "html",
		webui.LangHandlebars: "html",
		webui.LangRazor:      "html",
		webui.LangTypeScript: "ts",
		webui.LangJavaScript: "ts",
	}
	for language, service := range cases {
		h := editor(language)
		body := get(h, "/admin/e").Body.String()
		m := regexp.MustCompile(`"` + service + `":"([^"]+)"`).FindStringSubmatch(body)
		if m == nil {
			t.Errorf("%s: no %s service handed over", language, service)
			continue
		}
		assert.Equal(t, get(h, m[1]).Code, http.StatusOK)
		for _, other := range []string{"json", "css", "html", "ts"} {
			if other != service {
				assert.False(t, regexp.MustCompile(`"`+other+`":"`).MatchString(body))
			}
		}
	}
}

func TestALanguageWithNoServiceIsOnlyHighlighted(t *testing.T) {
	t.Parallel()

	body := get(editor(webui.LangGo), "/admin/e").Body.String()
	for _, service := range []string{"json", "css", "html", "ts"} {
		assert.False(t, regexp.MustCompile(`"`+service+`":"`).MatchString(body))
	}
}
