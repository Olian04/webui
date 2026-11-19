package webui_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

type embedArgs struct {
	ID string `webui:"id"`
}

func theFrame(t *testing.T, p *page.Page) page.El {
	t.Helper()

	var frames []page.El
	for _, e := range p.ByAttr("title", "") {
		if e.Tag() == "iframe" {
			frames = append(frames, e)
		}
	}
	return page.One(t, "the frame", frames)
}

// An Embed is a panel that shows a page of this site in a frame, which is plain markup: its address, its name, and no script of ours.
func TestAnEmbedIsAFrameOfAPageOfThisSiteWithTheNameOfThePanel(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Embed{Title: "Site map", Desc: "Where it is.", Src: "/frame/site-map"})
	p := pageOf(t, h, "/admin/e")

	frame := theFrame(t, p)
	assert.Equal(t, frame.Attr("src"), "/frame/site-map") // as written, not under the mount
	assert.Equal(t, frame.Attr("title"), "Site map")      // the name of the frame to a reader
	assert.Equal(t, frame.Attr("loading"), "lazy")
	assert.Equal(t, frame.Attr("referrerpolicy"), "same-origin")
	assert.False(t, frame.Has("sandbox")) // the page is the app's own code, of its own site: there is no sandbox to set
	assert.Equal(t, len(p.ByRole("heading", "Site map")), 1)
	// A way out for a page that is too small for what it shows.
	assert.True(t, hasLink(p, "Open in a new tab(opens in a new tab)", "/frame/site-map"))
	// It is a panel, so it is found as one.
	assert.Equal(t, len(p.Views()), 1)
}

// What is framed is the app's own page, and is asked for with the request: the address is built from the page's arguments.
func TestAnEmbedLoadReturnsTheAddressForTheRequest(t *testing.T) {
	t.Parallel()

	page := webui.Page[embedArgs]{Path: "/d/{id}", View: webui.Embed{
		Title: "Site map",
		Load: func(ctx context.Context) (string, webui.Loaded) {
			return "/frame/device/" + url.PathEscape(webui.ArgsOf[embedArgs](ctx).ID), webui.Ready()
		},
	}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	assert.Equal(t, theFrame(t, pageOf(t, h, "/admin/d/dev_1")).Attr("src"), "/frame/device/dev_1")
	assert.Equal(t, theFrame(t, pageOf(t, h, "/admin/d/dev_2")).Attr("src"), "/frame/device/dev_2")
}

// A page that is not of this site is not shown: the library shows no other site's page, so an address that Load returns that is not a path of this one is the app's mistake, which
// fails the panel and is logged, and is not told to the visitor.
func TestAnEmbedLoadThatReturnsAnAddressElsewhereIsNotDrawnAndIsLogged(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{"https://elsewhere.example/page", "//elsewhere.example/page", `/\elsewhere.example`, "relative/path", "", "/a\nb"} {
		var out bytes.Buffer
		h := webui.App{Logger: slog.New(slog.NewTextHandler(&out, nil)), Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", View: webui.Embed{
			Title: "Site map",
			Load:  func(context.Context) (string, webui.Loaded) { return bad, webui.Ready() },
		}}}}.MustCompile("/admin")
		body := serve(h, http.MethodGet, "/admin/e").Body.String()
		assert.False(t, strings.Contains(body, "<iframe"))
		assert.Contains(t, body, "Could not load")
		assert.False(t, strings.Contains(body, "elsewhere.example")) // never told
		assert.Contains(t, out.String(), "not a path on this site")
	}
}

// A Load that fails, or that the visitor may not be told of, is a panel that says so, as any panel does, and the rest of the page stands.
func TestAnEmbedLoadThatFailsOrRefusesSaysSoAndNothingIsFramed(t *testing.T) {
	t.Parallel()

	failing := webui.Embed{Title: "Site map", Load: func(context.Context) (string, webui.Loaded) { return "", webui.Failed(context.DeadlineExceeded) }}
	body := serve(editorApp(failing), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, "Could not load")
	assert.False(t, strings.Contains(body, "<iframe"))

	refusing := webui.Embed{Title: "Site map", Load: func(context.Context) (string, webui.Loaded) {
		return "", webui.Failed(webui.Forbidden("This is for the team that runs it."))
	}}
	body = serve(editorApp(refusing), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, "This is for the team that runs it.")
	assert.False(t, strings.Contains(body, "<iframe"))
}

// The policy of the page that holds the frame is the library's own, byte for byte: a frame of this site needs no more than default-src 'self' gives it.
func TestThePolicyOfAPageWithAnEmbedIsTheLibrarysOwn(t *testing.T) {
	t.Parallel()

	embedded := serve(editorApp(webui.Embed{Title: "Site map", Src: "/frame/site-map"}), http.MethodGet, "/admin/e").Header().Get("Content-Security-Policy")
	plain := serve(markdownApp("hello"), http.MethodGet, "/admin/e").Header().Get("Content-Security-Policy")
	assert.Equal(t, embedded, plain)
	assert.False(t, strings.Contains(embedded, "frame-src"))
	assert.Contains(t, embedded, "frame-ancestors 'none'") // the pages of the app are not for a frame, and that stays
}

func TestAnEmbedIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	load := func(context.Context) (string, webui.Loaded) { return "/frame/x", webui.Ready() }
	refuses(t, editorErrors(t, webui.Embed{Src: "/frame/x"}), "Embed", "Title")
	refuses(t, editorErrors(t, webui.Embed{Title: "A"}), "Embed", "Src", "Load")
	refuses(t, editorErrors(t, webui.Embed{Title: "A", Src: "/frame/x", Load: load}), "Embed", "both")
	for _, src := range []string{"https://elsewhere.example/x", "//elsewhere.example/x", "frame/x", `/\x`, "javascript:alert(1)"} {
		refuses(t, editorErrors(t, webui.Embed{Title: "A", Src: src}), "Embed.Src", "not a path on this site")
	}
	_ = editorApp(webui.Embed{Title: "A", Src: "/frame/x"}) // each of the two ways to say the address compiles
	_ = editorApp(webui.Embed{Title: "A", Load: load})
	form := webui.Form[toolState]{Title: "State", Load: loadToolState, Fields: []webui.Accessor[toolState]{minutesInput}}
	refuses(t, editorErrors(t, webui.Embed{Title: "A", Src: "/frame/x", HeadActions: []webui.ToolbarItem{form}}), "Embed.HeadActions[0] is a Form")
}

// An Embed is a panel like any, so a button in its head posts to it, and is done.
func TestAButtonInTheHeadOfAnEmbedIsDone(t *testing.T) {
	t.Parallel()

	ran := 0
	h := editorApp(webui.Embed{
		Title: "Site map", Src: "/frame/site-map",
		HeadActions: []webui.ToolbarItem{webui.Action[webui.NoArgs]{Label: "Rescan", Run: func(context.Context, webui.NoArgs) (webui.Outcome, error) {
			ran++
			return webui.Success("Scanned"), nil
		}}},
	})
	assert.DeepEqual(t, controls(pageOf(t, h, "/admin/e").Root()), []control{{"Rescan", "head:0", false}})
	rec := post(h, "/admin/e", url.Values{"_view": {"p"}, "_act": {"head:0"}})
	assert.Equal(t, flashOf(rec), "oScanned")
	assert.Equal(t, ran, 1)
}
