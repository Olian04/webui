package webui_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type note struct {
	Title string
	Body  string
}

func noteApp(start note) (http.Handler, func() note) {
	var (
		mu    sync.Mutex
		saved = start
	)
	form := webui.Form[note]{
		Load: func(context.Context) (note, error) {
			mu.Lock()
			defer mu.Unlock()
			return saved, nil
		},
		Fields: []webui.Accessor[note]{
			webui.String[note]{Label: "Title", Load: func(n note) string { return n.Title }, Store: func(n *note, v string) { n.Title = v }},
			webui.String[note]{
				Label: "Body", Multiline: true, Placeholder: "Say more", Rules: webui.StringRules{MinLen: 2, MaxLen: 12},
				Load: func(n note) string { return n.Body }, Store: func(n *note, v string) { n.Body = v },
			},
		},
		Submit: webui.Action[note]{Run: func(_ context.Context, n note) (webui.Outcome, error) {
			mu.Lock()
			saved = n
			mu.Unlock()
			return webui.Success("Saved"), nil
		}},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/n", Body: form}}}.MustCompile("/admin")
	return h, func() note {
		mu.Lock()
		defer mu.Unlock()
		return saved
	}
}

func TestAMultilineStringIsATextAreaWithItsRules(t *testing.T) {
	t.Parallel()

	h, _ := noteApp(note{Title: "t", Body: "one\ntwo"})
	body := serve(h, http.MethodGet, "/admin/n").Body.String()

	// The text begins after a newline, which the parser drops, so that a text that begins with one keeps it.
	assert.Contains(t, body, `<textarea id="p-f1" class="input textarea" name="f1" rows="5" placeholder="Say more" minlength="2" maxlength="12"`)
	assert.Contains(t, body, ">\none\ntwo</textarea>")
	assert.False(t, strings.Contains(body, `name="f1" type=`)) // not an input
	assert.False(t, strings.Contains(body, "pattern="))
	assert.Contains(t, body, `data-msg-too-long="Must be at most 12 characters."`) // the browser's words
}

func TestALineBreakIsOneCharacterWhateverTheBrowserSent(t *testing.T) {
	t.Parallel()

	h, saved := noteApp(note{Title: "t", Body: "x"})
	// A browser sends "ab\r\ncd\r\nef" for three lines: ten characters, and as it counts them, eight.
	rec := post(h, "/admin/n", formValues("f0", "t", "f1", "aaaa\r\nbbbb\r\ncc"))
	assert.Equal(t, rec.Code, http.StatusSeeOther) // 12 as the browser counts them, 14 as sent
	assert.Equal(t, saved().Body, "aaaa\nbbbb\ncc")

	rec = post(h, "/admin/n", formValues("f0", "t", "f1", "aaaa\r\nbbbb\r\nccc"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Must be at most 12 characters.")
	assert.Contains(t, rec.Body.String(), ">\naaaa\r\nbbbb\r\nccc</textarea>") // what was typed comes back as it was
}

func TestAMultilineTextReadOnlyAndInATableKeepsItsLines(t *testing.T) {
	t.Parallel()

	body := webui.String[note]{Label: "Body", Multiline: true, Load: func(n note) string { return n.Body }}
	notes := func(context.Context) ([]note, error) { return []note{{Title: "a", Body: "one\ntwo\nthree\nfour"}}, nil }
	h := webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/t", Body: webui.Table[note]{
			Title: "Notes", Rows: notes, Export: true,
			Columns: []webui.Accessor[note]{webui.String[note]{Label: "Title", Load: func(n note) string { return n.Title }}, body},
		}},
		webui.Page[webui.NoArgs]{Path: "/f", Body: webui.Form[note]{
			Load:   func(context.Context) (note, error) { return note{Body: "one\ntwo"}, nil },
			Fields: []webui.Accessor[note]{body},
		}},
	}}.MustCompile("/admin")

	assert.Contains(t, serve(h, http.MethodGet, "/admin/t").Body.String(), "<span class=\"multiline\">one\ntwo\nthree\nfour</span>")
	assert.Contains(t, serve(h, http.MethodGet, "/admin/f").Body.String(), `<div class="static"><span class="multiline">one`+"\n"+`two</span></div>`)
	// The file holds every line, quoted, in one cell.
	csv := serve(h, http.MethodGet, "/admin/t?notes.export=csv").Body.String()
	assert.Contains(t, csv, "\"one\ntwo\nthree\nfour\"")
	// A filter finds it by any of them.
	assert.Contains(t, serve(h, http.MethodGet, "/admin/t?notes.filter.body=three").Body.String(), `<td class="">a</td>`)
}

func TestAMultilineStringIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	load := func(n note) string { return n.Body }
	store := func(*note, string) {}
	problems := func(body webui.PageBody) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/n", Body: body}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	form := func(s webui.String[note]) webui.PageBody {
		return webui.Form[note]{
			Load:   func(context.Context) (note, error) { return note{}, nil },
			Fields: []webui.Accessor[note]{s},
			Submit: webui.Action[note]{Run: func(context.Context, note) (webui.Outcome, error) { return webui.Outcome{}, nil }},
		}
	}
	assert.Contains(t, problems(form(webui.String[note]{Label: "Body", Multiline: true, Load: load, Store: store,
		Rules: webui.StringRules{Pattern: &webui.PatternRule{Expr: `[a-z]+`}}})), "is Multiline and has a Pattern")
	assert.Contains(t, problems(form(webui.String[note]{Label: "Body", Multiline: true, Load: load, Store: store,
		Rules: webui.StringRules{Contains: "x"}})), "is Multiline and has a text rule")
	assert.Contains(t, problems(webui.Toolbar[webui.NoArgs]{Left: []webui.ToolbarItem[webui.NoArgs]{
		webui.String[webui.NoArgs]{Label: "Body", Multiline: true, Load: func(webui.NoArgs) string { return "" }, Store: func(*webui.NoArgs, string) {}},
	}}), "is Multiline in a toolbar")
}
