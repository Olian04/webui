package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// The Probes page is drawn in the order it is written, whichever panel answered first, and a panel that fails, by error or by panic, says so and leaves the rest.
func TestTheProbesAreDrawnInOrderAndAFailingOneSaysSo(t *testing.T) {
	h := handler(t)
	rec := get(h, "/admin/probes")
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()

	// The storage probes (400 ms) are slower than the network probes (250 ms) under them, and a Split is in order too: the clock probes wait for the queue probes.
	last := -1
	for _, panel := range []string{"Queue probes (300 ms) 1", "Clock probes (20 ms) 1", "Storage probes (400 ms) 1", "Network probes (250 ms) 1", "Heartbeat probes (10 ms) 1"} {
		at := strings.Index(body, panel)
		assert.True(t, at > last)
		last = at
	}

	// A panel whose Load returns an error and one whose Load panics each say that they could not load, the cause is never shown, and the panel after them is drawn.
	assert.Equal(t, strings.Count(body, "Could not load"), 2)
	assert.False(t, strings.Contains(body, "backend fell over"))
	assert.False(t, strings.Contains(body, "is unavailable"))
}

// The probes of a visitor who goes after the first panel's time are those that were ready: the page is sent as it is built.
func TestAVisitorWhoGoesEarlyHasTheProbesThatWereReady(t *testing.T) {
	h := handler(t)
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/probes", nil).WithContext(ctx))
	body := rec.Body.String()
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.True(t, shown(page.Parse(t, body), "Queue probes (300 ms) 1"))
	assert.False(t, strings.Contains(body, "Storage probes (400 ms) 1")) // it had not answered
}

// The probes page is read with its tab, and a request for the Tabs alone is read without the rest: the stamp of a panel outside it is the moment that panel was read, and only the panels
// of the tab that was asked for have a new one.
func TestTheProbesTabsReadOnlyTheTabThatIsAskedFor(t *testing.T) {
	h := handler(t)
	whole := pageAt(t, h, "/admin/probes")
	assert.True(t, shown(whole, "Last full sweep (read with the page)"))
	assert.True(t, shown(whole, "Sweep summary (read with the page)"))
	assert.True(t, shown(whole, "Ingest probes (500 ms)"))

	req := httptest.NewRequest(http.MethodGet, "/admin/probes?tabs.tab=storage", nil)
	req.Header.Set("X-Webui-View", "p.4")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	assert.Equal(t, rec.Code, http.StatusOK)
	alone := page.Parse(t, body)
	assert.True(t, page.IsFragment(body))
	assert.Equal(t, len(alone.ByAttr("data-tabs", "p.4")), 1)
	assert.True(t, shown(alone, "Storage sweep (500 ms)"))
	assert.False(t, strings.Contains(body, "Last full sweep"))
	assert.False(t, strings.Contains(body, "Ingest probes"))
}
