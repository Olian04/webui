package webui_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// Every response that leaves a visitor somewhere has a way on, written by the server, in the page that they were sent to: the states that were a line of plain text or a
// message with nothing to follow are HTML with the link that the state offers.

func TestAMethodThatAnAddressDoesNotTakeIsAnswered405InThePageWithALink(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	rec := serve(h, http.MethodDelete, "/admin/device")
	assert.Equal(t, rec.Code, http.StatusMethodNotAllowed)
	assert.True(t, rec.Header().Get("Allow") != "")
	p := page.Parse(t, rec.Body.String())
	text := p.Root().Text()
	assert.True(t, strings.Contains(text, "That is not how this address is used"))
	assert.True(t, strings.Contains(text, "It answers "+rec.Header().Get("Allow")))
	assert.True(t, len(linksTo(p, "/admin/device")) > 0) // opened as a visit, which it answers
	sidebarNav(t, p)                                     // in the shell, so that the way back is on screen
}

func TestACrossOriginPostIsRefusedInThePageAndAsAFragmentForScript(t *testing.T) {
	t.Parallel()

	s := newShop()
	form := formValues("f1", "10.0.0.9", "f2", "5")
	cross := func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }

	whole := post(s.h, "/admin/device/dev1", form, cross)
	assert.Equal(t, whole.Code, http.StatusForbidden)
	refused := page.Parse(t, whole.Body.String())
	assert.True(t, strings.Contains(refused.Root().Text(), "That request came from another site, and was refused."))
	assert.True(t, len(linksTo(refused, "/admin/device/dev1")) > 0)

	fragment := post(s.h, "/admin/device/dev1", form, cross, func(r *http.Request) { r.Header.Set("X-Webui-View", "p") })
	assert.Equal(t, fragment.Code, http.StatusForbidden)
	told := items(page.Parse(t, fragment.Body.String()))
	assert.Equal(t, len(told), 1)
	assert.True(t, strings.HasPrefix(told[0], "Failed: ") && strings.Contains(told[0], "came from another site"))
	assert.Equal(t, len(s.saved), 0)
}

func TestTheAddressIsNotValidPageLeadsOnToThePageWithoutItsArguments(t *testing.T) {
	t.Parallel()

	a, _, _ := app(nil)
	h := a.MustCompile("/admin")
	rec := serve(h, http.MethodGet, "/admin/device/abc?n=lots")
	assert.Equal(t, rec.Code, http.StatusBadRequest)
	p := page.Parse(t, rec.Body.String())
	assert.True(t, strings.Contains(p.Root().Text(), "One of the arguments in it could not be read."))
	assert.False(t, strings.Contains(p.Root().AllText(), "Remove the arguments")) // a visitor is not told to edit the address
	assert.True(t, hasLink(p, "Open the page without them", "/admin/device/abc"))

	// With no query there is nothing to drop, and no link that leads to the same refusal.
	none := serve(h, http.MethodGet, "/admin/device/abc/x")
	assert.False(t, strings.Contains(page.Parse(t, none.Body.String()).Root().AllText(), "Open the page without them"))
}

func TestAFailedPanelLeadsOnToTryAgain(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
		return webui.PagedWindow[Device]{}, webui.Failed(errors.New("down"))
	})
	p := pageOf(t, h, "/admin/device?q=x")
	assert.Equal(t, len(p.ByText("Could not load")), 1)
	assert.True(t, hasLink(p, "Try again", "/admin/device?q=x"))
	assert.False(t, strings.Contains(p.Root().AllText(), "Refresh to try again"))
}

func TestATableThatShowsNothingLeadsOnToItsFirstPageAndToItselfWithoutFilters(t *testing.T) {
	t.Parallel()

	// Past the end of the rows there are: the way on is the first page.
	h, _ := tableApp(func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
		return webui.PagedWindow[Device]{Total: 25}, webui.Ready()
	})
	past := pageOf(t, h, "/admin/device?devices.offset=100")
	assert.Equal(t, len(past.ByText("This page is past the end.")), 1)
	assert.True(t, hasLink(past, "Go to the first page", "/admin/device"))
	assert.Equal(t, named(past.Links(), "Clear the filters"), 0)

	// Filtered to nothing: the way on is the table without its filters, and the page that it was on is dropped with them.
	empty, _ := tableApp(func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
		return webui.PagedWindow[Device]{}, webui.Ready()
	})
	filtered := pageOf(t, empty, "/admin/device?devices.filter.id=zzz")
	assert.True(t, hasLink(filtered, "Clear the filters", "/admin/device"))

	// Nothing and nothing set: nothing to offer.
	bare := pageOf(t, empty, "/admin/device")
	assert.Equal(t, len(bare.ByText("Nothing to show")), 1)
	assert.Equal(t, named(bare.Links(), "Clear the filters"), 0)
	assert.Equal(t, named(bare.Links(), "Go to the first page"), 0)
}

// The refusal of a page, with its link on, is what a script's post to it is answered with, whatever the status of it is.
func TestARefusedPostOfScriptsKeepsTheWordsAndTheLinkOfTheRefusal(t *testing.T) {
	t.Parallel()

	back := webui.PageID[webui.NoArgs]("/records")
	list := webui.Page[webui.NoArgs]{Path: back, Nav: webui.Nav{Label: "Records"}, View: webui.Stack{}}
	one := webui.Page[recordArgs]{
		Path: "/records/{id}",
		Guard: func(ctx context.Context, a recordArgs) error {
			return webui.NotFound("There is no record "+a.ID+".").WithLink("Back to the records", webui.Open(ctx, back, webui.NoArgs{}))
		},
		View: webui.FileUpload[struct{}]{MaxSize: 1, Receive: webui.Action[webui.FileHandle[struct{}]]{
			Run: func(context.Context, webui.FileHandle[struct{}]) (webui.Outcome, error) {
				return webui.Success("ok"), nil
			},
		}},
	}
	h := webui.App{Pages: webui.Pages{list, one}}.MustCompile("/admin")
	rec := post(h, "/admin/records/x", nil, func(r *http.Request) { r.Header.Set("X-Webui-View", "p") })
	assert.Equal(t, rec.Code, http.StatusNotFound)
	told := page.Parse(t, rec.Body.String())
	assert.DeepEqual(t, items(told), []string{"Failed: There is no record x. Continue"})
	assert.True(t, hasLink(told, "Continue", "/admin/records"))
}

// A parent in the breadcrumb that the visitor may not open is its words and no link, and Cancel is not drawn to lead to it: neither leads them to a 403 that nothing on
// the page had said.
func TestABreadcrumbParentAndACancelThatLeadToARefusedPageAreLeftOut(t *testing.T) {
	t.Parallel()

	build := func(allowed bool) *page.Page {
		parent := webui.Page[webui.NoArgs]{
			Path: "/a", View: webui.Stack{},
			Guard: func(context.Context, webui.NoArgs) error {
				if allowed {
					return nil
				}
				return errors.New("no")
			},
		}
		child := webui.Page[webui.NoArgs]{Path: "/a/b", View: webui.Form[struct{}]{
			Load:   func(context.Context) (struct{}, webui.Loaded) { return struct{}{}, webui.Ready() },
			Submit: webui.Action[struct{}]{Run: func(context.Context, struct{}) (webui.Outcome, error) { return webui.Success("ok"), nil }},
		}}
		h := webui.App{Pages: webui.Pages{parent, child}}.MustCompile("/admin")
		return pageOf(t, h, "/admin/a/b")
	}
	open := build(true)
	assert.True(t, len(linksTo(open, "/admin/a")) > 0)
	assert.True(t, hasLink(open, "Cancel", "/admin/a"))

	shut := build(false)
	assert.Equal(t, len(linksTo(shut, "/admin/a")), 0)
	assert.Equal(t, named(shut.Links(), "Cancel"), 0)
	assert.True(t, len(shut.ByAttr("aria-current", "page")) > 0) // the page itself is still named in the crumbs
}

// A form that is only read, because its Guard said Disabled, says why in words in its footer.
func TestAFormThatIsOnlyReadSaysWhyInWords(t *testing.T) {
	t.Parallel()

	read := func(guard func(context.Context, struct{}) error) string {
		h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/f", View: webui.Form[struct{}]{
			Load:   func(context.Context) (struct{}, webui.Loaded) { return struct{}{}, webui.Ready() },
			Submit: webui.Action[struct{}]{Guard: guard, Run: func(context.Context, struct{}) (webui.Outcome, error) { return webui.Success("ok"), nil }},
		}}}}.MustCompile("/admin")
		return mainOf(t, pageOf(t, h, "/admin/f")).Text()
	}
	said := read(func(context.Context, struct{}) error { return webui.Disabled("Locked by the release") })
	assert.True(t, strings.Contains(said, "Locked by the release"))
	silent := read(func(context.Context, struct{}) error { return errors.New("no") })
	assert.False(t, strings.Contains(silent, "Locked by the release"))
	assert.False(t, strings.Contains(silent, "no"))
}

// The contract for a post that script makes, for any panel: it carries the header of the panel, and is answered in HTML with a status that says how it went. An accepted
// action is the item with a 200, a refused one the item with a 422, a Guard's refusal the item with a 403 and an error the item with a 500; a form that was refused is
// the form itself, drawn again, with the 422. None of it is a redirect, a cookie or data.
func TestAPostThatScriptMakesIsAnsweredInHTMLWithTheStatusOfHowItWent(t *testing.T) {
	t.Parallel()

	panel := func(r *http.Request) { r.Header.Set("X-Webui-View", "p") }

	s := newShop()
	ok := post(s.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"), panel)
	assert.Equal(t, ok.Code, http.StatusOK)
	assert.Equal(t, ok.Header().Get("Location"), "")
	assert.Equal(t, flashOf(ok), "")
	assert.DeepEqual(t, items(page.Parse(t, ok.Body.String())), []string{"Done: Device saved!"})

	// Refused by a rule: the form, drawn again with what was typed and what was refused, and nothing round it.
	refused := post(s.h, "/admin/device/dev1", formValues("f1", "nope", "f2", "5"), panel)
	assert.Equal(t, refused.Code, http.StatusUnprocessableEntity)
	fragment := page.Parse(t, refused.Body.String())
	assert.True(t, page.IsFragment(refused.Body.String())) // nothing round it
	assert.True(t, strings.Contains(fragment.Root().Text(), "Must be at least 7 characters."))
	typed, _ := fragment.Field("f1")
	assert.Equal(t, typed.Value, "nope")
	assert.Equal(t, fragment.Views()[0].Attr("data-view"), "p")

	// The same post without the header is as it always was: a page, with the 422.
	whole := post(s.h, "/admin/device/dev1", formValues("f1", "nope", "f2", "5"))
	assert.Equal(t, whole.Code, http.StatusUnprocessableEntity)
	assert.False(t, page.IsFragment(whole.Body.String()))
	sidebarNav(t, page.Parse(t, whole.Body.String()))

	// A Guard that refuses the visitor is a 403, which is not the answer to what was sent being refused.
	guarded := newUploadShop(func(f *webui.FileUpload[struct{}]) {
		f.Receive.Guard = func(context.Context, webui.FileHandle[struct{}]) error { return errors.New("requires the editor role") }
	})
	denied := postFragment(guarded.h, "a.bin", "data")
	assert.Equal(t, denied.Code, http.StatusForbidden)
	assert.DeepEqual(t, items(page.Parse(t, denied.Body.String())), []string{"Failed: requires the editor role"})
}
