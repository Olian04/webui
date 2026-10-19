package main

import (
	"context"
	"slices"

	"github.com/Olian04/webui/pkg/webui"
)

// CompareArgs are the arguments of the compare page, both of them lists: a slice is a query parameter that
// repeats, the parameter once for each element, so "?device=a&device=b&site=Malmö" is two devices and a site.
type CompareArgs struct {
	Device []string
	Site   []string
}

const compareHelp = `## Arguments that repeat

This page takes two lists in its address, and shows the devices they name: each **device** by its id, and every device at each **site**.
A field that is a slice (here a ` + "`[]string`" + `) is a query parameter that repeats, the parameter once for each element, in the order given,
and a page that links here builds its address the same way.

Try:

- [Two devices](/compare?device=dev_27c38b&device=dev_27c75c)
- [A site](/compare?site=Malm%C3%B6)
- [Two sites and a device](/compare?site=Malm%C3%B6&site=G%C3%B6teborg&device=dev_27c38b)
- [Nothing](/compare): an empty list is no parameter at all

A table on the page that is sorted or paged keeps every element of both lists, since they are part of its address.
`

var CompareDevices = webui.Table[Device]{
	Title:    "Devices",
	Desc:     "The devices that the address names.",
	PageSize: 5,
	Rows: func(ctx context.Context) ([]Device, error) {
		args := webui.ArgsOf[CompareArgs](ctx)
		var out []Device
		for _, id := range args.Device {
			if d, ok := service.Device(id); ok && !slices.ContainsFunc(out, func(o Device) bool { return o.ID == d.ID }) {
				out = append(out, d)
			}
		}
		if len(args.Site) > 0 {
			bySite, _ := service.Devices(map[string][]string{Site.Label: args.Site}, nil, Order{})
			for _, d := range bySite {
				if !slices.ContainsFunc(out, func(o Device) bool { return o.ID == d.ID }) {
					out = append(out, d)
				}
			}
		}
		return out, nil
	},
	RowClick: webui.Link[Device, DeviceArgs]{
		Page: Details,
		Args: func(_ context.Context, d Device) DeviceArgs { return DeviceArgs{ID: d.ID} },
	},
	Columns: []webui.Accessor[Device]{DeviceID, IP, Status, Site, Rate},
}

// A page whose arguments are lists. Its address is the whole of what it shows.
var Compare = webui.Page[CompareArgs]{
	Path: "/compare",
	Nav:  webui.Nav{Label: "Compare devices", Icon: "code-compare", Section: "Platform"},
	Body: webui.Stack{
		webui.Markdown[Handbook]{
			Title:   "Compare devices",
			Desc:    "Query parameters that repeat.",
			Load:    func(context.Context) (Handbook, error) { return Handbook{Text: compareHelp}, nil },
			Content: HandbookText,
		},
		CompareDevices,
	},
}
