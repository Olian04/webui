package webui_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// Every response that leaves a visitor somewhere has a way on, written by the server, in the page that they were sent to: the states that were a line of plain text or a
// message with nothing to follow are HTML with the link that the state offers.

func TestAMethodThatAnAddressDoesNotTakeIsAnswered405InThePageWithALink(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	rec := serve(h, http.MethodDelete, "/admin/device")
	assert.Equal(t, rec.Code, http.StatusMethodNotAllowed)
	assert.True(t, rec.Header().Get("Allow") != "")
	body := rec.Body.String()
	assert.Contains(t, body, "That is not how this address is used")
	assert.Contains(t, body, "It answers "+rec.Header().Get("Allow"))
	assert.Contains(t, body, `href="/admin/device"`) // opened as a visit, which it answers
	assert.Contains(t, body, `class="sidebar"`)      // in the shell, so that the way back is on screen
}

func TestACrossOriginPostIsRefusedInThePageAndAsAFragmentForScript(t *testing.T) {
	t.Parallel()

	s := newShop()
	form := formValues("f1", "10.0.0.9", "f2", "5")
	cross := func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }

	page := post(s.h, "/admin/device/dev1", form, cross)
	assert.Equal(t, page.Code, http.StatusForbidden)
	assert.Contains(t, page.Body.String(), "That request came from another site, and was refused.")
	assert.Contains(t, page.Body.String(), `href="/admin/device/dev1"`)

	fragment := post(s.h, "/admin/device/dev1", form, cross, func(r *http.Request) { r.Header.Set("X-Webui-Leaf", "p") })
	assert.Equal(t, fragment.Code, http.StatusForbidden)
	assert.Contains(t, fragment.Body.String(), `<li class="result" data-tone="bad">`)
	assert.Contains(t, fragment.Body.String(), "came from another site")
	assert.Equal(t, len(s.saved), 0)
}

func TestTheAddressIsNotValidPageLeadsOnToThePageWithoutItsArguments(t *testing.T) {
	t.Parallel()

	a, _, _ := app(nil)
	h := a.MustCompile("/admin")
	rec := serve(h, http.MethodGet, "/admin/device/abc?n=lots")
	assert.Equal(t, rec.Code, http.StatusBadRequest)
	body := rec.Body.String()
	assert.Contains(t, body, "One of the arguments in it could not be read.")
	assert.False(t, strings.Contains(body, "Remove the arguments")) // a visitor is not told to edit the address
	assert.Contains(t, body, `href="/admin/device/abc">`)
	assert.Contains(t, body, "Open the page without them")

	// With no query there is nothing to drop, and no link that leads to the same refusal.
	none := serve(h, http.MethodGet, "/admin/device/abc/x")
	assert.False(t, strings.Contains(none.Body.String(), "Open the page without them"))
}

func TestAFailedPanelLeadsOnToTryAgain(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(func(context.Context, webui.Query) (webui.Window[Device], error) {
		return webui.Window[Device]{}, errors.New("down")
	})
	body := serve(h, http.MethodGet, "/admin/device?q=x").Body.String()
	assert.Contains(t, body, "Could not load")
	assert.Contains(t, body, `href="/admin/device?q=x"`)
	assert.Contains(t, body, "Try again")
	assert.False(t, strings.Contains(body, "Refresh to try again"))
}

func TestATableThatShowsNothingLeadsOnToItsFirstPageAndToItselfWithoutFilters(t *testing.T) {
	t.Parallel()

	// Past the end of the rows there are: the way on is the first page.
	h, _ := tableApp(func(context.Context, webui.Query) (webui.Window[Device], error) {
		return webui.Window[Device]{Total: 25}, nil
	})
	past := serve(h, http.MethodGet, "/admin/device?devices.offset=100").Body.String()
	assert.Contains(t, past, "This page is past the end.")
	assert.Contains(t, past, "Go to the first page")
	assert.Contains(t, past, `href="/admin/device"`)
	assert.False(t, strings.Contains(past, "Clear the filters"))

	// Filtered to nothing: the way on is the table without its filters, and the page that it was on is dropped with them.
	empty, _ := tableApp(func(context.Context, webui.Query) (webui.Window[Device], error) { return webui.Window[Device]{}, nil })
	filtered := serve(empty, http.MethodGet, "/admin/device?devices.filter.id=zzz").Body.String()
	assert.Contains(t, filtered, "Clear the filters")
	assert.Contains(t, filtered, `href="/admin/device"`)

	// Nothing and nothing set: nothing to offer.
	bare := serve(empty, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, bare, "Nothing to show")
	assert.False(t, strings.Contains(bare, "Clear the filters"))
	assert.False(t, strings.Contains(bare, "Go to the first page"))
}

// The refusal of a page, with its link on, is what a script's post to it is answered with, whatever the status of it is.
func TestARefusedPostOfScriptsKeepsTheWordsAndTheLinkOfTheRefusal(t *testing.T) {
	t.Parallel()

	back := webui.PageID[webui.NoArgs]("/records")
	list := webui.Page[webui.NoArgs]{Path: back, Nav: webui.Nav{Label: "Records"}, Body: webui.Stack{}}
	one := webui.Page[recordArgs]{
		Path: "/records/{id}",
		Guard: func(ctx context.Context, a recordArgs) error {
			return webui.NotFound("There is no record "+a.ID+".").WithLink("Back to the records", webui.Open(ctx, back, webui.NoArgs{}))
		},
		Body: webui.FileUpload[struct{}]{MaxSize: 1, Receive: webui.Action[webui.FileHandle[struct{}]]{
			Run: func(context.Context, webui.FileHandle[struct{}]) (webui.Outcome, error) {
				return webui.Success("ok"), nil
			},
		}},
	}
	h := webui.App{Pages: webui.Pages{list, one}}.MustCompile("/admin")
	rec := post(h, "/admin/records/x", nil, func(r *http.Request) { r.Header.Set("X-Webui-Leaf", "p") })
	assert.Equal(t, rec.Code, http.StatusNotFound)
	assert.Contains(t, rec.Body.String(), "There is no record x.")
	assert.Contains(t, rec.Body.String(), `<a class="btn btn-ghost btn-sm result-next" href="/admin/records">Continue</a>`)
}

// A parent in the breadcrumb that the visitor may not open is its words and no link, and Cancel is not drawn to lead to it: neither leads them to a 403 that nothing on
// the page had said.
func TestABreadcrumbParentAndACancelThatLeadToARefusedPageAreLeftOut(t *testing.T) {
	t.Parallel()

	build := func(allowed bool) string {
		parent := webui.Page[webui.NoArgs]{
			Path: "/a", Body: webui.Stack{},
			Guard: func(context.Context, webui.NoArgs) error {
				if allowed {
					return nil
				}
				return errors.New("no")
			},
		}
		child := webui.Page[webui.NoArgs]{Path: "/a/b", Body: webui.Form[struct{}]{
			Load:   func(context.Context) (struct{}, error) { return struct{}{}, nil },
			Submit: webui.Action[struct{}]{Run: func(context.Context, struct{}) (webui.Outcome, error) { return webui.Success("ok"), nil }},
		}}
		h := webui.App{Pages: webui.Pages{parent, child}}.MustCompile("/admin")
		return serve(h, http.MethodGet, "/admin/a/b").Body.String()
	}
	open := build(true)
	assert.Contains(t, open, `href="/admin/a"`)
	assert.Contains(t, open, "Cancel")

	shut := build(false)
	assert.False(t, strings.Contains(shut, `href="/admin/a"`))
	assert.False(t, strings.Contains(shut, ">Cancel<") || strings.Contains(shut, "Cancel\n"))
	assert.Contains(t, shut, `aria-current="page"`) // the page itself is still named in the crumbs
}

// A form that is only read, because its Guard said Disabled, says why in words in its footer.
func TestAFormThatIsOnlyReadSaysWhyInWords(t *testing.T) {
	t.Parallel()

	page := func(guard func(context.Context, struct{}) error) string {
		h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/f", Body: webui.Form[struct{}]{
			Load:   func(context.Context) (struct{}, error) { return struct{}{}, nil },
			Submit: webui.Action[struct{}]{Guard: guard, Run: func(context.Context, struct{}) (webui.Outcome, error) { return webui.Success("ok"), nil }},
		}}}}.MustCompile("/admin")
		return serve(h, http.MethodGet, "/admin/f").Body.String()
	}
	said := page(func(context.Context, struct{}) error { return webui.Disabled("Locked by the release") })
	assert.Contains(t, said, `<span class="panel-note dim">Locked by the release</span>`)
	silent := page(func(context.Context, struct{}) error { return errors.New("no") })
	assert.False(t, strings.Contains(silent, "panel-note"))
}

// The contract for a post that script makes, for any panel: it carries the header of the leaf, and is answered in HTML with a status that says how it went. An accepted
// action is the item with a 200, a refused one the item with a 422, a Guard's refusal the item with a 403 and an error the item with a 500; a form that was refused is
// the form itself, drawn again, with the 422. None of it is a redirect, a cookie or data.
func TestAPostThatScriptMakesIsAnsweredInHTMLWithTheStatusOfHowItWent(t *testing.T) {
	t.Parallel()

	leaf := func(r *http.Request) { r.Header.Set("X-Webui-Leaf", "p") }

	s := newShop()
	ok := post(s.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"), leaf)
	assert.Equal(t, ok.Code, http.StatusOK)
	assert.Equal(t, ok.Header().Get("Location"), "")
	assert.Equal(t, flashOf(ok), "")
	assert.Contains(t, ok.Body.String(), `<li class="result" data-tone="ok">`)

	// Refused by a rule: the form, drawn again with what was typed and what was refused, and nothing round it.
	refused := post(s.h, "/admin/device/dev1", formValues("f1", "nope", "f2", "5"), leaf)
	assert.Equal(t, refused.Code, http.StatusUnprocessableEntity)
	body := refused.Body.String()
	assert.Contains(t, body, "must be a valid IPv4 address")
	assert.Contains(t, body, `value="nope"`)
	assert.False(t, strings.Contains(body, "<!doctype") || strings.Contains(body, `class="sidebar"`))
	assert.Contains(t, body, `data-leaf="p"`)

	// The same post without the header is as it always was: a page, with the 422.
	page := post(s.h, "/admin/device/dev1", formValues("f1", "nope", "f2", "5"))
	assert.Equal(t, page.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, page.Body.String(), `class="sidebar"`)

	// A Guard that refuses the visitor is a 403, which is not the answer to what was sent being refused.
	guarded := newUploadShop(func(f *webui.FileUpload[struct{}]) {
		f.Receive.Guard = func(context.Context, webui.FileHandle[struct{}]) error { return errors.New("requires the editor role") }
	})
	denied := postFragment(guarded.h, "a.bin", "data")
	assert.Equal(t, denied.Code, http.StatusForbidden)
	assert.Contains(t, denied.Body.String(), `data-tone="bad"><span class="result-text">requires the editor role</span>`)
}
