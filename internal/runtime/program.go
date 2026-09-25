package runtime

import "net/http"

type Program struct {
	Routes []Route
}

type Route struct {
	Path       string
	Method     string // Page = GET, Action = POST, PUT, DELETE, etc.
	Middleware []func(http.Handler) http.Handler
	Handler    http.Handler
}
