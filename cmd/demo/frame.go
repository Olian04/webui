package main

import (
	"fmt"
	"html"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// The pages that an Embed shows. An Embed shows a page of this site, which is a route of the app's own, and not a page of the app: those say that no page may frame them. These are server-rendered
// markup with an SVG drawn in it and no script, so they are as they are for a reader with none, and the frame that holds them is plain markup too. A route like these is an ordinary request that carries
// the visitor's cookies, so it is behind the same sign-in as the app, and it says in its own policy that only this site may frame it. The frame is not sandboxed: the page is the app's own code.
//
// They look as the app does because they link its stylesheets, from App.Styles: the library's, and the theme that the app was given (so -accent and -palette reach them). What they draw with are the
// library's own classes (a badge) and tokens (--green, --text), which is the one place a colour is said.

// frameHeaders is what a page that is meant for a frame says of itself: that only its own stylesheets may be loaded into it, and that only this site may frame it.
func frameHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'self' 'unsafe-inline'; frame-ancestors 'self'")
}

// frameStyle is what the page draws with that the library's stylesheet has no class for: the fills of the marks in the SVG, from its tokens.
const frameStyle = `<style>
body{height:auto;padding:14px 16px}
h1{font-size:15px;font-weight:600;margin:0 0 2px}
p{margin:0 0 12px;color:var(--text-2)}
svg{display:block;width:100%;max-width:560px;height:auto}
.ok{fill:var(--green)}.warn{fill:var(--orange)}.bad{fill:var(--red)}.dim{fill:var(--border-2)}
text{fill:var(--text);font:12px system-ui,sans-serif}
</style>`

func framePage(w http.ResponseWriter, styles []string, title, lead, body string) {
	frameHeaders(w)
	var links strings.Builder
	for _, href := range styles {
		fmt.Fprintf(&links, `<link rel="stylesheet" href="%s">`, html.EscapeString(href))
	}
	_, _ = fmt.Fprintf(w, `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="color-scheme" content="dark light"><title>%s</title>%s%s<h1>%s</h1><p>%s</p>%s`,
		html.EscapeString(title), links.String(), frameStyle, html.EscapeString(title), lead, body)
}

// frameStatus is the collector's status as a board of bars: how full the disk is, how deep the queue is, and whether it is healthy.
func frameStatus(styles []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sys := service.System()
		tone := map[string]string{"healthy": "ok", "degraded": "warn", "down": "bad"}[sys.Health]
		if tone == "" {
			tone = "dim"
		}
		queue := float64(min(sys.Queue, 200))
		disk := "ok"
		if sys.DiskUsed > 85 {
			disk = "warn"
		}
		body := fmt.Sprintf(`<svg viewBox="0 0 560 120" role="img" aria-label="Collector status: %[1]s, disk %[2]d%% used, %[3]d events queued">
<circle cx="24" cy="24" r="10" class="%[4]s"/><text x="44" y="29">%[1]s, version %[5]s, up %[6]s</text>
<text x="0" y="68">Disk used</text><rect x="90" y="56" width="400" height="16" rx="3" class="dim"/><rect x="90" y="56" width="%[7]d" height="16" rx="3" class="%[8]s"/><text x="498" y="69">%[2]d%%</text>
<text x="0" y="104">Queue</text><rect x="90" y="92" width="400" height="16" rx="3" class="dim"/><rect x="90" y="92" width="%[9]d" height="16" rx="3" class="ok"/><text x="498" y="105">%[3]d</text>
</svg>`, html.EscapeString(sys.Health), int(sys.DiskUsed), sys.Queue, tone, html.EscapeString(sys.Version), html.EscapeString(sys.Uptime), int(sys.DiskUsed*4), disk, int(queue*2))
		badge := map[string]string{"ok": "tone-ok", "warn": "tone-warn", "bad": "tone-bad", "dim": "tone-mute"}[tone]
		lead := fmt.Sprintf(`<span class="badge %s"><span class="dot" aria-hidden="true"></span>%s</span> A page of this site, drawn on the server with no script. Seen as the %s you are signed in as.`,
			badge, html.EscapeString(sys.Health), html.EscapeString(string(RoleOf(r.Context()))))
		framePage(w, styles, "At a glance", lead, body)
	}
}

// frameDevice is one device among the others at its site, as a map: the device is the large mark, and the others are small ones around it.
func frameDevice(styles []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		device, ok := service.Device(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		service.mu.Lock()
		var neighbours []string
		for _, d := range service.devices {
			if d.Site == device.Site && d.ID != id {
				neighbours = append(neighbours, d.ID)
			}
		}
		service.mu.Unlock()
		sort.Strings(neighbours)
		if len(neighbours) > 12 {
			neighbours = neighbours[:12]
		}
		var marks strings.Builder
		for i, n := range neighbours {
			angle := float64(i) / float64(max(len(neighbours), 1)) * 2 * math.Pi
			x, y := 280+190*math.Cos(angle), 130+95*math.Sin(angle)
			fmt.Fprintf(&marks, `<circle cx="%.0f" cy="%.0f" r="6" class="dim"><title>%s</title></circle>`, x, y, html.EscapeString(n))
		}
		body := fmt.Sprintf(`<svg viewBox="0 0 560 260" role="img" aria-label="%s at %s, among %d other devices">%s<circle cx="280" cy="130" r="14" class="ok"/><text x="280" y="164" text-anchor="middle">%s</text></svg>`,
			html.EscapeString(device.ID), html.EscapeString(device.Site), len(neighbours), marks.String(), html.EscapeString(device.ID))
		lead := fmt.Sprintf(`<span class="badge tone-mute">%s</span> %s, with the %d nearest of its site's devices. Seen as the %s you are signed in as.`,
			html.EscapeString(device.Site), html.EscapeString(device.ID), len(neighbours), html.EscapeString(string(RoleOf(r.Context()))))
		framePage(w, styles, "Site map", lead, body)
	}
}

// framePath is the address of the page that shows a device's site, for an Embed's Load to return.
func framePath(deviceID string) string { return "/frame/device/" + url.PathEscape(deviceID) }
