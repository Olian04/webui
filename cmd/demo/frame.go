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
// They look as the app does because they link the stylesheets that App.Styles names: the classless one, which gives plain elements (a heading, a paragraph, a table) the app's look from its tokens, and
// the theme that the app was given (so -accent and -palette reach them). There is no class in these pages: what they are promised is that semantic markup looks like the app, and a mark in the SVG
// is coloured from a token (--success-color), the one place a colour is said.

// frameHeaders is what a page that is meant for a frame says of itself: that only its own stylesheets may be loaded into it, and that only this site may frame it.
func frameHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'self' 'unsafe-inline'; frame-ancestors 'self'") // 'unsafe-inline' is for the style attributes of the SVG's marks
}

// svgStyle is the one thing that a page cannot get from plain elements: the SVG's own box, and the fills of its marks, from the tokens.
const svgStyle = ` style="display:block;width:100%;max-width:560px;height:auto"`

// fill is the style of a mark in the SVG: a colour from a token of the design, by what the mark says.
func fill(tone string) string {
	token := map[string]string{"ok": "--success-color", "warn": "--warning-color", "bad": "--danger-color", "dim": "--border-strong-color"}[tone]
	if token == "" {
		token = "--border-strong-color"
	}
	return ` style="fill:var(` + token + `)"`
}

const textStyle = ` style="fill:var(--text-color);font:12px system-ui,sans-serif"`

func framePage(w http.ResponseWriter, styles []string, title, lead, body string) {
	frameHeaders(w)
	var links strings.Builder
	for _, href := range styles {
		fmt.Fprintf(&links, `<link rel="stylesheet" href="%s">`, html.EscapeString(href))
	}
	_, _ = fmt.Fprintf(w, `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="color-scheme" content="dark light"><title>%s</title>%s<h1>%s</h1><p>%s</p>%s`,
		html.EscapeString(title), links.String(), html.EscapeString(title), lead, body)
}

// frameStatus is the collector's status: the facts in a table, and as a board of bars how full the disk is and how deep the queue is.
func frameStatus(styles []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sys := service.System()
		queue := min(sys.Queue, 200)
		disk := "ok"
		if sys.DiskUsed > 85 {
			disk = "warn"
		}
		bar := func(y int, label, value string, width int, tone string) string {
			return fmt.Sprintf(`<text x="0" y="%d"%s>%s</text><rect x="90" y="%d" width="400" height="16" rx="3"%s/><rect x="90" y="%d" width="%d" height="16" rx="3"%s/><text x="498" y="%d"%s>%s</text>`,
				y+12, textStyle, label, y, fill("dim"), y, width, fill(tone), y+13, textStyle, value)
		}
		board := fmt.Sprintf(`<svg viewBox="0 0 560 60"%s role="img" aria-label="Collector status: %s, disk %d%% used, %d events queued">%s%s</svg>`,
			svgStyle, html.EscapeString(sys.Health), int(sys.DiskUsed), sys.Queue,
			bar(0, "Disk used", fmt.Sprintf("%d%%", int(sys.DiskUsed)), int(sys.DiskUsed*4), disk), bar(36, "Queue", fmt.Sprint(sys.Queue), queue*2, "ok"))
		facts := fmt.Sprintf(`<table><thead><tr><th>Fact</th><th>Now</th></tr></thead><tbody><tr><td>Health</td><td>%s</td></tr><tr><td>Version</td><td>%s</td></tr><tr><td>Uptime</td><td>%s</td></tr></tbody></table>`,
			html.EscapeString(sys.Health), html.EscapeString(sys.Version), html.EscapeString(sys.Uptime))
		lead := fmt.Sprintf(`A page of this site, drawn on the server with no script, at <a href="/frame/status"><code>/frame/status</code></a>. Seen as the <strong>%s</strong> you are signed in as.`, html.EscapeString(string(RoleOf(r.Context()))))
		framePage(w, styles, "At a glance", lead, facts+board)
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
			fmt.Fprintf(&marks, `<circle cx="%.0f" cy="%.0f" r="6"%s><title>%s</title></circle>`, x, y, fill("dim"), html.EscapeString(n))
		}
		facts := fmt.Sprintf(`<table><thead><tr><th>Device</th><th>Address</th><th>Site</th><th>Status</th></tr></thead><tbody><tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr></tbody></table>`,
			html.EscapeString(device.ID), html.EscapeString(device.IP), html.EscapeString(device.Site), html.EscapeString(device.Status))
		svg := fmt.Sprintf(`<svg viewBox="0 0 560 260"%s role="img" aria-label="%s at %s, among %d other devices">%s<circle cx="280" cy="130" r="14"%s/><text x="280" y="164" text-anchor="middle"%s>%s</text></svg>`,
			svgStyle, html.EscapeString(device.ID), html.EscapeString(device.Site), len(neighbours), marks.String(), fill("ok"), textStyle, html.EscapeString(device.ID))
		lead := fmt.Sprintf(`The device and the %d nearest of its site's devices. Seen as the <strong>%s</strong> you are signed in as.`, len(neighbours), html.EscapeString(string(RoleOf(r.Context()))))
		framePage(w, styles, "Site map", lead, facts+svg)
	}
}

// framePath is the address of the page that shows a device's site, for an Embed's Load to return.
func framePath(deviceID string) string { return "/frame/device/" + url.PathEscape(deviceID) }
