// Package reloader compiles a [webui.App] again while it serves.
//
// It is for a program whose declaration changes as it runs, such as one whose pages follow what a backend says it has. A
// [Handler] is an [http.Handler]: mount it once, and call Compile each time the declaration changes.
//
//	h := reloader.New()
//	if err := h.Compile(discover(), "/admin"); err != nil { ... }
//	mux.Handle("/admin/", h)
//	// later, when the backend's resources changed:
//	if err := h.Compile(discover(), "/admin"); err != nil { log.Println(err) } // the app that was serving still is
//
// It is a package of its own because few programs need it, and because what it needs of the app is one internal hook,
// so the API of [webui] does not grow for it.
package reloader

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Olian04/webui/internal/metrics"
	"github.com/Olian04/webui/internal/reload"
	"github.com/Olian04/webui/pkg/webui"
)

// Handler is an app that can be compiled again while it serves. A request that is being answered is answered by the app it began with,
// and the next is answered by the new one, with no moment in which there is none.
//
// A compile that fails changes nothing: the app that was serving keeps serving, and the error says why, as [webui.App.Compile]'s
// does. Only before the first successful compile does the Handler serve a failure, the page that says what is wrong, as the handler of
// a compile that failed does.
//
// The metrics are the reason it exists, besides the swap. They can be registered once in a registry, so a second [webui.App.Compile]
// with the same [webui.App.Metrics] fails, which is what two different apps in one registry should do. A Handler registers them
// when it first compiles an app that has them, and the apps it compiles after count into the same ones, so the counts go on
// across a reload. They must be given the same registry as the first, which is checked.
type Handler struct {
	mu       sync.Mutex // one Compile at a time
	handler  atomic.Pointer[http.Handler]
	good     bool                  // a compile has succeeded, so a failure no longer replaces what serves
	recorder *metrics.Recorder     // what the apps count into, once there is one
	registry prometheus.Registerer // where it was registered
}

// New is a Handler with no app yet: until its first Compile it answers 503.
func New() *Handler { return &Handler{} }

// Compile compiles app, to be mounted at prefix as [webui.App.Compile] does, and when it succeeds replaces the app that is serving. It
// returns what [webui.App.Compile] would have, as one error that wraps a [webui.CompileError] for each problem; and the same error
// when app.Metrics is not the registry of the first app that had one, since the counts could not go on in two.
func (r *Handler) Compile(app webui.App, prefix string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var rec *metrics.Recorder
	if app.Metrics != nil && r.recorder != nil {
		same, err := sameRegistry(r.registry, app.Metrics)
		switch {
		case err != nil:
			return err
		case !same:
			return fmt.Errorf("reloader: Compile: App.Metrics is not the registry that the first app was given, so what is counted could not go on; give each reload the same one")
		}
		rec = r.recorder
	}

	h, got, err := reload.Compile(app, prefix, rec)
	if err != nil {
		if !r.good {
			r.handler.Store(&h) // nothing has served yet: say what is wrong
		}
		return err
	}
	if app.Metrics != nil && r.recorder == nil && got != nil {
		r.recorder, r.registry = got, app.Metrics
	}
	r.good = true
	r.handler.Store(&h)
	return nil
}

// ServeHTTP answers with the app that was compiled last and succeeded, or with the failure of the first, or 503 when there has been none.
func (r *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h := r.handler.Load()
	if h == nil {
		http.Error(w, "the app is not ready", http.StatusServiceUnavailable)
		return
	}
	(*h).ServeHTTP(w, req)
}

// sameRegistry is whether two registerers are one, which they are when they are the same value: a registry, or a wrapper of one that
// was made once and is given again. A kind of value that cannot be compared cannot be told, and is said so.
func sameRegistry(a, b prometheus.Registerer) (bool, error) {
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return false, nil
	}
	if !reflect.TypeOf(a).Comparable() {
		return false, errors.New("reloader: Compile: App.Metrics is a kind of value that cannot be compared with the first, so it cannot be told whether it is the same registry; pass a pointer")
	}
	return a == b, nil
}
