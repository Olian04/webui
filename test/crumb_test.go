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
)

type crumbArgs struct {
	Id string
}

type crumbSub struct {
	Id string
}

// crumbApp is a list, a page for one thing at "/thing/{id}" whose crumb says its name, and a page below it.
func crumbApp(logger *slog.Logger, crumb func(context.Context, crumbArgs) string, guard func(context.Context, crumbArgs) error) http.Handler {
	list := webui.Page[webui.NoArgs]{Path: "/thing", Nav: webui.Nav{Label: "Things"}, Body: webui.Stack{}}
	one := webui.Page[crumbArgs]{Path: "/thing/{id}", Crumb: crumb, Guard: guard, Body: webui.Stack{}}
	log := webui.Page[crumbSub]{Path: "/thing/{id}/log", Body: webui.Stack{}}
	return webui.App{Brand: webui.Brand{Name: "Demo"}, Logger: logger, Pages: webui.Pages{list, one, log}}.MustCompile("/admin")
}

func nameOf(_ context.Context, a crumbArgs) string { return "Thing " + strings.ToUpper(a.Id) }

func TestAPagesCrumbSaysWhatItReturnsInTheBreadcrumbAndTheTabTitle(t *testing.T) {
	t.Parallel()

	body := serve(crumbApp(nil, nameOf, nil), http.MethodGet, "/admin/thing/abc").Body.String()
	assert.Contains(t, body, `<a href="/admin/thing">Things</a>`)
	assert.Contains(t, body, `<span class="cur" aria-current="page">Thing ABC</span>`)
	assert.Contains(t, body, "<title>Thing ABC — Demo</title>")
}

// A page below it has the page for the thing as a parent, with the arguments the path gives it.
func TestAParentsCrumbIsItsOwnWhereAPageBelowItHasItInThePath(t *testing.T) {
	t.Parallel()

	body := serve(crumbApp(nil, nameOf, nil), http.MethodGet, "/admin/thing/abc/log").Body.String()
	assert.Contains(t, body, `<a href="/admin/thing/abc">Thing ABC</a>`)
	assert.Contains(t, body, `<span class="cur" aria-current="page">Log</span>`)
	assert.Contains(t, body, "<title>Log — Demo</title>")
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
		assert.Contains(t, rec.Body.String(), `<span class="cur" aria-current="page">abc</span>`)
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
	body := serve(crumbApp(nil, slow, nil), http.MethodGet, "/admin/thing/abc").Body.String()
	assert.True(t, time.Since(started) < 5*time.Second)
	assert.Contains(t, body, `<span class="cur" aria-current="page">abc</span>`)
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
	assert.Contains(t, serve(crumbApp(nil, crumb, nil), http.MethodGet, "/admin/thing/abc").Body.String(), `aria-current="page">ok</span>`)
}

// A page whose path takes the rest has the segments of it for crumbs, and no Crumb of its own; a parent above it may have one.
func TestACrumbOnAPageThatTakesTheRestIsACompileErrorAndAParentAboveItMayHaveOne(t *testing.T) {
	t.Parallel()

	rest := webui.Page[objectArgs]{Path: "/bucket/{name}/{key...}", Crumb: func(context.Context, objectArgs) string { return "x" }, Body: webui.Stack{}}
	errs := compileErrors(t, webui.App{Pages: webui.Pages{rest}})
	assert.Contains(t, errs[0].Error(), "has a Crumb and its path ends in a {name...}")

	buckets := webui.Page[webui.NoArgs]{Path: "/bucket", Nav: webui.Nav{Label: "Buckets"}, Body: webui.Stack{}}
	bucket := webui.Page[bucketArgs]{Path: "/bucket/{name}", Crumb: func(_ context.Context, a bucketArgs) string { return "Bucket " + a.Name }, Body: webui.Stack{}}
	object := webui.Page[objectArgs]{Path: "/bucket/{name}/{key...}", Body: webui.Stack{}}
	h := webui.App{Pages: webui.Pages{buckets, bucket, object}}.MustCompile("/admin")
	body := serve(h, http.MethodGet, "/admin/bucket/photos/2026/img.png").Body.String()
	assert.Contains(t, body, `<a href="/admin/bucket/photos">Bucket photos</a>`)
	assert.Contains(t, body, `<a href="/admin/bucket/photos/2026">2026</a>`)
	assert.Contains(t, body, `<span class="cur" aria-current="page">img.png</span>`)
}

// The landing page's crumb replaces its navigation label in the trail.
func TestTheLandingPagesCrumbReplacesItsNavigationLabelInTheTrail(t *testing.T) {
	t.Parallel()

	home := webui.Page[webui.NoArgs]{Path: "/", Nav: webui.Nav{Label: "Status"}, Crumb: func(context.Context, webui.NoArgs) string { return "All systems" }, Body: webui.Stack{}}
	body := serve(webui.App{Pages: webui.Pages{home}}.MustCompile("/admin"), http.MethodGet, "/admin/").Body.String()
	assert.Contains(t, body, `<span class="cur" aria-current="page">All systems</span>`)
}
