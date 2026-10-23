package webui

import (
	"context"
	"time"

	"github.com/Olian04/webui/internal/runtime"
)

// RefreshAfter says that the panel whose Load is running is not final, and is to be asked for again after d: a job that the panel shows is still
// going, and the state of it is what the panel shows. The panel says so in its head ("Updating"), and a page with script asks the server for
// it again after d, replaces it, and does so again for as long as the answer says it; the answer that does not call RefreshAfter ends it.
// A page without script says how long to wait, and the visitor reloads.
//
// It is how a button that starts something long is followed up. The button's action starts the job and returns [Success] as any action does; the
// panel that shows the job asks its own state in its Load, and calls RefreshAfter while it is not done:
//
//	Rows: func(ctx context.Context) ([]Backup, error) {
//	    backups := store.Backups()
//	    if slices.ContainsFunc(backups, Backup.Running) {
//	        webui.RefreshAfter(ctx, 2*time.Second)
//	    }
//	    return backups, nil
//	},
//
// The request that asks again is the same GET that loads the panel, so nothing has to be safe to repeat. d is at least a second. Of several calls
// by one Load the shortest counts, and a call outside a Load does nothing. It is for a panel that shows state: a panel with a form that
// is being filled in is not replaced while a field of it has the focus, but one that is not being filled in is. A [Toolbar]'s values have no
// context to call it with, and do not refresh.
func RefreshAfter(ctx context.Context, d time.Duration) { runtime.RefreshAfter(ctx, d) }
