package main

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/Olian04/webui/pkg/webui"
)

// The Probes page is the collector checking itself: each panel is a group of health probes, and each group answers in the time its backend takes, so that what a visitor sees while a page's
// backends answer is the page's own. A page with a panel that is slower than 50 ms is sent as it is built, in the order it has its panels, with the spinner and the status bar saying that more
// is coming; a page whose panels answer sooner is sent in one piece.

// Probe is a row of a panel of probes: which one it is, and how long its backend took to answer.
type Probe struct{ Panel, Backend string }

var (
	ProbePanel   = webui.String[Probe]{Label: "Probe", Load: func(p Probe) string { return p.Panel }}
	ProbeBackend = webui.String[Probe]{Label: "Answered after", Load: func(p Probe) string { return p.Backend }}
)

// wait is a backend that takes d to answer, and stops when the visitor goes.
func wait(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

// probes is a table of probes whose backend takes d, with as many rows as it is given, so that the panels differ in height.
func probes(title string, d time.Duration, rows int) webui.Table[Probe] {
	return webui.Table[Probe]{
		Title:   title,
		Columns: []webui.Accessor[Probe]{ProbePanel, ProbeBackend},
		Loader: webui.RowsLoader[Probe]{Load: func(ctx context.Context) ([]Probe, webui.Loaded) {
			wait(ctx, d)
			out := make([]Probe, rows)
			for i := range out {
				out[i] = Probe{Panel: title + " " + strconv.Itoa(i+1), Backend: d.String()}
			}
			return out, webui.Ready()
		}},
	}
}

// Stamp is a row that says when it was read, so that a panel that was read again can be told from one that was not.
type Stamp struct{ Panel, ReadAt string }

var (
	StampPanel  = webui.String[Stamp]{Label: "Probe", Load: func(s Stamp) string { return s.Panel }}
	StampReadAt = webui.String[Stamp]{Label: "Read at", Load: func(s Stamp) string { return s.ReadAt }}
)

// stamped is a table whose backend takes d and says when it answered, to the millisecond.
func stamped(title string, d time.Duration) webui.Table[Stamp] {
	return webui.Table[Stamp]{
		Title:   title,
		Columns: []webui.Accessor[Stamp]{StampPanel, StampReadAt},
		Loader: webui.RowsLoader[Stamp]{Load: func(ctx context.Context) ([]Stamp, webui.Loaded) {
			wait(ctx, d)
			return []Stamp{{Panel: title, ReadAt: time.Now().Format("15:04:05.000")}}, webui.Ready()
		}},
	}
}

// Probes is the collector's health probes, in groups that answer in different times. The panels are drawn in the order the page has them, whichever answered first: the storage probes (400 ms)
// are slower than the network probes (250 ms) under them, which wait for them. A Split is sent like the page, in order, so the clock probes on its right wait for the queue probes on its left.
// The Tabs read only the tab that is followed: with script, a tab is followed by asking for the Tabs alone, so the sweep panels above and below keep the moment they were read at, and only the
// tab that was chosen is read. A panel whose Load returns an error is a panel that says it could not load, wherever it is, and so is one whose Load panics: once the page has begun to be sent
// it cannot be a 500, so the cause is logged and not shown, and the panels after it are drawn.
var Probes = webui.Page[webui.NoArgs]{
	Path: "/probes",
	View: webui.Stack{
		webui.Split{
			probes("Queue probes (300 ms)", 300*time.Millisecond, 3),
			probes("Clock probes (20 ms)", 20*time.Millisecond, 3),
		},
		probes("Storage probes (400 ms)", 400*time.Millisecond, 4),
		probes("Network probes (250 ms)", 250*time.Millisecond, 3),
		stamped("Last full sweep (read with the page)", 0),
		webui.Tabs{
			{Label: "Ingest", View: stamped("Ingest probes (500 ms)", 500*time.Millisecond)},
			{Label: "Storage", View: stamped("Storage sweep (500 ms)", 500*time.Millisecond)},
			{Label: "Network", View: stamped("Network sweep (500 ms)", 500*time.Millisecond)},
		},
		stamped("Sweep summary (read with the page)", 0),
		webui.Table[Probe]{
			Title:   "Archive probes (unreachable)",
			Columns: []webui.Accessor[Probe]{ProbePanel, ProbeBackend},
			Loader: webui.RowsLoader[Probe]{Load: func(ctx context.Context) ([]Probe, webui.Loaded) {
				wait(ctx, 50*time.Millisecond)
				return nil, webui.Failed(errors.New("the archive is unavailable"))
			}},
		},
		webui.Form[Probe]{
			Title: "Webhook probes (crashed)",
			Load: func(ctx context.Context) (Probe, webui.Loaded) {
				wait(ctx, 100*time.Millisecond)
				panic("this panel's backend fell over")
			},
			Fields: []webui.Accessor[Probe]{ProbePanel},
		},
		probes("Heartbeat probes (10 ms)", 10*time.Millisecond, 2),
	},
}
