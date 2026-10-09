package main

import (
	"context"

	"github.com/Olian04/webui/pkg/webui"
)

var (
	Version = webui.String[SystemInfo]{
		Label: "Version",
		Load:  func(s SystemInfo) string { return s.Version },
	}
	Health = webui.Badge[SystemInfo]{
		Label: "Health",
		Load:  func(s SystemInfo) string { return s.Health },
		Kinds: map[string]webui.Tone{"healthy": webui.ToneOK, "degraded": webui.ToneWarning, "down": webui.ToneCritical},
	}
	Uptime = webui.String[SystemInfo]{
		Label: "Uptime",
		Load:  func(s SystemInfo) string { return s.Uptime },
	}
	DiskUsed = webui.Slider[SystemInfo]{
		Label: "Disk used (%)", Min: 0, Max: 100, Precision: 0,
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

// SystemStatus is also on the landing page.
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
		Desc:     "The total is not known: Load returns a window and no count, so the pager offers Next while a page is full.",
		PageSize: 8,
		Load: func(_ context.Context, q webui.Query) (webui.Window[AuditEntry], error) {
			// Rows.Total is left zero: unknown, not "zero rows". The table pages by
			// "a full page may have a successor". (Sort and filters are not offered
			// by this source, so Load ignores them.)
			return webui.Window[AuditEntry]{Items: service.Audit(q.Offset, q.Limit)}, nil
		},
		Columns: []webui.Accessor[AuditEntry]{AuditAt, AuditActor, AuditAction, AuditTarget},
	},
}

// The landing page is the page at "/". It has no Nav of its own: the brand in
// the sidebar and the first breadcrumb both lead here. Without such a page the
// root goes to the first entry in the navigation.
var Overview = webui.Page[webui.NoArgs]{
	Path: "/",
	Body: webui.Stack{SystemStatus, SitesTable},
}
