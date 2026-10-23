package webui

import (
	"context"
	"time"

	"github.com/Olian04/webui/internal/runtime"
)

// Loaded is how a Load ended, in terms of what the panel is told: it is what a Load returns beside the model, as an action's Run returns an [Outcome]. It is
// built with [Ready] and [Failed], and says more with a method, as an Outcome does. The zero Loaded is [Ready].
//
//   - [Ready] says the model was loaded, and the panel is final.
//   - [Failed] says it was not, and why: the panel says it could not load, the cause is logged and the rest of the page stands. A [Refusal] given to it is
//     told to the visitor, and is not logged.
//   - [Loaded.RefreshAfter] says that the panel is not final, and is to be asked for again.
//
// The error that a Load used to return is Failed's: return m, webui.Failed(err).
type Loaded struct {
	err     error
	refresh time.Duration
}

// Ready says that the model was loaded and the panel is final: what it shows is what there is.
//
//	return model, webui.Ready()
func Ready() Loaded { return Loaded{} }

// Failed says that the model could not be loaded, for a reason that nobody at the page can fix: the panel says that it could not load, the cause is logged
// and not shown, and the rest of the page stands. An err that is a [Refusal] is what the visitor is told instead, and is not logged. A nil err is [Ready].
//
//	return Device{}, webui.Failed(fmt.Errorf("devices: %w", err))
func Failed(err error) Loaded { return Loaded{err: err} }

// RefreshAfter says that the panel is not final, and is to be asked for again after d: a job that the panel shows is still going, and the state of it is what the
// panel shows. The panel says so in its head ("Updating"), and a page with script asks the server for it again after d, replaces it, and does so again for as
// long as the answer says it; the answer that does not ends it. A page without script says how long to wait, and the visitor reloads. It does nothing to a [Failed]
// load, which has nothing to show again.
//
// It is how a button that starts something long is followed up. The button's action starts the job and returns [Success] as any action does; the panel that
// shows the job asks its own state in its Load, and says that it is not done:
//
//	Rows: func(ctx context.Context) ([]Backup, webui.Loaded) {
//	    backups := store.Backups()
//	    if slices.ContainsFunc(backups, Backup.Running) {
//	        return backups, webui.Ready().RefreshAfter(2 * time.Second)
//	    }
//	    return backups, webui.Ready()
//	},
//
// The request that asks again is the same GET that loads the panel, so nothing has to be safe to repeat. d is at least a second. It is for a panel that shows
// state: one with a form that is being filled in is not replaced while a field of it has the focus, but one that is not being filled in is.
func (l Loaded) RefreshAfter(d time.Duration) Loaded {
	l.refresh = max(d, time.Second)
	return l
}

// apply is what the library does with how a Load ended: it says that the panel is not final, for a load that did not fail, and returns the error as the IR holds it,
// where a [Refusal] is the visitor's to be told.
func (l Loaded) apply(ctx context.Context) error {
	if l.err != nil {
		return refused(l.err)
	}
	if l.refresh > 0 {
		runtime.RefreshAfter(ctx, l.refresh)
	}
	return nil
}
