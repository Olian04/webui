package webui_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Olian04/webui/pkg/webui"
)

// memo keeps what load returned for ttl, joins the requests that arrive while it is loading, and keeps no error.
// The load runs without the cancellation of the request that started it, so that a visitor who leaves does not
// fail the others waiting for it. It is the app's: the library does not cache.
type memo[T any] struct {
	ttl time.Duration

	mu      sync.Mutex
	value   T
	at      time.Time
	loading *flight[T]
}

type flight[T any] struct {
	done  chan struct{}
	value T
	err   error
}

func (m *memo[T]) get(ctx context.Context, load func(context.Context) (T, error)) (T, error) {
	m.mu.Lock()
	if !m.at.IsZero() && time.Since(m.at) < m.ttl {
		defer m.mu.Unlock()
		return m.value, nil
	}
	f := m.loading
	if f == nil {
		f = &flight[T]{done: make(chan struct{})}
		m.loading = f
		go func() {
			f.value, f.err = load(context.WithoutCancel(ctx))
			m.mu.Lock()
			if f.err == nil {
				m.value, m.at = f.value, time.Now()
			}
			m.loading = nil
			m.mu.Unlock()
			close(f.done)
		}()
	}
	m.mu.Unlock()

	select {
	case <-f.done:
		return f.value, f.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// invalidate forgets what was kept, for the action that changed it.
func (m *memo[T]) invalidate() {
	m.mu.Lock()
	m.at = time.Time{}
	m.mu.Unlock()
}

// A table whose rows come from a slow source, kept for five seconds and shared by every request, and a form
// that forgets them when it saves. The library keeps nothing: the cache is the app's, here a few lines of it.
func Example_caching() {
	type device struct{ ID, IP string }
	id := webui.String[device]{Label: "ID", Load: func(d device) string { return d.ID }}
	ip := webui.String[device]{Label: "IP", Load: func(d device) string { return d.IP }, Store: func(d *device, v string) { d.IP = v }}

	var listings int
	devices := &memo[[]device]{ttl: 5 * time.Second}
	list := func(ctx context.Context) ([]device, error) {
		return devices.get(ctx, func(context.Context) ([]device, error) {
			listings++ // the slow call: a query, or another service
			return []device{{"dev_1", "10.0.0.1"}, {"dev_2", "192.168.1.2"}}, nil
		})
	}

	page := webui.Page[webui.NoArgs]{
		Path: "/devices",
		Body: webui.Stack{
			// Rows is called on every page load, sort, filter and page change: all served from one listing.
			webui.Table[device]{Title: "Devices", Rows: list, Columns: []webui.Accessor[device]{id, ip}},
			webui.Form[device]{
				Title:  "Edit",
				Load:   func(context.Context) (device, error) { return device{ID: "dev_1", IP: "10.0.0.1"}, nil },
				Fields: []webui.Accessor[device]{id, ip},
				Submit: webui.Action[device]{Run: func(context.Context, device) (webui.Outcome, error) {
					devices.invalidate() // what it changed is not what the table shows any more
					return webui.Success("Saved"), nil
				}},
			},
		},
	}
	_ = webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")

	ctx := context.Background()
	for range 3 {
		_, _ = list(ctx) // three requests inside five seconds
	}
	fmt.Println("listings after three requests:", listings)
	devices.invalidate() // as the action does
	_, _ = list(ctx)
	fmt.Println("listings after a save:", listings)
	// Output:
	// listings after three requests: 1
	// listings after a save: 2
}
