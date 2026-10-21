package main

import (
	"context"
	"strconv"

	"github.com/Olian04/webui/pkg/webui"
)

var (
	Version = webui.String[SystemInfo]{
		Label: "Version",
		Load:  func(s SystemInfo) string { return s.Version },
	}
	Health = webui.Select[SystemInfo]{
		Label: "Health",
		Load:  func(s SystemInfo) []string { return []string{s.Health} },
		Options: []webui.Option{
			{Value: "healthy", Tone: webui.ToneOK}, {Value: "degraded", Tone: webui.ToneWarning}, {Value: "down", Tone: webui.ToneCritical},
		},
	}
	Uptime = webui.String[SystemInfo]{
		Label: "Uptime",
		Load:  func(s SystemInfo) string { return s.Uptime },
	}
	DiskUsed = webui.Slider[SystemInfo]{
		Label: "Disk used (%)", Min: 0, Max: 100,
		Load: func(s SystemInfo) float64 { return s.DiskUsed },
	}
	Queue = webui.Int[SystemInfo]{
		Label: "Queue depth",
		Load:  func(s SystemInfo) int { return s.Queue },
	}
)

// A Form whose Submit has no Run is read-only: no submit button, and every field
// shown as a value. Here that is a status page, with a badge and a bar.
var System = webui.Page[webui.NoArgs]{
	Path: "/system",
	Nav:  webui.Nav{Label: "System", Icon: "wave-square", Section: "Operations"},
	Body: SystemStatus,
}

// SystemStatus is the status page.
var SystemStatus = webui.Form[SystemInfo]{
	Title:  "Collector",
	Desc:   "A Form with no Submit: read-only, nothing to save.",
	Load:   func(context.Context) (SystemInfo, error) { return service.System(), nil },
	Fields: []webui.Accessor[SystemInfo]{webui.Group[SystemInfo]{Version, Health}, webui.Group[SystemInfo]{Uptime, Queue}, DiskUsed},
}

var (
	AuditAt     = webui.String[AuditEntry]{Label: "When", Load: func(e AuditEntry) string { return e.At }}
	AuditActor  = webui.String[AuditEntry]{Label: "Who", Load: func(e AuditEntry) string { return e.Actor }}
	AuditAction = webui.String[AuditEntry]{Label: "What", Load: func(e AuditEntry) string { return e.Action }}
	AuditTarget = webui.String[AuditEntry]{Label: "Target", Load: func(e AuditEntry) string { return e.Target }}
)

// The audit log is for editors. The page Guard runs before anything is loaded: as
// a viewer the page is the "Not permitted" state, and the log was never read.
var Audit = webui.Page[webui.NoArgs]{
	Path:  "/audit",
	Nav:   webui.Nav{Label: "Audit log", Icon: "list", Section: "Operations"},
	Guard: canEdit[webui.NoArgs],
	Body: webui.Table[AuditEntry]{
		Title:    "Audit log",
		Desc:     "A Feed: a source that pages by a cursor and hands out its rows newest first, in its own order. A cursor continues one sequence, so there are no filters, and a column sorts only the page on screen: the next page continues the sequence and is ordered within itself.",
		PageSize: 8,
		// A feed is exported from its first page to its last, whatever page is on screen.
		Export: true,
		Feed: func(_ context.Context, after string, limit int) ([]AuditEntry, string, error) {
			// The cursor is where the page starts. It is whatever the source likes, so long
			// as it can read it back; here, an index. Asking for one row more than a page is
			// how this source knows there is a next one.
			start, _ := strconv.Atoi(after)
			rows := service.Audit(start, limit+1)
			if len(rows) <= limit {
				return rows, "", nil
			}
			return rows[:limit], strconv.Itoa(start + limit), nil
		},
		Columns: []webui.Accessor[AuditEntry]{AuditAt, AuditActor, AuditAction, AuditTarget},
	},
}

// The landing page is the page at "/". It has no Nav of its own: the brand in
// the sidebar and the first breadcrumb both lead here. Without such a page the
// root goes to the first page it can find that needs no arguments, with a warning.
var Overview = webui.Page[webui.NoArgs]{
	Path: "/",
	Body: webui.Stack{OverviewToolbar, SitesTable},
}

// The values of the landing page's toolbar are accessors of the page's arguments, which this page has none of: their Load reads
// the system instead, which a toolbar may do, since a value is only read. A Store would do nothing here, and none is given.
var (
	overviewHealth = webui.Select[webui.NoArgs]{
		Label: "Health",
		Load:  func(webui.NoArgs) []string { return []string{service.System().Health} },
		Options: []webui.Option{
			{Value: "healthy", Tone: webui.ToneOK}, {Value: "degraded", Tone: webui.ToneWarning}, {Value: "down", Tone: webui.ToneCritical},
		},
	}
	overviewVersion = webui.String[webui.NoArgs]{Label: "Version", Load: func(webui.NoArgs) string { return service.System().Version }}
	overviewUptime  = webui.String[webui.NoArgs]{Label: "Uptime", Load: func(webui.NoArgs) string { return service.System().Uptime }}
	overviewQueue   = webui.Int[webui.NoArgs]{
		Label: "Queue depth", Help: "Events that are waiting to be written.",
		Load: func(webui.NoArgs) int { return service.System().Queue },
	}
	overviewDisk = webui.Slider[webui.NoArgs]{
		Label: "Disk used (%)", Min: 0, Max: 100,
		Load: func(webui.NoArgs) float64 { return service.System().DiskUsed },
	}
)

// OverviewToolbar is the status of the collector at a glance, in the row that is a Toolbar: the values of the system, read-only, from the
// left edge. A toolbar has no inputs; it is only values here, and so it has no form either.
var OverviewToolbar = webui.Toolbar[webui.NoArgs]{
	Left: []webui.ToolbarItem[webui.NoArgs]{overviewHealth, overviewVersion, overviewUptime, overviewQueue, overviewDisk},
}
