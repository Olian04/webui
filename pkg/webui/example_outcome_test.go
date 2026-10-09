package webui_test

import (
	"context"

	"github.com/Olian04/webui/pkg/webui"
)

// An action says how it ended, and the library decides the toast, its colour, and
// the response. A Reject keeps what the user typed and points at the fields, a
// Failure speaks for the whole form, and a Success or Warning is accepted: the
// user is sent back, or on, with Then.
func ExampleOutcome() {
	type device struct{ IP string }
	ip := webui.String[device]{Label: "IP", Load: func(d device) string { return d.IP }}

	save := webui.Action[device]{
		Run: func(ctx context.Context, d device) (webui.Outcome, error) {
			switch d.IP {
			case "10.0.0.1":
				// Every rule passed, and the service still refuses: the form is shown
				// again with what was typed, and this message beside the field.
				return webui.Reject(webui.Field[device](ip, "already in use")), nil
			case "":
				// Not about one field: the form is shown again, with this in an error toast.
				return webui.Failure("Could not reach the device"), nil
			case "0.0.0.0":
				// Accepted, but the user should know something.
				return webui.Warning("Saved, but 3 devices still use the old address"), nil
			}
			// Accepted. Then takes the user on, to a page built with Open.
			return webui.Success("Device saved").Then(webui.Open(ctx, devicesPath, webui.NoArgs{})), nil
		},
	}
	_ = save
}

const devicesPath webui.PageID[webui.NoArgs] = "/device"
