package webui_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

type crumbArgs struct {
	Id string
}

type crumbSub struct {
	Id string
}

// crumbApp is a list, a page for one thing at "/thing/{id}" whose crumb says its name, and a page below it.
func crumbApp(logger *slog.Logger, crumb func(context.Context, crumbArgs) string, guard func(context.Context, crumbArgs) error) http.Handler {
	list := webui.Page[webui.NoArgs]{Path: "/thing", Nav: webui.Nav{Label: "Things"}, View: webui.Stack{}}
	one := webui.Page[crumbArgs]{Path: "/thing/{id}", Crumb: crumb, Guard: guard, View: webui.Stack{}}
	log := webui.Page[crumbSub]{Path: "/thing/{id}/log", View: webui.Stack{}}
	return webui.App{Brand: webui.Brand{Name: "Demo"}, Logger: logger, Pages: webui.Pages{list, one, log}}.MustCompile("/admin")
}

func nameOf(_ context.Context, a crumbArgs) string { return "Thing " + strings.ToUpper(a.Id) }

func TestAPagesCrumbSaysWhatItReturnsInTheBreadcrumbAndTheTabTitle(t *testing.T) {
	t.Parallel()

	p := pageOf(t, crumbApp(nil, nameOf, nil), "/admin/thing/abc")
	assert.DeepEqual(t, trail(t, p), []string{"Demo>/admin/thing", "Things>/admin/thing", "Thing ABC"})
	assert.Equal(t, p.Title(), "Thing ABC — Demo")
}

// A page below it has the page for the thing as a parent, with the arguments the path gives it.
func TestAParentsCrumbIsItsOwnWhereAPageBelowItHasItInThePath(t *testing.T) {
	t.Parallel()

	p := pageOf(t, crumbApp(nil, nameOf, nil), "/admin/thing/abc/log")
	assert.DeepEqual(t, trail(t, p), []string{"Demo>/admin/thing", "Things>/admin/thing", "Thing ABC>/admin/thing/abc", "Log"})
	assert.Equal(t, p.Title(), "Log — Demo")
}

func TestACrumbThatSaysNothingFailsOrPanicsLeavesTheWordsOfThePath(t *testing.T) {
	t.Parallel()

	for name, crumb := range map[string]func(context.Context, crumbArgs) string{
		"empty": func(context.Context, crumbArgs) string { return "" },
		"panic": func(context.Context, crumbArgs) string { panic("boom") },
	} {
		var logs bytes.Buffer
		h := crumbApp(slog.New(slog.NewTextHandler(&logs, nil)), crumb, nil)
		rec := serve(h, http.MethodGet, "/admin/thing/abc")
		assert.Equal(t, rec.Code, http.StatusOK)
		assert.Equal(t, trail(t, page.Parse(t, rec.Body.String()))[2], "abc")
		assert.Equal(t, strings.Contains(logs.String(), "a page's Crumb failed"), name == "panic")
	}
}

// The deadline is the context's, as a Load's is: a Crumb that waits on it gives up after two seconds, and says nothing.
func TestACrumbThatIsTooSlowGivesUpAtItsDeadlineAndTheWordsOfThePathStay(t *testing.T) {
	t.Parallel()

	slow := func(ctx context.Context, _ crumbArgs) string {
		select {
		case <-ctx.Done():
			return ""
		case <-time.After(10 * time.Second):
			return "never"
		}
	}
	started := time.Now()
	p := pageOf(t, crumbApp(nil, slow, nil), "/admin/thing/abc")
	assert.True(t, time.Since(started) < 5*time.Second)
	assert.Equal(t, trail(t, p)[2], "abc")
}

// The page's Guard speaks for what its Crumb knows: a visitor it refuses is not told, and the page that refuses is not drawn with it.
func TestACrumbIsNotAskedForAPageWhoseGuardRefusesTheVisitor(t *testing.T) {
	t.Parallel()

	asked := false
	crumb := func(_ context.Context, a crumbArgs) string { asked = true; return "Secret " + a.Id }
	guard := func(context.Context, crumbArgs) error { return errors.New("no") }
	h := crumbApp(nil, crumb, guard)

	rec := serve(h, http.MethodGet, "/admin/thing/abc")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.False(t, strings.Contains(rec.Body.String(), "Secret"))
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/thing/abc/log").Body.String(), "Secret"))
	assert.False(t, asked)
}

func TestACrumbIsHandedTheContextOfTheRequest(t *testing.T) {
	t.Parallel()

	crumb := func(ctx context.Context, a crumbArgs) string {
		assert.Equal(t, webui.ArgsOf[crumbArgs](ctx), a)
		return "ok"
	}
	assert.Equal(t, trail(t, pageOf(t, crumbApp(nil, crumb, nil), "/admin/thing/abc"))[2], "ok")
}

// A page whose path takes the rest has the segments of it for crumbs, and no Crumb of its own; a parent above it may have one.
func TestACrumbOnAPageThatTakesTheRestIsACompileErrorAndAParentAboveItMayHaveOne(t *testing.T) {
	t.Parallel()

	rest := webui.Page[objectArgs]{Path: "/bucket/{name}/{key...}", Crumb: func(context.Context, objectArgs) string { return "x" }, View: webui.Stack{}}
	errs := compileErrors(t, webui.App{Pages: webui.Pages{rest}})
	refuses(t, errs[0].Error(), "Crumb", "{name...}")

	buckets := webui.Page[webui.NoArgs]{Path: "/bucket", Nav: webui.Nav{Label: "Buckets"}, View: webui.Stack{}}
	bucket := webui.Page[bucketArgs]{Path: "/bucket/{name}", Crumb: func(_ context.Context, a bucketArgs) string { return "Bucket " + a.Name }, View: webui.Stack{}}
	object := webui.Page[objectArgs]{Path: "/bucket/{name}/{key...}", View: webui.Stack{}}
	h := webui.App{Pages: webui.Pages{buckets, bucket, object}}.MustCompile("/admin")
	assert.DeepEqual(t, trail(t, pageOf(t, h, "/admin/bucket/photos/2026/img.png"))[2:], []string{
		"Bucket photos>/admin/bucket/photos", "2026>/admin/bucket/photos/2026", "img.png",
	})
}

// The landing page's crumb replaces its navigation label in the trail.
func TestTheLandingPagesCrumbReplacesItsNavigationLabelInTheTrail(t *testing.T) {
	t.Parallel()

	home := webui.Page[webui.NoArgs]{Path: "/", Nav: webui.Nav{Label: "Status"}, Crumb: func(context.Context, webui.NoArgs) string { return "All systems" }, View: webui.Stack{}}
	got := trail(t, pageOf(t, webui.App{Pages: webui.Pages{home}}.MustCompile("/admin"), "/admin/"))
	assert.Equal(t, got[len(got)-1], "All systems") // and not "Status", the label the page has in the sidebar
}

// Every crumb is in the page and is read out, whatever the width of the screen cuts from the picture, and the page's own is where the reader is.
func TestEveryCrumbIsInThePageAndTheCurrentPageIsMarked(t *testing.T) {
	t.Parallel()

	p := pageOf(t, bucketApp(), "/admin/bucket/photos/2026/img.png")
	got := trail(t, p)
	assert.Equal(t, got[len(got)-1], "img.png")
	var current []string
	for _, c := range crumbs(t, p).Children() {
		if c.Attr("aria-current") == "page" {
			current = append(current, c.Text())
		}
	}
	assert.DeepEqual(t, current, []string{"img.png"}) // the one that is where the reader is, and not a link
	assert.Equal(t, len(crumbs(t, p).Links()), len(got)-1)
}
