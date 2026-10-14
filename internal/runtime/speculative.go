package runtime

import (
	"net/http"
	"strings"
)

// speculative is whether the browser asks for a page ahead of the visitor, which it does when the
// pointer rests on a link (see the Speculation-Rules header of a full page). It says so in Sec-Purpose.
func speculative(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Sec-Purpose"), "prefetch")
}

// deferred answers a speculative request with 503 while a toast is waiting for the visitor, and says
// whether it did. The toast is read from its cookie by the next page that is drawn, and cleared: a
// page fetched ahead would take it, and be thrown away if the visitor follows another link. A
// response that is not a success is not used by the browser, so the visitor's own request is the one
// that gets the toast.
func deferred(w http.ResponseWriter, r *http.Request) bool {
	if !speculative(r) {
		return false
	}
	if _, err := r.Cookie(flashCookie); err != nil {
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	return true
}
