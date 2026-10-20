// Package metrics is what the runtime counts and times, as Prometheus metrics, and nothing
// else: no Go runtime or process metrics, which the host application has its own for. It
// only collects. It serves nothing: the application registers the metrics with its own
// registry and serves them beside the rest of its own.
//
// Every label is a low-cardinality value the application declared: the path template of a
// page ("/device/{id}") and never the address that was asked for, a method, a status, and a
// small fixed set of kinds. A Recorder that is nil does nothing, so the call sites do not
// check whether metrics are on.
package metrics

import (
	"context"
	"errors"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Namespace prefixes every metric.
const Namespace = "webui"

// Recorder holds the metrics. The methods are safe for concurrent use, and a nil Recorder
// is a Recorder that records nothing.
type Recorder struct {
	requests    *prometheus.CounterVec
	duration    *prometheus.HistogramVec
	inFlight    prometheus.Gauge
	loads       *prometheus.HistogramVec
	loadErrors  *prometheus.CounterVec
	actions     *prometheus.CounterVec
	exports     *prometheus.CounterVec
	search      prometheus.Histogram
	searchFails *prometheus.CounterVec
	navFails    *prometheus.CounterVec
}

// New makes the metrics and registers them with reg. A metric that reg already has, the same
// in name, labels and help, is the one that is counted into: an app compiled again, or another
// app given the same registry, adds to the counts that are there, and none of them fails. Wrap
// reg with prometheus.WrapRegistererWith, with a label of its own, to keep apps apart. It fails
// only for a metric that has the name of one of these and is not the same, which is a clash.
func New(reg prometheus.Registerer) (*Recorder, error) {
	r := &Recorder{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace, Name: "http_requests_total",
			Help: "Requests the app answered, by the page that answered, the method, and the status.",
		}, []string{"page", "method", "code"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: Namespace, Name: "http_request_duration_seconds",
			Help:    "Time to answer a request, from its arrival to the end of the handler.",
			Buckets: prometheus.DefBuckets,
		}, []string{"page", "method"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace, Name: "http_requests_in_flight",
			Help: "Requests being answered now.",
		}),
		loads: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: Namespace, Name: "load_duration_seconds",
			Help:    "Time the application's own Load functions took, by the page and the kind of panel.",
			Buckets: prometheus.DefBuckets,
		}, []string{"page", "kind"}),
		loadErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace, Name: "load_errors_total",
			Help: "Loads that failed, by the page and the kind of panel. The panel says so and the page stays.",
		}, []string{"page", "kind"}),
		actions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace, Name: "actions_total",
			Help: "Actions run, by the page and what came of them: success, warning, failure, reject, error (Run returned an error) or denied (a Guard refused).",
		}, []string{"page", "result"}),
		exports: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace, Name: "exports_total",
			Help: "Table exports, by the page and the format.",
		}, []string{"page", "format"}),
		search: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: Namespace, Name: "search_duration_seconds",
			Help:    "Time to answer a search, across every page that is searched.",
			Buckets: prometheus.DefBuckets,
		}),
		searchFails: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace, Name: "search_errors_total",
			Help: "Pages whose Search failed or timed out, and were left out of the results.",
		}, []string{"page"}),
		navFails: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace, Name: "nav_entries_errors_total",
			Help: "Pages whose NavEntries failed or timed out, and were left out of the sidebar.",
		}, []string{"page"}),
	}
	info := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace, Name: "build_info",
		Help: "The version of the webui library the program was built with, always 1.",
	}, []string{"version"})
	info.WithLabelValues(version()).Set(1)

	a := adopter{reg: reg}
	r.requests, r.duration, r.inFlight = adopt(&a, r.requests), adopt(&a, r.duration), adopt(&a, r.inFlight)
	r.loads, r.loadErrors, r.actions, r.exports = adopt(&a, r.loads), adopt(&a, r.loadErrors), adopt(&a, r.actions), adopt(&a, r.exports)
	r.search, r.searchFails, r.navFails = adopt(&a, r.search), adopt(&a, r.searchFails), adopt(&a, r.navFails)
	adopt(&a, info)
	if a.err != nil {
		return nil, a.err
	}
	return r, nil
}

// adopter registers collectors with a registry and keeps the first error, so that a list of them is not a list of checks.
type adopter struct {
	reg prometheus.Registerer
	err error
}

// adopt registers c, or when the registry already has one the same, says that one is to be used: what it counts is what
// the first app counted too.
func adopt[C prometheus.Collector](a *adopter, c C) C {
	err := a.reg.Register(c)
	var already prometheus.AlreadyRegisteredError
	if errors.As(err, &already) {
		if existing, ok := already.ExistingCollector.(C); ok {
			return existing
		}
	}
	if err != nil && a.err == nil {
		a.err = err
	}
	return c
}

// version is the library's version as the program that uses it was built, or "unknown".
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/Olian04/webui" {
			return dep.Version
		}
	}
	if info.Main.Path == "github.com/Olian04/webui" && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}

// Begin counts a request as in flight, and returns what ends it: call it with the page that
// answered, the method and the status.
func (r *Recorder) Begin() func(page, method string, status int, started time.Time) {
	if r == nil {
		return func(string, string, int, time.Time) {}
	}
	r.inFlight.Inc()
	return func(page, method string, status int, started time.Time) {
		r.inFlight.Dec()
		r.requests.WithLabelValues(page, method, strconv.Itoa(status)).Inc()
		r.duration.WithLabelValues(page, method).Observe(time.Since(started).Seconds())
	}
}

// Load records one run of an application's Load: a panel of a kind (form, editor, table,
// diff, markdown) on a page, how long it took, and whether it failed.
func (r *Recorder) Load(page, kind string, started time.Time, err error) {
	if r == nil {
		return
	}
	r.loads.WithLabelValues(page, kind).Observe(time.Since(started).Seconds())
	if err != nil && !errors.Is(err, context.Canceled) { // a visitor who left did not break the Load
		r.loadErrors.WithLabelValues(page, kind).Inc()
	}
}

// Action records an action's result on a page.
func (r *Recorder) Action(page, result string) {
	if r != nil {
		r.actions.WithLabelValues(page, result).Inc()
	}
}

// Export records a table export.
func (r *Recorder) Export(page, format string) {
	if r != nil {
		r.exports.WithLabelValues(page, format).Inc()
	}
}

// Search records the time a search took.
func (r *Recorder) Search(started time.Time) {
	if r != nil {
		r.search.Observe(time.Since(started).Seconds())
	}
}

// SearchFailed records a page whose Search failed.
func (r *Recorder) SearchFailed(page string) {
	if r != nil {
		r.searchFails.WithLabelValues(page).Inc()
	}
}

// NavFailed records a page whose NavEntries failed.
func (r *Recorder) NavFailed(page string) {
	if r != nil {
		r.navFails.WithLabelValues(page).Inc()
	}
}
