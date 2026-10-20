package webui_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// reloadApp is an app with one page, at path, whose entry in the navigation says word.
func reloadApp(reg prometheus.Registerer, path webui.PageID[webui.NoArgs], word string) webui.App {
	return webui.App{Metrics: reg, Pages: webui.Pages{webui.Page[webui.NoArgs]{
		Path: path, Nav: webui.Nav{Label: word},
		Body: webui.Stack{},
	}}}
}

func TestAReloaderAnswers503UntilItHasAnApp(t *testing.T) {
	t.Parallel()

	assert.Equal(t, serve(webui.NewReloader(), http.MethodGet, "/admin/a").Code, http.StatusServiceUnavailable)
}

func TestAReloaderServesTheAppItWasLastGivenAndTheOldOneIsGone(t *testing.T) {
	t.Parallel()

	r := webui.NewReloader()
	assert.NoError(t, r.Compile(reloadApp(nil, "/a", "alpha"), "/admin"))
	assert.Contains(t, serve(r, http.MethodGet, "/admin/a").Body.String(), "alpha")
	assert.Equal(t, serve(r, http.MethodGet, "/admin/b").Code, http.StatusNotFound)

	assert.NoError(t, r.Compile(reloadApp(nil, "/b", "beta"), "/admin"))
	assert.Contains(t, serve(r, http.MethodGet, "/admin/b").Body.String(), "beta")
	assert.Equal(t, serve(r, http.MethodGet, "/admin/a").Code, http.StatusNotFound)

	// The mount prefix may change too.
	assert.NoError(t, r.Compile(reloadApp(nil, "/b", "beta"), "/tools"))
	assert.Equal(t, serve(r, http.MethodGet, "/tools/b").Code, http.StatusOK)
	assert.Equal(t, serve(r, http.MethodGet, "/admin/b").Code, http.StatusNotFound)
}

// A compile that fails changes nothing, and says why.
func TestAReloadThatFailsKeepsTheAppThatWasServing(t *testing.T) {
	t.Parallel()

	r := webui.NewReloader()
	assert.NoError(t, r.Compile(reloadApp(nil, "/a", "alpha"), "/admin"))

	err := r.Compile(webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/bad", Body: webui.Table[Device]{}}}}, "/admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "a Table has none of Rows, Load and Feed")
	assert.Contains(t, serve(r, http.MethodGet, "/admin/a").Body.String(), "alpha")
	assert.Equal(t, serve(r, http.MethodGet, "/admin/bad").Code, http.StatusNotFound)

	assert.Error(t, r.Compile(reloadApp(nil, "/a", "alpha"), "admin/")) // not a prefix
	assert.Equal(t, serve(r, http.MethodGet, "/admin/a").Code, http.StatusOK)
}

// Before there is an app that works, the failure is what is served, as the handler of a compile that failed does.
func TestAReloaderThatHasOnlyFailedServesTheFailureUntilAnAppWorks(t *testing.T) {
	t.Parallel()

	r := webui.NewReloader()
	assert.Error(t, r.Compile(webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/bad", Body: webui.Table[Device]{}}}}, "/admin"))
	rec := serve(r, http.MethodGet, "/admin/anything")
	assert.Equal(t, rec.Code, http.StatusInternalServerError)
	assert.Contains(t, rec.Body.String(), "Failed to compile")

	assert.NoError(t, r.Compile(reloadApp(nil, "/a", "alpha"), "/admin"))
	assert.Equal(t, serve(r, http.MethodGet, "/admin/a").Code, http.StatusOK)

	// And now a failure no longer replaces it.
	assert.Error(t, r.Compile(webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/bad", Body: webui.Table[Device]{}}}}, "/admin"))
	assert.Equal(t, serve(r, http.MethodGet, "/admin/a").Code, http.StatusOK)
}

// What is counted goes on across a reload, in the registry that was given, which a second Compile of an App cannot register again.
func TestTheMetricsOfAReloaderGoOnAcrossAReload(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	r := webui.NewReloader()
	assert.NoError(t, r.Compile(reloadApp(reg, "/a", "alpha"), "/admin"))
	serve(r, http.MethodGet, "/admin/a")
	serve(r, http.MethodGet, "/admin/a")
	assert.Equal(t, gathered(t, reg, "webui_http_requests_total")["code=200,method=GET,page=/a"], 2.0)

	assert.NoError(t, r.Compile(reloadApp(reg, "/b", "beta"), "/admin"))
	serve(r, http.MethodGet, "/admin/b")
	serve(r, http.MethodGet, "/admin/a") // the page that is gone
	requests := gathered(t, reg, "webui_http_requests_total")
	assert.Equal(t, requests["code=200,method=GET,page=/a"], 2.0)
	assert.Equal(t, requests["code=200,method=GET,page=/b"], 1.0)
	assert.Equal(t, requests["code=404,method=GET,page=unmatched"], 1.0)
	assert.Equal(t, len(gathered(t, reg, "webui_build_info")), 1)

	// Two apps in one registry is still an error, for whoever does not use a Reloader.
	_, err := reloadApp(reg, "/c", "gamma").Compile("/admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "App.Metrics")
}

// The counts could not go on in two registries, so a reload that gives another is refused, and nothing changes.
func TestAReloadWithAnotherRegistryIsRefused(t *testing.T) {
	t.Parallel()

	one, two := prometheus.NewRegistry(), prometheus.NewRegistry()
	r := webui.NewReloader()
	assert.NoError(t, r.Compile(reloadApp(one, "/a", "alpha"), "/admin"))

	err := r.Compile(reloadApp(two, "/b", "beta"), "/admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "App.Metrics is not the registry that the first app was given")
	assert.Equal(t, serve(r, http.MethodGet, "/admin/a").Code, http.StatusOK)
	assert.Equal(t, len(gathered(t, two, "webui_build_info")), 0)

	// The same wrapped registry, made once and given again, is the same registry.
	wrapped := prometheus.WrapRegistererWith(prometheus.Labels{"app": "admin"}, prometheus.NewRegistry())
	w := webui.NewReloader()
	assert.NoError(t, w.Compile(reloadApp(wrapped, "/a", "alpha"), "/admin"))
	assert.NoError(t, w.Compile(reloadApp(wrapped, "/b", "beta"), "/admin"))
}

// An app that counts nothing can be reloaded, and one that counts can be added later.
func TestAReloaderMayStartWithoutMetricsAndGainThem(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	r := webui.NewReloader()
	assert.NoError(t, r.Compile(reloadApp(nil, "/a", "alpha"), "/admin"))
	assert.NoError(t, r.Compile(reloadApp(reg, "/b", "beta"), "/admin"))
	serve(r, http.MethodGet, "/admin/b")
	assert.Equal(t, gathered(t, reg, "webui_http_requests_total")["code=200,method=GET,page=/b"], 1.0)
	assert.NoError(t, r.Compile(reloadApp(reg, "/c", "gamma"), "/admin"))
	serve(r, http.MethodGet, "/admin/c")
	assert.Equal(t, gathered(t, reg, "webui_http_requests_total")["code=200,method=GET,page=/c"], 1.0)
}

// A reload while requests are served is safe: each is answered by one app or the other, and the race detector says so.
func TestARequestDuringAReloadIsAnsweredByOneAppOrTheOther(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	r := webui.NewReloader()
	assert.NoError(t, r.Compile(reloadApp(reg, "/a", "alpha"), "/admin"))

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				for _, path := range []string{"/admin/a", "/admin/b"} {
					rec := serve(r, http.MethodGet, path)
					body := rec.Body.String()
					switch rec.Code {
					case http.StatusOK:
						if !strings.Contains(body, "alpha") && !strings.Contains(body, "beta") {
							t.Errorf("%s: a page of neither app", path)
						}
					case http.StatusNotFound:
					default:
						t.Errorf("%s: status %d", path, rec.Code)
					}
				}
			}
		}()
	}
	for i := range 20 {
		path, word := webui.PageID[webui.NoArgs]("/a"), "alpha"
		if i%2 == 0 {
			path, word = "/b", "beta"
		}
		assert.NoError(t, r.Compile(reloadApp(reg, path, word), "/admin"))
	}
	close(stop)
	wg.Wait()
}
