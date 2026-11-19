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

// frameHeaders is what a page that is meant for a frame says of itself: that nothing may be loaded into it, and that only this site may frame it.
func frameHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'self'")
}

const frameStyle = `<style>
:root{color-scheme:light dark;font:14px/1.4 system-ui,sans-serif}
body{margin:0;padding:14px 16px;background:Canvas;color:CanvasText}
h1{font-size:15px;margin:0 0 2px}
p{margin:0 0 10px;opacity:.75}
svg{display:block;width:100%;max-width:560px;height:auto}
.ok{fill:#2e7d32}.warn{fill:#b26a00}.bad{fill:#b3261e}.dim{fill:currentColor;opacity:.18}
text{fill:currentColor;font:12px system-ui,sans-serif}
</style>`

func framePage(w http.ResponseWriter, title, lead, body string) {
	frameHeaders(w)
	fmt.Fprintf(w, `<!doctype html><html lang="en"><meta charset="utf-8"><title>%s</title>%s<h1>%s</h1><p>%s</p>%s`, html.EscapeString(title), frameStyle, html.EscapeString(title), lead, body)
}

// frameStatus is the collector's status as a board of bars: how full the disk is, how deep the queue is, and whether it is healthy.
func frameStatus(w http.ResponseWriter, r *http.Request) {
	sys := service.System()
	tone := map[string]string{"healthy": "ok", "degraded": "warn", "down": "bad"}[sys.Health]
	if tone == "" {
		tone = "dim"
	}
	queue := float64(sys.Queue)
	if queue > 200 {
		queue = 200
	}
	body := fmt.Sprintf(`<svg viewBox="0 0 560 120" role="img" aria-label="Collector status: %[1]s, disk %[2]d%% used, %[3]d events queued">
<circle cx="24" cy="24" r="10" class="%[4]s"/><text x="44" y="29">%[1]s, version %[5]s, up %[6]s</text>
<text x="0" y="68">Disk used</text><rect x="90" y="56" width="400" height="16" rx="3" class="dim"/><rect x="90" y="56" width="%[7]d" height="16" rx="3" class="%[8]s"/><text x="498" y="69">%[2]d%%</text>
<text x="0" y="104">Queue</text><rect x="90" y="92" width="400" height="16" rx="3" class="dim"/><rect x="90" y="92" width="%[9]d" height="16" rx="3" class="ok"/><text x="498" y="105">%[3]d</text>
</svg>`, html.EscapeString(sys.Health), int(sys.DiskUsed), sys.Queue, tone, html.EscapeString(sys.Version), html.EscapeString(sys.Uptime),
		int(sys.DiskUsed*4), map[bool]string{true: "warn", false: "ok"}[sys.DiskUsed > 85], int(queue*2))
	framePage(w, "At a glance", "A page of this site, drawn on the server with no script. Seen as the "+html.EscapeString(string(RoleOf(r.Context())))+" you are signed in as.", body)
}

// frameDevice is one device among the others at its site, as a map: the device is the large mark, and the others are small ones around it.
func frameDevice(w http.ResponseWriter, r *http.Request) {
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
		angle := float64(i) / float64(max(len(neighbours), 1)) * 6.2831853
		x, y := 280+190*math.Cos(angle), 130+95*math.Sin(angle)
		fmt.Fprintf(&marks, `<circle cx="%.0f" cy="%.0f" r="6" class="dim"><title>%s</title></circle>`, x, y, html.EscapeString(n))
	}
	body := fmt.Sprintf(`<svg viewBox="0 0 560 260" role="img" aria-label="%s at %s, among %d other devices">%s<circle cx="280" cy="130" r="14" class="ok"/><text x="280" y="164" text-anchor="middle">%s</text></svg>`,
		html.EscapeString(device.ID), html.EscapeString(device.Site), len(neighbours), marks.String(), html.EscapeString(device.ID))
	framePage(w, "Site map", html.EscapeString(device.ID)+" at "+html.EscapeString(device.Site)+", with the "+fmt.Sprint(len(neighbours))+" nearest of its site's devices. Seen as the "+html.EscapeString(string(RoleOf(r.Context())))+" you are signed in as.", body)
}

// framePath is the address of the page that shows a device's site, for an Embed's Load to return.
func framePath(deviceID string) string { return "/frame/device/" + url.PathEscape(deviceID) }
