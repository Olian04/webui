package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/render"
)

// minRefresh is the shortest time a panel can ask to be asked for again after, so that none holds the server to a request for each
// instant.
const minRefresh = time.Second

// refreshKey is in the context that a leaf is built with, which holds what its Load says of whether it is final.
type refreshKey struct{}

// refreshHint is what one leaf's Load says: that it is to be asked for again after d. Loads of one leaf may run on goroutines of their own.
type refreshHint struct {
	mu    sync.Mutex
	after time.Duration
}

// RefreshAfter says that the panel being built is not final, and is to be asked for again after d: the panel says so in its HTML, and script asks
// for it again then, until an answer does not say it. It is for a Load, and does nothing outside one. d is at least a second, and of several calls the
// shortest is kept.
func RefreshAfter(ctx context.Context, d time.Duration) {
	h, _ := ctx.Value(refreshKey{}).(*refreshHint)
	if h == nil {
		return
	}
	d = max(d, minRefresh)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.after == 0 || d < h.after {
		h.after = d
	}
}

// refreshing builds a leaf with its own hint in the context, and draws it as one that is not final when its Load said so. Every leaf is built with
// its own, because the panels of a page are built at once, and one says nothing of another.
func refreshing(ctx context.Context, build func(context.Context) templ.Component) templ.Component {
	h := &refreshHint{}
	comp := build(context.WithValue(ctx, refreshKey{}, h))
	h.mu.Lock()
	after := h.after
	h.mu.Unlock()
	if after == 0 {
		return comp
	}
	return render.Refreshing(comp, after)
}
