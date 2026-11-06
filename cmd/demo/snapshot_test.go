package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSnapshot is how a change to the renderer or the runtime that is meant to change nothing is held to it: it writes every page of the demo, for an editor and for a viewer, to the
// directory in SNAP, with what differs between two runs of the same code (the time, the build time, the key that addresses an editor's files) made the same. Run it on main and on the
// branch, and diff the directories:
//
//	git worktree add /tmp/main origin/main && (cd /tmp/main && SNAP=/tmp/snap-main go test ./cmd/demo -run TestSnapshot -count=1)
//	SNAP=/tmp/snap-branch go test ./cmd/demo -run TestSnapshot -count=1
//	diff -rq /tmp/snap-main /tmp/snap-branch
//
// An empty diff is the same markup, byte for byte, on every page, which the tests that assert on parts of a page cannot say. It does nothing without SNAP.
func TestSnapshot(t *testing.T) {
	dir := os.Getenv("SNAP")
	if dir == "" {
		t.Skip("set SNAP to a directory to write the pages to")
	}
	same := []*regexp.Regexp{
		regexp.MustCompile(`datetime="[^"]*"`), regexp.MustCompile(`The HTML was built [^"]*"`), regexp.MustCompile(`>built [0-9:]+Z<`),
		regexp.MustCompile(`(data-build[^>]*>)[0-9.]+ ms<`), regexp.MustCompile(`[0-9]+(\.[0-9]+)? ms`),
		regexp.MustCompile(`[A-Z][a-z]{2} [0-9]{1,2} [0-9]{2}:[0-9]{2}`), regexp.MustCompile(`>[0-9]{2}:[0-9]{2}<`), regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}`),
		regexp.MustCompile(`[0-9]+ (second|minute|hour)s? ago`), regexp.MustCompile(`\?k=[A-Za-z0-9_-]+`), regexp.MustCompile(`/code/[0-9a-f]+\.`),
		regexp.MustCompile(`\.[0-9a-f]{8}\.(css|js|woff|png|svg)`),
	}
	paths := []string{
		"/admin/", "/admin/device", "/admin/alert", "/admin/settings", "/admin/rule", "/admin/layout/review", "/admin/handbook", "/admin/activity", "/admin/audit", "/admin/compare",
		"/admin/site", "/admin/retention", "/admin/config", "/admin/system", "/admin/firmware", "/admin/files", "/admin/layout/toolbars", "/admin/layout/split", "/admin/layout/tabs",
		"/admin/loading/order", "/admin/nowhere", "/admin/device?devices.sort=id&devices.offset=10", "/admin/alert?open-alerts.sort=severity",
	}
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // G703: the directory is the one that whoever runs this asked for, in the environment
		t.Fatal(err)
	}
	name := strings.NewReplacer("/", "_", "?", "_", "&", "_", "=", "_")
	for _, who := range []struct {
		role string
		site http.Handler
	}{{"editor", handlerAs(t, Editor)}, {"viewer", handlerAs(t, Viewer)}} {
		for _, path := range paths {
			rec := httptest.NewRecorder()
			who.site.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			body := rec.Body.String()
			for _, re := range same {
				body = re.ReplaceAllString(body, "X")
			}
			if err := os.WriteFile(filepath.Join(dir, who.role+name.Replace(path)+".html"), []byte(body), 0o600); err != nil { //nolint:gosec // G703: as above, and the name is made of the test's own paths
				t.Fatal(err)
			}
		}
	}
}
