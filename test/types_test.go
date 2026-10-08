package webui_test

import (
	"context"
	"image"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
)

type Device struct {
	Id       string
	Ip       string
	Count    int
	Duration float64
}

type DetailsArgs struct {
	Id     string
	Debug  bool
	Offset int
}

func TestSketchTypes(t *testing.T) {
	t.Parallel()

	var DevicesNav = webui.Nav{Label: "Devices"}

	var (
		ID = webui.String[Device]{
			Label: "ID",
			Load:  func(d Device) string { return d.Id },
		}
		IP = webui.String[Device]{
			Label: "IP",
			Load:  func(d Device) string { return d.Ip },
			Store: func(d *Device, val string) { d.Ip = val },
			Rules: webui.StringRules{
				Required: true,
				MinLen:   7,
				Pattern: &webui.PatternRule{
					Expr:    `^\d{1,3}(\.\d{1,3}){3}$`,
					Message: "must be a valid IPv4 address",
				},
			},
		}
		Occurrences = webui.Int[Device]{
			Label: "Occurrences",
			Load:  func(d Device) int { return d.Count },
		}
		Rate = webui.Float[Device]{
			Label: "Rate",
			Load:  func(d Device) float64 { return float64(d.Count) / d.Duration },
		}
	)

	var Details = webui.Page[DetailsArgs]{
		Path: "/device/{id}",
		Guard: func(_ context.Context, a DetailsArgs) error {
			_ = a.Id
			return nil
		},
		Body: webui.Form[Device]{
			Load: func(ctx context.Context) (Device, error) {
				_ = webui.ArgsOf[DetailsArgs](ctx)
				return Device{}, nil
			},
			Submit: webui.Action[Device]{
				Guard: func(_ context.Context, d Device) error {
					_ = d.Id
					return nil
				},
				Run: func(_ context.Context, d Device) (webui.Outcome, error) {
					_ = d
					return webui.Reject(webui.Field[Device](IP, "already in use by another device")), nil
				},
			},
			Fields: []webui.Accessor[Device]{
				webui.Group[Device]{ID, IP},
				Rate,
			},
		},
	}

	var Devices = webui.Page[webui.NoArgs]{
		Path: "/device",
		Nav:  DevicesNav,
		Body: webui.Table[Device]{
			Load: func(_ context.Context, _ webui.Query) (webui.Rows[Device], error) {
				return webui.Rows[Device]{}, nil
			},
			RowClick: webui.Link[Device, DetailsArgs]{
				Page: Details,
				Args: func(_ context.Context, d Device) DetailsArgs {
					return DetailsArgs{Id: d.Id}
				},
			},
			Columns: []webui.Accessor[Device]{
				ID,
				webui.String[Device]{Label: "IP", Load: func(d Device) string { return d.Ip }},
				Occurrences,
				webui.Placeholder[Device]{Accessor: IP, Text: "10.0.0.1"},
			},
		},
	}

	_ = webui.Stack{
		webui.Split{
			webui.Form[Device]{},
			webui.Table[Device]{},
		},
		webui.Tabs{
			Panels: []webui.Tab{{Label: "Raw", Body: webui.Table[Device]{}}},
		},
	}

	app := webui.App{
		Brand: webui.Brand{
			Name: "Demo",
			Logo: image.Image(nil),
		},
		Theme: webui.Theme{},
		Pages: webui.Pages{
			Devices,
			Details,
		},
	}

	_ = app
}
