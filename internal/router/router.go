package router

import (
	"net/http"

	"github.com/Olian04/webui/internal/runtime"
)

type Router struct {
	Mux *http.ServeMux
}

func New(program *runtime.Program) *Router {
	router := &Router{
		Mux: http.NewServeMux(),
	}

	// Sorting not required.
	// ServeMux keeps a routingNode tree and, on each request, picks the most specific pattern.
	// Registration order does not decide the winner.
	for _, route := range program.Routes {
		pattern := route.Method + " " + route.Path
		handler := route.Handler
		for _, middleware := range route.Middleware {
			handler = middleware(handler)
		}
		router.Mux.Handle(pattern, handler)
	}

	return router
}
