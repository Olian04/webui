package main

import (
	"context"

	"github.com/Olian04/webui/pkg/webui"
)

var (
	CollectorName = webui.String[Settings]{
		Label: "Collector name",
		Load:  func(s Settings) string { return s.Name },
		Store: func(s *Settings, v string) { s.Name = v },
		// The text rules and the pattern are all one pattern to the browser, which says which of them a value fails, in
		// the words that the server uses. A pattern is written in the syntax that Go and a browser both read.
		Rules: webui.StringRules{
			Required: true, MinLen: 3, MaxLen: 32,
			NotStartsWith: "-", NotEndsWith: "-", NotContains: "--",
			Pattern: &webui.PatternRule{Expr: `[a-z0-9\-]+`, Message: "must be lowercase letters, digits and dashes"},
		},
		// Shown in the input while it is empty.
		Placeholder: "eu-north-1",
	}
	// A Secret is a password input for a value that is never read back: the stored key is not in the page, and
	// submitting it empty leaves it as it is, so only a key that is typed is stored.
	APIKey = webui.String[Settings]{
		Label:       "API key",
		Load:        func(s Settings) string { return s.APIKey },
		Store:       func(s *Settings, v string) { s.APIKey = v },
		Secret:      true,
		Rules:       webui.StringRules{MinLen: 8, MaxLen: 128},
		Placeholder: "Leave empty to keep it",
	}
	Port = webui.Int[Settings]{
		Label: "Port",
		Load:  func(s Settings) int { return s.Port },
		Store: func(s *Settings, v int) { s.Port = v },
		// A zero lower bound is a real constraint, so bounds are pointers.
		Rules: webui.NumberRules[int]{Required: true, Min: new(1), Max: new(65535)},
	}
	// A Float has a Precision (decimals shown) and rules like any number. The
	// bounds are pointers, because a zero bound is a real constraint.
	SampleRate = webui.Float[Settings]{
		// Help is what the field is for, in the popover beside its label, above the rules: the label and the rules cannot say that
		// 0.25 keeps one event in four.
		Label: "Sample rate", Precision: 2, Help: "The share of events that are kept: 0.25 keeps one in four. Lower it when ingest is slow.",
		Load:  func(s Settings) float64 { return s.SampleRate },
		Store: func(s *Settings, v float64) { s.SampleRate = v },
		Rules: webui.NumberRules[float64]{Required: true, Min: new(0.01), Max: new(1.0)},
	}
	// A Slider with a Store is a range input in a form, held to its range by the
	// server too; without a Store it is a bar, as the System page shows.
	MaxLoad = webui.Slider[Settings]{
		Label: "Shed load above (%)", Min: 0, Max: 100, Help: "Above this load, ingest starts dropping the events that are not marked critical.",
		Load:  func(s Settings) float64 { return s.MaxLoad },
		Store: func(s *Settings, v float64) { s.MaxLoad = v },
	}
	// An OptionalInt is a number that may be absent: emptied, it is stored as nil, which here means no limit, and
	// that is not the same as a limit of zero. The same holds for an OptionalFloat and an OptionalSlider.
	Burst = webui.OptionalInt[Settings]{
		Label: "Burst limit (events/s)", Help: "Events a second let through at once. Empty is no limit, which is not the same as 0, which lets none through.",
		Load:  func(s Settings) *int { return s.Burst },
		Store: func(s *Settings, v *int) { s.Burst = v },
		Rules: webui.NumberRules[int]{Min: new(0), Max: new(100000)},
		// Shown in the input while it is empty.
		Placeholder: "No limit",
	}
	Quota = webui.OptionalSlider[Settings]{
		Label: "Quota (%)", Min: 0, Max: 100, StepSize: 5,
		Load:  func(s Settings) *float64 { return s.Quota },
		Store: func(s *Settings, v *float64) { s.Quota = v },
	}
	// A Bool is yes or no, and is a checkbox: an unchecked one is not sent, so an absent field is no.
	Compress = webui.Bool[Settings]{
		Label: "Compress events", Help: "Compress what is sent to the sinks. Saves bandwidth and costs some CPU.",
		Load:  func(s Settings) bool { return s.Compress },
		Store: func(s *Settings, v bool) { s.Compress = v },
	}
	// An OptionalBool is yes, no, or not set, which a checkbox cannot say: it is a choice with an empty one.
	Paused = webui.OptionalBool[Settings]{
		Label: "Paused",
		Load:  func(s Settings) *bool { return s.Paused },
		Store: func(s *Settings, v *bool) { s.Paused = v },
	}
	// Email, Phone, Website and ColorPicker are strings of a kind that has an input of its own (the right keyboard on a phone,
	// a picker for a colour), checked again on the server. An email is held to what the browser holds it to; a web address
	// to more, an http or https address, since what is stored will be linked.
	Contact = webui.Email[Settings]{
		Label: "Contact email",
		Load:  func(s Settings) string { return s.Contact },
		Store: func(s *Settings, v string) { s.Contact = v },
	}
	// A Phone is stored as E.164, "+46701234567", whatever it is typed as. It is a selector of the country calling code and
	// the national number, which script spaces as it is typed; DefaultCountry is where the selector starts.
	OnCall = webui.Phone[Settings]{
		Label: "On-call phone", DefaultCountry: "SE",
		Load:  func(s Settings) string { return s.OnCall },
		Store: func(s *Settings, v string) { s.OnCall = v },
	}
	Webhook = webui.Website[Settings]{
		Label: "Alert webhook", Placeholder: "https://hooks.example.com/…",
		Load:  func(s Settings) string { return s.Webhook },
		Store: func(s *Settings, v string) { s.Webhook = v },
	}
	// A color is picked, and is "#rrggbb". It has no rules: a picker cannot be empty or be of any other shape.
	Label = webui.ColorPicker[Settings]{
		Label: "Graph colour",
		Load:  func(s Settings) string { return s.Label },
		Store: func(s *Settings, v string) { s.Label = v },
	}
	// DateOnly makes a moment a date: the input picks a day, and it is stored as midnight UTC.
	CertEnds = webui.Timestamp[Settings]{
		Label: "Certificate ends", DateOnly: true,
		Load:  func(s Settings) int { return s.CertEnds },
		Store: func(s *Settings, v int) { s.CertEnds = v },
	}
	// A Timestamp with a Store is a date and time picker, in UTC, that stores Unix
	// seconds. A Datetime stores an ISO 8601 string the same way.
	Maintenance = webui.Timestamp[Settings]{
		Label: "Maintenance window starts",
		Load:  func(s Settings) int { return s.Maintain },
		Store: func(s *Settings, v int) { s.Maintain = v },
	}
	Days = webui.Int[RetentionPolicy]{
		Label: "Retention (days)",
		Load:  func(r RetentionPolicy) int { return r.Days },
		Store: func(r *RetentionPolicy, v int) { r.Days = v },
		Rules: webui.NumberRules[int]{Min: new(0), Max: new(365)},
	}
)

var Ingest = webui.Page[webui.NoArgs]{
	Path: "/settings",
	Nav:  webui.Nav{Label: "Ingest", Icon: "database", Section: "Configuration"},
	Body: webui.Form[Settings]{
		Title: "Ingest",
		Desc:  "Every constraint below is declared as data and rendered as an HTML attribute.",
		Load:  func(context.Context) (Settings, webui.Loaded) { return service.Settings(), webui.Ready() },
		Fields: []webui.Accessor[Settings]{
			webui.Group[Settings]{CollectorName, Port},
			webui.Group[Settings]{SampleRate, MaxLoad},
			Maintenance,
			APIKey,
			webui.Group[Settings]{Burst, Quota},
			webui.Group[Settings]{Paused, Compress},
			webui.Group[Settings]{Contact, OnCall},
			webui.Group[Settings]{Webhook, Label},
			CertEnds,
			webui.Group[Settings]{Logo, Certificate},
		},
		Submit: SaveIngest,
	},
}

var Retention = webui.Page[webui.NoArgs]{
	Path: "/retention",
	Nav:  webui.Nav{Label: "Retention and archival policy for every kind of collected event", Section: "Configuration"}, // no Icon: its initial, R, stands in when the sidebar is a rail; the label is wider than the sidebar, so it is cut short with an ellipsis
	Body: webui.Form[RetentionPolicy]{
		Title:  "Retention",
		Load:   func(context.Context) (RetentionPolicy, webui.Loaded) { return service.Retention(), webui.Ready() },
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
		// Stay keeps the user on the form, which is a page they go on tuning: a saved form
		// would otherwise return to the page that opened it.
		if s.MaxLoad < 20 {
			return webui.Warning("Saved, but ingest will start shedding at a very low load").Stay(), nil
		}
		return webui.Success("Saved").Stay(), nil
	},
}

var SaveRetention = webui.Action[RetentionPolicy]{
	// On a form, the question is asked when Save is clicked, and pressing Enter in a field does not save.
	Confirm: "Save the retention policy? Events older than it are deleted.",
	Guard:   canEdit[RetentionPolicy],
	Run: func(ctx context.Context, r RetentionPolicy) (webui.Outcome, error) {
		service.SetRetention(r)
		// Then says where to go next, in place of staying on the form: the system
		// page, to see what the policy cost. The toast is shown there.
		return webui.Success("Saved").Then(webui.Open(ctx, System, webui.NoArgs{})), nil
	},
}
