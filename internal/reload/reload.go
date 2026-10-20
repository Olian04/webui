// Package reload is the one place where pkg/reloader reaches into pkg/webui: the compile that counts into the metrics
// of an earlier one. pkg/webui sets it from init, and the public API of neither grows for it.
package reload

import (
	"net/http"

	"github.com/Olian04/webui/internal/metrics"
)

// Compile compiles an app, which is a webui.App, to be mounted at prefix, counting into rec when it is given one, and says
// what the app counts into. It is set when pkg/webui is linked in.
var Compile func(app any, prefix string, rec *metrics.Recorder) (http.Handler, *metrics.Recorder, error)
