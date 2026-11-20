package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/Olian04/webui/pkg/codeeditor/lang"
	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// headShop is a page of one panel whose head has the buttons and the link that every kind of panel may be given.
type headShop struct {
	h    http.Handler
	mu   sync.Mutex
	ran  int
	deny error
}

func headKinds(head []webui.ToolbarItem) map[string]webui.View {
	src := &calendarSource{}
	devices := webui.RowsLoader[Device]{Load: func(context.Context) ([]Device, webui.Loaded) { return []Device{{Id: "a"}}, webui.Ready() }}
	id := webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}
	return map[string]webui.View{
		"Table":    webui.Table[Device]{Title: "Things", HeadActions: head, Loader: devices, Columns: []webui.Accessor[Device]{id}},
		"List":     webui.List[Device]{Title: "Things", HeadActions: head, Loader: devices, Headline: id},
		"Cards":    webui.Cards[Device]{Title: "Things", HeadActions: head, Loader: devices, Headline: id},
		"Form":     webui.Form[snippet]{Title: "Things", HeadActions: head, Load: loadSnippet, Fields: []webui.Accessor[snippet]{snippetView}},
		"Editor":   webui.Editor[snippet]{Title: "Things", HeadActions: head, Load: loadSnippet, Content: snippetView, Language: lang.JSON},
		"Diff":     webui.Diff[snippet]{Title: "Things", HeadActions: head, Load: loadSnippet, Original: snippetOld, Modified: snippetView},
		"Markdown": webui.Markdown[handbook]{Title: "Things", HeadActions: head, Load: func(context.Context) (handbook, webui.Loaded) { return handbook{Text: "hi"}, webui.Ready() }, Content: handbookText},
		"Calendar": webui.Calendar[release]{Title: "Things", HeadActions: head, Load: src.load, Date: releaseWhen, Today: midSeptember},
		"FileUpload": webui.FileUpload[struct{}]{
			Title: "Things", HeadActions: head, MaxSize: 1,
			Receive: webui.Action[webui.FileHandle[struct{}]]{Run: func(context.Context, webui.FileHandle[struct{}]) (webui.Outcome, error) {
				return webui.Success("Up"), nil
			}},
		},
	}
}

func newHeadShop(kind string, confirm string) *headShop {
	s := &headShop{}
	head := []webui.ToolbarItem{
		webui.Action[webui.NoArgs]{
			Label: "Rescan", Confirm: confirm,
			Guard: func(context.Context, webui.NoArgs) error { return s.deny },
			Run: func(context.Context, webui.NoArgs) (webui.Outcome, error) {
				s.mu.Lock()
				s.ran++
				s.mu.Unlock()
				return webui.Success("Scanned"), nil
			},
		},
		webui.Link[webui.NoArgs, newArgs]{Label: "Add one", Page: newDevice, Args: func(context.Context, webui.NoArgs) newArgs { return newArgs{From: "x"} }},
	}
	s.h = webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", View: headKinds(head)[kind]}, newDevice}}.MustCompile("/admin")
	return s
}

var headKindNames = []string{"Table", "List", "Cards", "Form", "Editor", "Diff", "Markdown", "Calendar", "FileUpload"}

// Whatever it shows, a panel has the buttons and links that were declared for it in its head, beside its title.
func TestAPanelOfAnyKindHasTheControlsThatWereDeclaredForItInItsHead(t *testing.T) {
	t.Parallel()

	for _, kind := range headKindNames {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			s := newHeadShop(kind, "")
			panel := page.One(t, "the panel", pageOf(t, s.h, "/admin/e").Views())
			assert.DeepEqual(t, controls(panel), []control{{"Rescan", "head:0", false}})
			assert.True(t, hasLinkIn(panel, "Add one", "/admin/device/new?from=x"))
			// They come after the title, in the head, and not in the body.
			title := page.One(t, "the title", panel.ByRole("heading", "Things"))
			assert.True(t, title.Before(panel.ByAttr("name", "_act")[0]))
		})
	}
}

func hasLinkIn(e page.El, name, href string) bool {
	for _, l := range e.Links() {
		if l.Name == name && l.Href == href {
			return true
		}
	}
	return false
}

// A button of the head posts to the panel by its position, and is done as an action of a toolbar is.
func TestAButtonInAPanelsHeadIsDoneWhateverThePanelIs(t *testing.T) {
	t.Parallel()

	for _, kind := range headKindNames {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			s := newHeadShop(kind, "")
			rec := post(s.h, "/admin/e", url.Values{"_view": {"p"}, "_act": {"head:0"}})
			assert.Equal(t, rec.Code, http.StatusSeeOther)
			assert.Equal(t, flashOf(rec), "oScanned")
			assert.Equal(t, s.ran, 1)

			// A position that is not a button, in the head or past it, is not understood and nothing is done.
			for _, act := range []string{"head:1", "head:2", "head:-1", "head:x", "head:"} {
				rec = post(s.h, "/admin/e", url.Values{"_view": {"p"}, "_act": {act}})
				assert.Equal(t, flashOf(rec), "eThat request was not understood.")
			}
			assert.Equal(t, s.ran, 1)
		})
	}
}

// A panel that was given no controls for its head has none to be asked for.
func TestAPanelWithNoHeadControlsDoesNoHeadAction(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Markdown[handbook]{Title: "Things", Load: func(context.Context) (handbook, webui.Loaded) { return handbook{Text: "hi"}, webui.Ready() }, Content: handbookText})
	rec := post(h, "/admin/e", url.Values{"_view": {"p"}, "_act": {"head:0"}})
	assert.Equal(t, rec.Code, http.StatusMethodNotAllowed) // a page that has nothing to post does not take a post
	assert.Equal(t, len(pageOf(t, h, "/admin/e").ByAttr("name", "_act")), 0)
}

// The Guard that leaves a button out is the one that refuses it, and the one that draws it disabled.
func TestAHeadButtonThatIsGuardedIsLeftOutOrDisabledAndRefused(t *testing.T) {
	t.Parallel()

	s := newHeadShop("Markdown", "")
	s.deny = errors.New("requires the editor role")
	p := pageOf(t, s.h, "/admin/e")
	assert.Equal(t, len(controls(p.Root())), 0)
	assert.False(t, strings.Contains(p.Root().AllText(), "requires the editor role"))
	assert.True(t, hasLink(p, "Add one", "/admin/device/new?from=x")) // a link is its own

	rec := post(s.h, "/admin/e", url.Values{"_view": {"p"}, "_act": {"head:0"}})
	assert.Equal(t, flashOf(rec), "erequires the editor role")
	assert.Equal(t, s.ran, 0)

	s.deny = webui.Disabled("requires the editor role")
	assert.DeepEqual(t, controls(pageOf(t, s.h, "/admin/e").Root()), []control{{"Rescan", "head:0", true}})
	assert.True(t, strings.Contains(pageOf(t, s.h, "/admin/e").Root().AllText(), "Not available: requires the editor role"))
}

// A head button that asks first is asked of in a dialog of its own, as a row's is.
func TestAHeadButtonMayAskBeforeItIsDone(t *testing.T) {
	t.Parallel()

	s := newHeadShop("Table", "Scan all of them?")
	assert.True(t, strings.Contains(pageOf(t, s.h, "/admin/e").Root().AllText(), "Scan all of them?"))
}

// A Form is not a control of a head, and what is not a button or a link does not compile.
func TestTheHeadOfAPanelTakesAnActionOrALinkAndNothingElse(t *testing.T) {
	t.Parallel()

	form := webui.Form[toolState]{Title: "State", Load: loadToolState, Fields: []webui.Accessor[toolState]{minutesInput}}
	got := editorErrors(t, webui.Markdown[handbook]{Title: "x", HeadActions: []webui.ToolbarItem{form}, Load: func(context.Context) (handbook, webui.Loaded) { return handbook{}, webui.Ready() }, Content: handbookText})
	assert.True(t, strings.Contains(got, "Markdown.HeadActions[0] is a Form"))

	got = editorErrors(t, webui.Markdown[handbook]{Title: "x", HeadActions: []webui.ToolbarItem{webui.Action[webui.NoArgs]{Label: "Do"}}, Load: func(context.Context) (handbook, webui.Loaded) { return handbook{}, webui.Ready() }, Content: handbookText})
	assert.True(t, strings.Contains(got, "Markdown.HeadActions[0]"))
}
