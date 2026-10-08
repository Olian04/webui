package main

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/Olian04/webui/pkg/webui"
)

// SitesNav is declared on its own so the site page can light it without having
// an entry of its own: Nav.Shadow names it.
var SitesNav = webui.Nav{Label: "Sites"}

// The site is a path argument, so each site has its own address, and the
// breadcrumb reads Collector › Sites › Stockholm: the parent is the page mounted
// at /site.
type SiteArgs struct {
	Name string `webui:"name"`
}

var (
	SiteName = webui.String[SiteSummary]{
		Label: "Site",
		Key:   "name",
		Load:  func(s SiteSummary) string { return s.Name },
	}
	SiteDevicesCount = webui.Int[SiteSummary]{
		Label: "Devices",
		Key:   "devices",
		Load:  func(s SiteSummary) int { return s.Devices },
	}
	SiteDegraded = webui.Int[SiteSummary]{
		Label: "Degraded",
		Key:   "degraded",
		Load:  func(s SiteSummary) int { return s.Degraded },
	}
	SiteHealth = webui.Badge[SiteSummary]{
		Label: "Health",
		Key:   "health",
		Load:  func(s SiteSummary) string { return s.Health() },
		Kinds: map[string]webui.Tone{"ok": webui.ToneOK, "degraded": webui.ToneWarning},
	}
	SiteRate = webui.Slider[SiteSummary]{
		Label: "Mean rate / s", Key: "rate", Max: 15, Precision: 2,
		Load: func(s SiteSummary) float64 { return s.Rate },
	}
)

var Sites = webui.Page[webui.NoArgs]{
	Path: "/site",
	Nav:  SitesNav,
	// A second page offering Search: typing "stock" lists devices and sites,
	// each under its page's name.
	Search: func(ctx context.Context, query string) ([]webui.SearchResult, error) {
		var results []webui.SearchResult
		for _, site := range service.Sites() {
			if strings.Contains(strings.ToLower(site.Name), strings.ToLower(query)) {
				results = append(results, webui.SearchResult{
					Title:  site.Name,
					Desc:   fmt.Sprintf("%d devices", site.Devices),
					Target: webui.Open(ctx, SiteDetail, SiteArgs{Name: site.Name}),
				})
			}
		}
		return results, nil
	},
	Body: webui.Table[SiteSummary]{
		Title: "Sites",
		Desc:  "An unpaged table: Load returns every row, so it also does the sorting and the filtering.",
		Load: func(_ context.Context, q webui.Query) (webui.Rows[SiteSummary], error) {
			sites := filteredBy(service.Sites(), q.Filters, map[string]func(s SiteSummary, values []string) bool{
				"name":   containing(func(s SiteSummary) string { return s.Name }),
				"health": oneOf(func(s SiteSummary) string { return s.Health() }),
			})
			sites = within(sites, boundsOf(q.Ranges), map[string]func(SiteSummary) float64{
				"devices":  func(s SiteSummary) float64 { return float64(s.Devices) },
				"degraded": func(s SiteSummary) float64 { return float64(s.Degraded) },
				"rate":     func(s SiteSummary) float64 { return s.Rate },
			})
			sites = sortedBy(sites, q.Sort, q.Desc, map[string]func(x, y SiteSummary) int{
				"name":     func(x, y SiteSummary) int { return cmp.Compare(x.Name, y.Name) },
				"devices":  func(x, y SiteSummary) int { return cmp.Compare(x.Devices, y.Devices) },
				"degraded": func(x, y SiteSummary) int { return cmp.Compare(x.Degraded, y.Degraded) },
				"health":   func(x, y SiteSummary) int { return cmp.Compare(x.Health(), y.Health()) },
				"rate":     func(x, y SiteSummary) int { return cmp.Compare(x.Rate, y.Rate) },
			})
			return webui.Rows[SiteSummary]{Items: sites, Total: len(sites)}, nil
		},
		RowClick: webui.Link[SiteSummary, SiteArgs]{
			Page: SiteDetail,
			Args: func(_ context.Context, s SiteSummary) SiteArgs { return SiteArgs{Name: s.Name} },
		},
		Columns: []webui.Accessor[SiteSummary]{SiteName, SiteDevicesCount, SiteDegraded, SiteHealth, SiteRate},
	},
}

var SiteDetail = webui.Page[SiteArgs]{
	Path: "/site/{name}",
	Nav:  webui.Nav{Shadow: &SitesNav}, // no entry of its own: it lights Sites
	Guard: func(_ context.Context, a SiteArgs) error {
		if _, ok := service.Site(a.Name); !ok {
			return fmt.Errorf("there is no site %q", a.Name)
		}
		return nil
	},
	// Two tables side by side. Each keeps its own sort, filters and page in the
	// address under its ID, so they need distinct ones: ?devices.sort=… and
	// ?alerts.sort=… do not touch each other.
	Body: webui.Split{SiteDevices, SiteAlerts},
}

var SiteDevices = webui.Table[Device]{
	ID:       "devices",
	Title:    "Devices",
	PageSize: 5,
	Load: func(ctx context.Context, q webui.Query) (webui.Rows[Device], error) {
		args, err := webui.ArgsOf[SiteArgs](ctx)
		if err != nil {
			return webui.Rows[Device]{}, err
		}
		// The page says which site; the user's filters narrow within it.
		filters := map[string][]string{"site": {args.Name}}
		for key, values := range q.Filters {
			filters[key] = values
		}
		devices, total := service.Devices(filters, boundsOf(q.Ranges), Order{Offset: q.Offset, Limit: q.Limit, Sort: q.Sort, Desc: q.Desc})
		return webui.Rows[Device]{Items: devices, Total: total}, nil
	},
	RowClick: webui.Link[Device, DeviceArgs]{
		Page: Details,
		Args: func(_ context.Context, d Device) DeviceArgs { return DeviceArgs{ID: d.ID} },
	},
	Columns: []webui.Accessor[Device]{DeviceID, IP, Status, Occurrences, Rate},
}

var SiteAlerts = webui.Table[Alert]{
	ID:    "alerts",
	Title: "Open alerts",
	Load: func(ctx context.Context, q webui.Query) (webui.Rows[Alert], error) {
		args, err := webui.ArgsOf[SiteArgs](ctx)
		if err != nil {
			return webui.Rows[Alert]{}, err
		}
		alerts := alertRows(service.AlertsAt(args.Name), q)
		return webui.Rows[Alert]{Items: alerts, Total: len(alerts)}, nil
	},
	RowClick: webui.Link[Alert, DeviceArgs]{
		Page: Details,
		Args: func(_ context.Context, a Alert) DeviceArgs { return DeviceArgs{ID: a.Device, Minutes: 15} },
	},
	Columns: []webui.Accessor[Alert]{AlertID, Severity, AlertDevice},
}
