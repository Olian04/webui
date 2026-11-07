package webui_test

import (
	"context"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// recorder is an Observer of the program's own: it notes what it is told, and counts nothing with Prometheus.
type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) note(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, s)
}

func (r *recorder) Begin() func(page, method string, status int, started time.Time) {
	return func(page, method string, status int, _ time.Time) {
		r.note("request " + method + " " + page + " " + http.StatusText(status))
	}
}
func (r *recorder) Load(page, kind string, _ time.Time, err error) {
	r.note("load " + kind + " " + page + " failed=" + http.StatusText(map[bool]int{true: 500, false: 200}[err != nil]))
}
func (r *recorder) Action(page, result string) { r.note("action " + page + " " + result) }
func (r *recorder) Export(page, format string) { r.note("export " + page + " " + format) }
func (r *recorder) Search(time.Time)           { r.note("search") }
func (r *recorder) SearchFailed(page string)   { r.note("search failed " + page) }
func (r *recorder) NavFailed(page string)      { r.note("nav failed " + page) }
func (r *recorder) told() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.calls, "\n")
}

// An app tells its Observer what it does, and any Observer will do: one that the program writes, as well as the one that the prom package makes of a registry.
func TestAnAppTellsAnObserverOfTheProgramsOwnWhatItDoes(t *testing.T) {
	t.Parallel()

	var rec recorder
	h := webui.App{Observer: &rec, Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/p", View: webui.Form[thing]{
		Load:   func(context.Context) (thing, webui.Loaded) { return thing{Name: "x"}, webui.Ready() },
		Fields: []webui.Accessor[thing]{thingName},
		Submit: webui.Action[thing]{Run: func(context.Context, thing) (webui.Outcome, error) { return webui.Success("done"), nil }},
	}}}}.MustCompile("/admin")

	assert.Equal(t, serve(h, http.MethodGet, "/admin/p").Code, http.StatusOK)
	assert.Equal(t, post(h, "/admin/p", formValues("_view", "p", "f0", "y")).Code, http.StatusSeeOther)

	told := rec.told()
	assert.Contains(t, told, "load form /p failed=OK")
	assert.Contains(t, told, "request GET /p OK")
	assert.Contains(t, told, "action /p success")
	assert.Contains(t, told, "request POST /p See Other")
}

// The library does not link the Prometheus client for a program that does not count with it: it is in the prom package, which a program imports to use it.
func TestThePrometheusClientIsLinkedOnlyByTheProgramsThatImportTheProm(t *testing.T) {
	t.Parallel()

	out, err := exec.Command("go", "list", "-deps", "../pkg/webui").Output()
	if err != nil {
		t.Skipf("go list is not available: %v", err)
	}
	assert.False(t, strings.Contains(string(out), "prometheus"))
}
