package main

import (
	"context"

	"github.com/Olian04/webui/pkg/webui"
)

func ptr[T any](v T) *T { return &v }

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
	// A Float has a Precision (decimals shown) and rules like any number. The
	// bounds are pointers, because a zero bound is a real constraint.
	SampleRate = webui.Float[Settings]{
		Label: "Sample rate", Precision: 2,
		Load:  func(s Settings) float64 { return s.SampleRate },
		Store: func(s *Settings, v float64) { s.SampleRate = v },
		Rules: webui.NumberRules[float64]{Required: true, Min: ptr(0.01), Max: ptr(1.0)},
	}
	// A Slider with a Store is a range input in a form, held to its range by the
	// server too; without a Store it is a bar, as the System page shows.
	MaxLoad = webui.Slider[Settings]{
		Label: "Shed load above (%)", Min: 0, Max: 100,
		Load:  func(s Settings) float64 { return s.MaxLoad },
		Store: func(s *Settings, v float64) { s.MaxLoad = v },
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
	Nav:  webui.Nav{Label: "Ingest", Icon: "database", Section: "Configuration"},
	Body: webui.Form[Settings]{
		Title: "Ingest",
		Desc:  "Every constraint below is declared as data and rendered as an HTML attribute.",
		Load:  func(context.Context) (Settings, error) { return service.Settings(), nil },
		Fields: []webui.Accessor[Settings]{
			// A Placeholder decorates an accessor; it shows in an empty input.
			webui.Group[Settings]{webui.Placeholder[Settings]{Accessor: CollectorName, Text: "eu-north-1"}, Port},
			webui.Group[Settings]{SampleRate, MaxLoad},
		},
		Submit: SaveIngest,
	},
}

var Retention = webui.Page[webui.NoArgs]{
	Path: "/retention",
	Nav:  webui.Nav{Label: "Retention", Section: "Configuration"}, // no Icon: its initial, R, stands in when the sidebar is a rail
	Body: webui.Form[RetentionPolicy]{
		Title:  "Retention",
		Load:   func(context.Context) (RetentionPolicy, error) { return service.Retention(), nil },
		Fields: []webui.Accessor[RetentionPolicy]{Days},
		Submit: SaveRetention,
	},
}

var SaveIngest = webui.Action[Settings]{
	Guard: canEdit[Settings],
	Run: func(_ context.Context, s Settings) (webui.Outcome, error) {
		// A reason that belongs to the whole form, not to one field: Failure shows
		// the form again with what was typed kept, and says why in an error toast.
		if s.SampleRate > 0.9 && s.MaxLoad < 50 {
			return webui.Failure("Keeping nearly every event while shedding at a low load would drop most of them"), nil
		}
		service.SetSettings(s)
		// Accepted, but worth knowing: Warning is a Success with something to be aware of.
		if s.MaxLoad < 20 {
			return webui.Warning("Saved, but ingest will start shedding at a very low load"), nil
		}
		return webui.Success("Saved"), nil
	},
}

var SaveRetention = webui.Action[RetentionPolicy]{
	Guard: canEdit[RetentionPolicy],
	Run: func(ctx context.Context, r RetentionPolicy) (webui.Outcome, error) {
		service.SetRetention(r)
		// Then says where to go next, in place of staying on the form: the system
		// page, to see what the policy cost. The toast is shown there.
		return webui.Success("Saved").Then(webui.Open(ctx, System, webui.NoArgs{})), nil
	},
}
