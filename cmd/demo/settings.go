package main

import (
	"context"

	"github.com/Olian04/webui/pkg/webui"
)

func ptr[T any](v T) *T { return &v }

// A page with no arguments says so in its toolbar, rather than showing an empty
// strip.

var (
	CollectorName = webui.String[Settings]{
		Label: "Collector name",
		Load:  func(s Settings) string { return s.Name },
		Store: func(s *Settings, v string) { s.Name = v },
		Rules: webui.StringRules{Required: true, MinLen: 3, MaxLen: 32},
	}
	Port = webui.Int[Settings]{
		Label: "Port",
		Load:  func(s Settings) int { return s.Port },
		Store: func(s *Settings, v int) { s.Port = v },
		// A zero lower bound is a real constraint, so bounds are pointers.
		Rules: webui.NumberRules[int]{Required: true, Min: ptr(1), Max: ptr(65535)},
	}
	Days = webui.Int[RetentionPolicy]{
		Label: "Retention (days)",
		Load:  func(r RetentionPolicy) int { return r.Days },
		Store: func(r *RetentionPolicy, v int) { r.Days = v },
		Rules: webui.NumberRules[int]{Min: ptr(0), Max: ptr(365)},
	}
)

var Ingest = webui.Page[webui.NoArgs]{
	Path: "/settings",
	Nav:  webui.Nav{Label: "Ingest", Section: "Configuration"},
	Body: webui.Form[Settings]{
		Title:  "Ingest",
		Desc:   "Every constraint below is declared as data and rendered as an HTML attribute.",
		Load:   func(context.Context) (Settings, error) { return service.Settings(), nil },
		Fields: []webui.Accessor[Settings]{webui.Group[Settings]{CollectorName, Port}},
		Submit: SaveIngest,
	},
}

var Retention = webui.Page[webui.NoArgs]{
	Path: "/retention",
	Nav:  webui.Nav{Label: "Retention"},
	Body: webui.Form[RetentionPolicy]{
		Title:  "Retention",
		Load:   func(context.Context) (RetentionPolicy, error) { return service.Retention(), nil },
		Fields: []webui.Accessor[RetentionPolicy]{Days},
		Submit: SaveRetention,
	},
}

var SaveIngest = webui.Action[Settings]{
	Guard: canEdit[Settings],
	Run: func(_ context.Context, s Settings) (webui.Effect, error) {
		service.SetSettings(s)
		return webui.Effect{Toast: "Saved"}, nil
	},
}

var SaveRetention = webui.Action[RetentionPolicy]{
	Guard: canEdit[RetentionPolicy],
	Run: func(_ context.Context, r RetentionPolicy) (webui.Effect, error) {
		service.SetRetention(r)
		return webui.Effect{Toast: "Saved"}, nil
	},
}
