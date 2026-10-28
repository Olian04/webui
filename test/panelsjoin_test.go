package webui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// A visitor who leaves while a panel is loading ends the request, and a panel's Load is not left running behind it: the
// handler has returned when no Load of the page is still going, as it has when the sidebar's workers are done, so that
// a program that shuts the server down and then closes what its Loads use does not close it under one.
func TestAPanelsLoadHasFinishedWhenARequestThatTheVisitorLeftHasReturned(t *testing.T) {
	t.Parallel()

	var started, finished atomic.Bool
	began := make(chan struct{})
	page := webui.Page[webui.NoArgs]{Path: "/p", Body: webui.Table[Device]{
		Load: func(ctx context.Context, _ webui.Query) (webui.Window[Device], webui.Loaded) {
			if started.CompareAndSwap(false, true) {
				close(began)
			}
			<-ctx.Done()
			time.Sleep(50 * time.Millisecond) // what a Load does to let go of what it holds
			finished.Store(true)
			return webui.Window[Device]{}, webui.Ready()
		},
		Columns: []webui.Accessor[Device]{formID},
	}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")

	ctx, leave := context.WithCancel(t.Context())
	defer leave()
	go func() {
		<-began
		leave()
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(ctx, http.MethodGet, "/admin/p", nil))

	assert.True(t, started.Load())
	assert.True(t, finished.Load())
}
