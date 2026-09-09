// Package echo is the demo domain model. It renders in every mode.
//
// The domain is what stays constant across modes: it holds behaviour and knows
// nothing about how it is reached. Each mode supplies its own IO adapter over
// this same package — a CLI command, an HTTP handler, or the public library
// facade — so choosing a mode changes the edges, never the core.
//
// Keep it dependency-free: no HTTP, slog, Prometheus, or config imports.
package echo

import "strings"

type Service struct{}

// Request and Response carry JSON tags so a transport can serialize them
// directly, without a separate DTO layer.
type Request struct {
	Message string `json:"message"`
}

type Response struct {
	Message string `json:"message"`
}

func NewService() Service {
	return Service{}
}

func (Service) Echo(req Request) Response {
	return Response{Message: strings.TrimSpace(req.Message)}
}
