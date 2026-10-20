package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type thing struct{ Name string }
type thingArgs struct{ ID string }

var (
	thingName = webui.String[thing]{
		Label: "Name", Load: func(t thing) string { return t.Name }, Store: func(t *thing, v string) { t.Name = v },
		Rules: webui.StringRules{Required: true},
	}
)

// metricsApp is a page of one thing, a list of things and a search, whose loaders and actions
// fail when told to.
func metricsApp(reg prometheus.Registerer, loadErr, runErr error) http.Handler {
	return webui.App{Metrics: reg, Pages: webui.Pages{
		webui.Page[thingArgs]{
			Path: "/thing/{id}",
			Search: func(context.Context, string, int) ([]webui.Hit[thingArgs], error) {
				return []webui.Hit[thingArgs]{{Args: thingArgs{ID: "a"}, Title: "A"}}, nil
			},
			Body: webui.Form[thing]{
				Load:   func(context.Context) (thing, error) { return thing{Name: "x"}, loadErr },
				Fields: []webui.Accessor[thing]{thingName},
				Submit: webui.Action[thing]{
					Guard: func(_ context.Context, t thing) error {
						if t.Name == "forbidden" {
							return errors.New("no")
						}
						return nil
					},
					Run: func(_ context.Context, t thing) (webui.Outcome, error) {
						if runErr != nil {
							return webui.Outcome{}, runErr
						}
						if t.Name == "warn" {
							return webui.Warning("careful"), nil
						}
						return webui.Success("Saved"), nil
					},
				},
			},
		},
		webui.Page[webui.NoArgs]{
			Path: "/things", Nav: webui.Nav{Label: "Things"},
			Body: webui.Table[thing]{
				Title: "Things",
				Load: func(context.Context, webui.Query) (webui.Window[thing], error) {
					return webui.Window[thing]{Items: []thing{{Name: "a"}}, Total: 1}, loadErr
				},
				Key:     func(t thing) string { return t.Name },
				Columns: []webui.Accessor[thing]{thingName},
				Export:  true,
			},
		},
	}}.MustCompile("/admin")
}

// gathered is every sample of a metric, as its labels and value: a histogram's is its count.
func gathered(t *testing.T, reg *prometheus.Registry, name string) map[string]float64 {
	t.Helper()

	families, err := reg.Gather()
	assert.NoError(t, err)
	out := map[string]float64{}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			var parts []string
			for _, l := range m.GetLabel() {
				parts = append(parts, l.GetName()+"="+l.GetValue())
			}
			key := strings.Join(parts, ",")
			switch {
			case m.GetHistogram() != nil:
				out[key] = float64(m.GetHistogram().GetSampleCount())
			case m.GetCounter() != nil:
				out[key] = m.GetCounter().GetValue()
			case m.GetGauge() != nil:
				out[key] = m.GetGauge().GetValue()
			}
		}
	}
	return out
}

func TestOnlyWebuiMetricsAreRegisteredAndNoGoOnes(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	h := metricsApp(reg, nil, nil)
	serve(h, http.MethodGet, "/admin/thing/a")
	serve(h, http.MethodGet, "/admin/nothing")
	families, err := reg.Gather()
	assert.NoError(t, err)
	assert.True(t, len(families) > 0)
	for _, f := range families {
		assert.True(t, strings.HasPrefix(f.GetName(), "webui_"))
	}
}

func TestAnAppWithNoRegistryCountsNothingAndServesNoMetrics(t *testing.T) {
	t.Parallel()

	h := metricsApp(nil, nil, nil)
	assert.Equal(t, serve(h, http.MethodGet, "/admin/thing/a").Code, http.StatusOK)
	// The app does not serve them either way: the application does, beside its own.
	reg := prometheus.NewRegistry()
	assert.Equal(t, serve(metricsApp(reg, nil, nil), http.MethodGet, "/admin/_webui/metrics").Code, http.StatusNotFound)
}

func TestRequestsAreCountedByThePagesTemplateNotByTheAddress(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	h := metricsApp(reg, nil, nil)
	serve(h, http.MethodGet, "/admin/thing/a")
	serve(h, http.MethodGet, "/admin/thing/b")
	serve(h, http.MethodGet, "/admin/nothing")
	serve(h, http.MethodGet, "/admin/_webui/search?q=a")

	requests := gathered(t, reg, "webui_http_requests_total")
	assert.Equal(t, requests["code=200,method=GET,page=/thing/{id}"], 2.0) // two addresses, one series
	assert.Equal(t, requests["code=404,method=GET,page=unmatched"], 1.0)
	assert.Equal(t, requests["code=200,method=GET,page=/_webui"], 1.0)
	assert.Equal(t, gathered(t, reg, "webui_http_request_duration_seconds")["method=GET,page=/thing/{id}"], 2.0)
	assert.Equal(t, gathered(t, reg, "webui_http_requests_in_flight")[""], 0.0)
	assert.Equal(t, len(gathered(t, reg, "webui_build_info")), 1)
}

func TestLoadsAreTimedAndTheirFailuresCounted(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	h := metricsApp(reg, nil, nil)
	serve(h, http.MethodGet, "/admin/thing/a")
	serve(h, http.MethodGet, "/admin/things")
	assert.Equal(t, gathered(t, reg, "webui_load_duration_seconds")["kind=form,page=/thing/{id}"], 1.0)
	assert.Equal(t, gathered(t, reg, "webui_load_duration_seconds")["kind=table,page=/things"], 1.0)
	assert.Equal(t, len(gathered(t, reg, "webui_load_errors_total")), 0)

	bad := prometheus.NewRegistry()
	failing := metricsApp(bad, errors.New("down"), nil)
	serve(failing, http.MethodGet, "/admin/things") // the panel says so, and the page stays
	assert.Equal(t, gathered(t, bad, "webui_load_errors_total")["kind=table,page=/things"], 1.0)
}

func TestActionsAreCountedByWhatCameOfThem(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	h := metricsApp(reg, nil, nil)
	submit := func(name string) {
		post(h, "/admin/thing/a", url.Values{"_leaf": {"p"}, "f0": {name}})
	}
	submit("ok")
	submit("warn")
	submit("forbidden")
	submit("") // required: the rules refuse it before anything runs
	actions := gathered(t, reg, "webui_actions_total")
	assert.Equal(t, actions["page=/thing/{id},result=success"], 1.0)
	assert.Equal(t, actions["page=/thing/{id},result=warning"], 1.0)
	assert.Equal(t, actions["page=/thing/{id},result=denied"], 1.0)
	assert.Equal(t, actions["page=/thing/{id},result=invalid"], 1.0)

	failing := prometheus.NewRegistry()
	broken := metricsApp(failing, nil, errors.New("boom"))
	post(broken, "/admin/thing/a", url.Values{"_leaf": {"p"}, "f0": {"ok"}})
	assert.Equal(t, gathered(t, failing, "webui_actions_total")["page=/thing/{id},result=error"], 1.0)
}

func TestSearchesAndExportsAreCounted(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	h := metricsApp(reg, nil, nil)
	serve(h, http.MethodGet, "/admin/_webui/search?q=a")
	assert.Equal(t, gathered(t, reg, "webui_search_duration_seconds")[""], 1.0)
	rec := serve(h, http.MethodGet, "/admin/things?things.export=csv")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, gathered(t, reg, "webui_exports_total")["format=csv,page=/things"], 1.0)
}

// An app compiled again, or another given the same registry, counts into the metrics that are there, and none of it fails.
func TestAnAppCompiledAgainOrAnotherGivenTheSameRegistryCountsIntoTheSameMetrics(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	first := metricsApp(reg, nil, nil)
	serve(first, http.MethodGet, "/admin/thing/a")
	second := metricsApp(reg, nil, nil) // the same app again
	serve(second, http.MethodGet, "/admin/thing/a")
	serve(first, http.MethodGet, "/admin/thing/a") // and the first goes on counting too
	other, err := webui.App{Metrics: reg, Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/", Body: webui.Stack{}}}}.Compile("/admin")
	assert.NoError(t, err)
	serve(other, http.MethodGet, "/admin/")

	requests := gathered(t, reg, "webui_http_requests_total")
	assert.Equal(t, requests["code=200,method=GET,page=/thing/{id}"], 3.0)
	assert.Equal(t, requests["code=200,method=GET,page=/"], 1.0)
	assert.Equal(t, len(gathered(t, reg, "webui_build_info")), 1)
}

// Wrapping the registry for each, with a label of its own, keeps apps apart.
func TestWrappingTheRegistryForEachAppKeepsTheirCountsApart(t *testing.T) {
	t.Parallel()

	one := prometheus.NewRegistry()
	for _, name := range []string{"a", "b"} {
		wrapped := prometheus.WrapRegistererWith(prometheus.Labels{"app": name}, one)
		h, err := webui.App{Metrics: wrapped, Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/", Body: webui.Stack{}}}}.Compile("/" + name)
		assert.NoError(t, err)
		serve(h, http.MethodGet, "/"+name+"/")
	}
	requests := gathered(t, one, "webui_http_requests_total")
	assert.Equal(t, requests["app=a,code=200,method=GET,page=/"], 1.0)
	assert.Equal(t, requests["app=b,code=200,method=GET,page=/"], 1.0)
}

// A metric of the same name that is not the same is a clash, and is still refused.
func TestAMetricOfTheNameOfAWebuiOneThatIsNotTheSameIsRefused(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewCounter(prometheus.CounterOpts{Namespace: "webui", Name: "exports_total", Help: "the program's own"}))
	_, err := webui.App{Metrics: reg, Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/", Body: webui.Stack{}}}}.Compile("/admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "App.Metrics")
}
