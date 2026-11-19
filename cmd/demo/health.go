package main

import (
	"context"
	"errors"
	"time"

	"github.com/Olian04/webui/pkg/webui"
)

// Check is one health check of the collector: what was checked, how it came out, how long it took, and when it was read.
type Check struct{ Name, Status, Took, Detail, At string }

var (
	CheckName   = webui.String[Check]{Label: "Check", Load: func(c Check) string { return c.Name }}
	CheckStatus = webui.Select[Check]{
		Label: "Status", Load: func(c Check) []string { return []string{c.Status} },
		Options: []webui.Option{{Value: "ok", Tone: webui.ToneOK}, {Value: "warning", Tone: webui.ToneWarning}, {Value: "critical", Tone: webui.ToneCritical}},
	}
	CheckTook   = webui.String[Check]{Label: "Took", Load: func(c Check) string { return c.Took }}
	CheckDetail = webui.String[Check]{Label: "Detail", Load: func(c Check) string { return c.Detail }}
	CheckAt     = webui.String[Check]{Label: "Read at", Load: func(c Check) string { return c.At }}
)

// checksOf are the checks of each part of the collector. A real collector would run them; here they are what it found.
var checksOf = map[string][]Check{
	"ingest": {
		{Name: "Accepts events", Status: "ok", Took: "12 ms", Detail: "37 devices are reporting"},
		{Name: "Queue depth", Status: "warning", Took: "8 ms", Detail: "128 events are waiting; the limit is 100"},
		{Name: "Writer lag", Status: "ok", Took: "4 ms", Detail: "1.2 s behind the newest event"},
	},
	"clock": {
		{Name: "Clock offset", Status: "ok", Took: "3 ms", Detail: "+4 ms against pool.ntp.org"},
		{Name: "Time zone", Status: "ok", Took: "1 ms", Detail: "UTC"},
	},
	"storage": {
		{Name: "Disk space", Status: "ok", Took: "41 ms", Detail: "71% used"},
		{Name: "Write latency", Status: "ok", Took: "58 ms", Detail: "95th percentile 12 ms"},
		{Name: "Retention job", Status: "warning", Took: "23 ms", Detail: "last ran 26 hours ago"},
		{Name: "Compaction", Status: "ok", Took: "19 ms", Detail: "finished at 02:10"},
	},
	"network": {
		{Name: "DNS resolution", Status: "ok", Took: "6 ms", Detail: "3 resolvers answer"},
		{Name: "Upstream sinks", Status: "ok", Took: "88 ms", Detail: "2 of 2 reachable"},
		{Name: "TLS certificates", Status: "critical", Took: "14 ms", Detail: "hooks.example.com expires in 6 days"},
	},
	"summary": {
		{Name: "Ingest", Status: "warning", Took: "24 ms", Detail: "1 check needs a look"},
		{Name: "Storage", Status: "warning", Took: "141 ms", Detail: "1 check needs a look"},
		{Name: "Network", Status: "critical", Took: "108 ms", Detail: "1 check is failing"},
	},
	"findings": {
		{Name: "Queue depth", Status: "warning", Took: "8 ms", Detail: "Ingest: 128 events are waiting"},
		{Name: "Retention job", Status: "warning", Took: "23 ms", Detail: "Storage: last ran 26 hours ago"},
		{Name: "TLS certificates", Status: "critical", Took: "14 ms", Detail: "Network: hooks.example.com expires in 6 days"},
	},
	"heartbeat": {
		{Name: "Collector heartbeat", Status: "ok", Took: "2 ms", Detail: "answered"},
	},
}

// wait is a backend that takes d to answer, and stops when the visitor goes.
func wait(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

// checks is a panel of the checks of one part of the collector, whose backend takes d to answer, as a real one takes the time that reading a disk or a network takes. Each row says when
// it was read, so that a panel that was read again can be told from one that was not.
func checks(title, group, desc string, d time.Duration) webui.Table[Check] {
	return webui.Table[Check]{
		Title:   title,
		Desc:    desc,
		Columns: []webui.Accessor[Check]{CheckName, CheckStatus, CheckTook, CheckDetail, CheckAt},
		Loader: webui.RowsLoader[Check]{Load: func(ctx context.Context) ([]Check, webui.Loaded) {
			wait(ctx, d)
			now := time.Now().Format("15:04:05.000")
			rows := append([]Check(nil), checksOf[group]...)
			for i := range rows {
				rows[i].At = now
			}
			return rows, webui.Ready()
		}},
	}
}

// HealthChecks is the collector checking itself: each panel is the checks of one part of it, and each answers in the time its backend takes, so that what a visitor sees while a page's backends
// answer is the page's own. The page is sent as it is built, in the order it is written, whichever panel answered first: the storage checks (400 ms) are slower than the network checks
// (250 ms) under them, which wait for them, and a Split is sent in order too, so the clock checks beside the ingest checks wait for them. The Tabs read only the tab that is followed: with
// script a tab is followed by asking for the Tabs alone, so the panels above and below keep the moment they were read at, and only the history of the tab that was chosen is read. A
// panel whose Load returns an error says it could not load, wherever it is, and so does one whose Load panics: once the page has begun to be sent it cannot be a 500, so the cause is logged
// and not shown, and the panels after it are drawn.
var HealthChecks = webui.Page[webui.NoArgs]{
	Path: "/health",
	View: webui.Stack{
		webui.Split{
			checks("Ingest", "ingest", "Whether events are arriving, and being written as fast as they come.", 300*time.Millisecond),
			checks("Clock", "clock", "Whether the collector's own clock can be trusted to stamp events.", 20*time.Millisecond),
		},
		checks("Storage", "storage", "Reading the disk and the jobs that keep it small is the slowest of the checks.", 400*time.Millisecond),
		checks("Network", "network", "Resolvers, the sinks that events are forwarded to, and the certificates they are reached with.", 250*time.Millisecond),
		checks("Last sweep", "summary", "One line for each part of the collector, read with the page.", 0),
		webui.Tabs{
			{Label: "Ingest", View: checks("Ingest history", "ingest", "The ingest checks again, read when this tab is followed.", 500*time.Millisecond)},
			{Label: "Storage", View: checks("Storage history", "storage", "The storage checks again, read when this tab is followed.", 500*time.Millisecond)},
			{Label: "Network", View: checks("Network history", "network", "The network checks again, read when this tab is followed.", 500*time.Millisecond)},
		},
		checks("Open findings", "findings", "What needs a look across all of them, read with the page.", 0),
		webui.Table[Check]{
			Title:   "Archive",
			Columns: []webui.Accessor[Check]{CheckName, CheckStatus, CheckTook, CheckDetail},
			Loader: webui.RowsLoader[Check]{Load: func(ctx context.Context) ([]Check, webui.Loaded) {
				wait(ctx, 50*time.Millisecond)
				return nil, webui.Failed(errors.New("the archive is unavailable"))
			}},
		},
		webui.Form[Check]{
			Title: "Webhook delivery",
			Load: func(ctx context.Context) (Check, webui.Loaded) {
				wait(ctx, 100*time.Millisecond)
				panic("this panel's backend fell over")
			},
			Fields: []webui.Accessor[Check]{CheckName},
		},
		checks("Heartbeat", "heartbeat", "The quickest check there is, and the last panel of the page.", 10*time.Millisecond),
	},
}
