package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// The app counts what it does (the requests each page answers, how long its Load functions
// take, what its actions come to) as Prometheus metrics, and serves none of it: App.Metrics is
// the registry it puts them in. This program is the host, so it has a registry of its own, with
// the metrics a Go program usually has, and serves it where it likes; a real one would protect
// that address as it does the rest of its operations.
var registry = func() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return reg
}()

func metricsHandler(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
