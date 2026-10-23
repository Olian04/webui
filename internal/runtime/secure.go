package runtime

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Olian04/webui/internal/ir"
)

// csp allows what the shell uses and nothing else. Inline styles stay because
// the components size bars and skeletons with style attributes; scripts are
// ours alone, so an injected one would not run.
const csp = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
	"object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

// policy is the content security policy of a page that draws pictures from origins: the
// same as everyone else's, with them added to img-src. Every other page's is csp,
// byte for byte. An origin that could break out of the directive is left out, though
// the declaration was checked when the app was compiled.
func policy(imgOrigins []string) string {
	var allowed []string
	for _, o := range imgOrigins {
		if strings.HasPrefix(o, "https://") && !strings.ContainsAny(o, " ;,'\"\\\n\r\t") {
			allowed = append(allowed, o)
		}
	}
	if len(allowed) == 0 {
		return csp
	}
	return strings.Replace(csp, "img-src 'self' data:", "img-src 'self' data: "+strings.Join(allowed, " "), 1)
}

// securedFor sets the headers every response carries, with the content security policy it is given.
// The context of the request carries the app's logger, for the code that is called with no more than a value.
func securedFor(policy string, log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(ir.WithLogger(context.WithValue(r.Context(), startedKey{}, time.Now()), log))
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", policy)
		next.ServeHTTP(w, r)
	})
}

// startedKey is where the time a request arrived is kept in its context, for the time it took to
// build the page that answers it.
type startedKey struct{}

// startedAt is when the request arrived.
func startedAt(r *http.Request) time.Time {
	if started, ok := r.Context().Value(startedKey{}).(time.Time); ok {
		return started
	}
	return time.Now()
}

// timing stamps a response with how long it took to build from the request's arrival, as Server-Timing, for the browser's own tools. It
// returns that and when it was built, so that the status bar can say both in the HTML: a full page in its bar, a panel in an element
// that follows it (see render.StatusUpdate).
func timing(w http.ResponseWriter, r *http.Request) (built time.Time, took time.Duration) {
	built = time.Now().UTC()
	if started, ok := r.Context().Value(startedKey{}).(time.Time); ok {
		took = built.Sub(started)
	}
	w.Header().Set("Server-Timing", "build;dur="+strconv.FormatFloat(float64(took.Microseconds())/1000, 'f', 1, 64))
	return built, took
}
