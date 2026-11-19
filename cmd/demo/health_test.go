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

// The Health checks page is drawn in the order it is written, whichever panel answered first, and a panel that fails, by error or by panic, says so and leaves the rest.
func TestTheHealthChecksAreDrawnInOrderAndAFailingOneSaysSo(t *testing.T) {
	h := handler(t)
	rec := get(h, "/admin/health")
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()

	// The storage checks (400 ms) are slower than the network checks (250 ms) under them, and a Split is in order too: the clock checks wait for the ingest checks.
	last := -1
	for _, check := range []string{"Accepts events", "Clock offset", "Disk space", "DNS resolution", "Collector heartbeat"} {
		at := strings.Index(body, check)
		assert.True(t, at > last)
		last = at
	}

	// A panel whose Load returns an error and one whose Load panics each say that they could not load, the cause is never shown, and the panel after them is drawn.
	assert.Equal(t, strings.Count(body, "Could not load"), 2)
	assert.False(t, strings.Contains(body, "backend fell over"))
	assert.False(t, strings.Contains(body, "is unavailable"))
}

// A visitor who goes before the slowest panel has answered has the checks that were ready: the page is sent as it is built.
func TestAVisitorWhoGoesEarlyHasTheChecksThatWereReady(t *testing.T) {
	h := handler(t)
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/health", nil).WithContext(ctx))
	body := rec.Body.String()
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.True(t, shown(page.Parse(t, body), "Accepts events"))
	assert.False(t, strings.Contains(body, "Disk space")) // the storage checks had not answered
}

// The page is read with its tab, and a request for the Tabs alone is read without the rest: the time a panel outside it was read at is the page's, and only the history of the tab that
// was asked for is read again.
func TestTheHealthHistoryTabsReadOnlyTheTabThatIsAskedFor(t *testing.T) {
	h := handler(t)
	whole := pageAt(t, h, "/admin/health")
	assert.True(t, shown(whole, "Last sweep"))
	assert.True(t, shown(whole, "Open findings"))
	assert.True(t, shown(whole, "Ingest history"))

	req := httptest.NewRequest(http.MethodGet, "/admin/health?tabs.tab=storage", nil)
	req.Header.Set("X-Webui-View", "p.4")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	assert.Equal(t, rec.Code, http.StatusOK)
	alone := page.Parse(t, body)
	assert.True(t, page.IsFragment(body))
	assert.Equal(t, len(alone.ByAttr("data-tabs", "p.4")), 1)
	assert.True(t, shown(alone, "Storage history"))
	assert.False(t, strings.Contains(body, "Last sweep"))
	assert.False(t, strings.Contains(body, "Ingest history"))
}
