package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// The data the demo serves. It stands in for whatever a real app would call: a
// database, another service. Nothing here knows about webui.

type Device struct {
	ID, IP, Status, Site string
	Count                int
	Duration             float64
	LastSeen             int // Unix seconds
}

// epoch is "now" for the seed data, fixed so the demo and its tests read the same
// every time: the moments the data was made at count back from here.
var epoch = time.Date(2026, 10, 9, 11, 30, 0, 0, time.UTC)

// Rate is occurrences per second.
func (d Device) Rate() float64 { return float64(d.Count) / d.Duration }

type Alert struct {
	ID, Severity, Device, Message string
	Raised                        string // an ISO 8601 moment
	Acked                         bool
}

// Event is something a device did; Age is how many minutes ago.
type Event struct {
	At, Kind, Detail string
	Age              int
}

type Settings struct {
	Name       string
	Port       int
	SampleRate float64 // 0 to 1: the share of events kept
	MaxLoad    float64 // percent: where ingest starts shedding
	Maintain   int     // Unix seconds: when the next maintenance window starts
}

type RetentionPolicy struct{ Days int }

// SiteSummary is one site rolled up from its devices.
type SiteSummary struct {
	Name     string
	Devices  int
	Degraded int
	Rate     float64 // mean occurrences per second
}

// Health is "ok" while no device at the site is degraded.
func (s SiteSummary) Health() string {
	if s.Degraded > 0 {
		return "degraded"
	}
	return "ok"
}

// SystemInfo is what the collector reports about itself. Nobody edits it.
type SystemInfo struct {
	Version  string
	Health   string
	Uptime   string
	DiskUsed float64 // percent
	Queue    int
}

// AuditEntry is something somebody did.
type AuditEntry struct{ At, Actor, Action, Target string }

// Order is what the list asks of Devices besides its filters.
type Order struct {
	Offset, Limit int
	Sort          string
	Desc          bool
}

type Service struct {
	mu        sync.Mutex
	devices   []Device
	alerts    []Alert
	settings  Settings
	retention RetentionPolicy
	audit     []AuditEntry // newest first
	started   time.Time
}

var service = newService()

func newService() *Service {
	s := &Service{
		settings:  Settings{Name: "eu-north-1", Port: 8125, SampleRate: 0.25, MaxLoad: 80, Maintain: int(epoch.Add(36 * time.Hour).Unix())},
		retention: RetentionPolicy{Days: 30},
		started:   time.Now().Add(-9*24*time.Hour - 4*time.Hour),
	}
	sites := []string{"Stockholm", "Malmö", "Göteborg"}
	statuses := []string{"healthy", "healthy", "degraded", "quiet"}
	for i := range 37 {
		d := Device{
			ID: fmt.Sprintf("dev_%06x", 0x27c38b+i*977), IP: fmt.Sprintf("10.0.%d.%d", i/8, 10+i),
			Status: statuses[i%len(statuses)], Site: sites[i%len(sites)],
			Count: 40 + (i*53)%900, Duration: 60 + float64(i%7)*30,
			LastSeen: int(epoch.Add(-time.Duration(i*17) * time.Minute).Unix()),
		}
		if d.Site == "Göteborg" && d.Status == "degraded" {
			d.Status = "healthy" // one site is fine, so the sites page has both
		}
		s.devices = append(s.devices, d)
	}
	severities := []string{"critical", "warning", "info"}
	for i := range 11 {
		s.alerts = append(s.alerts, Alert{
			ID: fmt.Sprintf("alt_%03d", i), Severity: severities[i%3], Device: s.devices[i*3+i%3].ID, // spread over the three sites; alt_000 is dev_27c38b
			Message: fmt.Sprintf("Ingest lag above %ds", 5+i),
			Raised:  epoch.Add(-time.Duration(i*53) * time.Minute).Format(time.RFC3339),
		})
	}
	// A history to scroll: 43 entries, one every 37 minutes.
	actors := []string{"ines", "oskar", "sam", "ines", "you"}
	actions := []string{"saved settings", "acknowledged alert", "changed IP", "dismissed alert", "saved retention"}
	for i := range 43 {
		s.audit = append(s.audit, AuditEntry{
			At:    time.Now().Add(-time.Duration(i+1) * 37 * time.Minute).Format("Jan 2 15:04"),
			Actor: actors[i%len(actors)], Action: actions[(i*3)%len(actions)],
			Target: s.devices[(i*5)%len(s.devices)].ID,
		})
	}
	return s
}

// record adds to the audit trail, newest first. The caller holds no lock.
func (s *Service) record(action, target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audit = slices.Insert(s.audit, 0, AuditEntry{At: time.Now().Format("Jan 2 15:04"), Actor: "you", Action: action, Target: target})
}

func (s *Service) Device(id string) (Device, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.devices {
		if d.ID == id {
			return d, true
		}
	}
	return Device{}, false
}

func (s *Service) IPTaken(ip, except string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.devices, func(d Device) bool { return d.IP == ip && d.ID != except })
}

func (s *Service) SetIP(id, ip string) {
	s.mu.Lock()
	for i := range s.devices {
		if s.devices[i].ID == id {
			s.devices[i].IP = ip
		}
	}
	s.mu.Unlock()
	s.record("changed IP", id)
}

// Bounds limit a number, both ends inclusive; nil is unbounded.
type Bounds struct{ Min, Max *float64 }

// Devices returns one page of the devices passing the filters and the bounds,
// and how many pass.
func (s *Service) Devices(filters map[string][]string, bounds map[string]Bounds, o Order) ([]Device, int) {
	s.mu.Lock()
	all := slices.Clone(s.devices)
	s.mu.Unlock()

	all = filteredBy(all, filters, deviceFilters)
	all = within(all, bounds, deviceNumbers)
	all = sortedBy(all, o.Sort, o.Desc, deviceKeys)
	lo := min(o.Offset, len(all))
	hi := len(all)
	if o.Limit > 0 {
		hi = min(lo+o.Limit, len(all))
	}
	return all[lo:hi], len(all)
}

// Find is the global search: devices whose id, address or site contains query.
func (s *Service) Find(query string, limit int) []Device {
	needle := strings.ToLower(query)
	s.mu.Lock()
	defer s.mu.Unlock()
	var found []Device
	for _, d := range s.devices {
		if strings.Contains(strings.ToLower(d.ID+" "+d.IP+" "+d.Site), needle) {
			found = append(found, d)
			if len(found) == limit {
				break
			}
		}
	}
	return found
}

// deviceKeys are the orders the list offers, by the Label of its columns.
var deviceKeys = map[string]func(x, y Device) int{
	"ID":          func(x, y Device) int { return cmp.Compare(x.ID, y.ID) },
	"IP":          func(x, y Device) int { return cmp.Compare(x.IP, y.IP) },
	"Status":      func(x, y Device) int { return cmp.Compare(x.Status, y.Status) },
	"Site":        func(x, y Device) int { return cmp.Compare(x.Site, y.Site) },
	"Occurrences": func(x, y Device) int { return cmp.Compare(x.Count, y.Count) },
	"Rate / s":    func(x, y Device) int { return cmp.Compare(x.Rate(), y.Rate()) },
}

// deviceFilters are the filters the list offers, by the same Label as its sorts.
// Status has a fixed set of options and arrives as the options chosen; the text
// columns arrive as the text typed. The numeric columns are not here: they arrive
// as bounds, in deviceNumbers.
var deviceFilters = map[string]func(d Device, values []string) bool{
	"ID":     containing(func(d Device) string { return d.ID }),
	"IP":     containing(func(d Device) string { return d.IP }),
	"Site":   containing(func(d Device) string { return d.Site }),
	"Status": oneOf(func(d Device) string { return d.Status }),
}

// deviceNumbers are the numeric columns, which a table filters by range.
var deviceNumbers = map[string]func(d Device) float64{
	"Occurrences": func(d Device) float64 { return float64(d.Count) },
	"Rate / s":    func(d Device) float64 { return d.Rate() },
}

// within keeps the rows whose number lies inside every bound in force, which is
// what a table's Load receives in Query.Ranges.
func within[T any](rows []T, bounds map[string]Bounds, value map[string]func(T) float64) []T {
	if len(bounds) == 0 {
		return rows
	}
	return slices.DeleteFunc(slices.Clone(rows), func(row T) bool {
		for key, b := range bounds {
			get, ok := value[key]
			if !ok {
				continue
			}
			if x := get(row); (b.Min != nil && x < *b.Min) || (b.Max != nil && x > *b.Max) {
				return true
			}
		}
		return false
	})
}

// containing matches rows whose text contains what was typed, ignoring case.
func containing[T any](text func(T) string) func(T, []string) bool {
	return func(row T, values []string) bool {
		return strings.Contains(strings.ToLower(text(row)), strings.ToLower(values[0]))
	}
}

// oneOf matches rows whose value is any of the options chosen.
func oneOf[T any](value func(T) string) func(T, []string) bool {
	return func(row T, values []string) bool { return slices.Contains(values, value(row)) }
}

// filteredBy keeps the rows that pass every filter in force, which is what a
// table's Load receives in Query.Filters. A filter nobody registered a match for
// keeps everything, like a sort key nobody registered.
func filteredBy[T any](rows []T, filters map[string][]string, by map[string]func(row T, values []string) bool) []T {
	if len(filters) == 0 {
		return rows
	}
	return slices.DeleteFunc(slices.Clone(rows), func(row T) bool {
		for key, values := range filters {
			if match, ok := by[key]; ok && !match(row, values) {
				return true
			}
		}
		return false
	})
}

// sortedBy orders rows by the comparison registered for key, which is what a
// table's Load receives in Query.Sort. A key nobody registered leaves the order
// alone. A table whose Load ignored Query.Sort would show sort links that do
// nothing, so every table in the demo does this.
func sortedBy[T any](rows []T, key string, desc bool, by map[string]func(x, y T) int) []T {
	compare, ok := by[key]
	if !ok {
		return rows
	}
	rows = slices.Clone(rows)
	slices.SortStableFunc(rows, func(x, y T) int {
		if desc {
			return compare(y, x)
		}
		return compare(x, y)
	})
	return rows
}

// Events are made up on the spot: a device's last hour or so. A window, in
// minutes, keeps only the recent ones; zero keeps them all.
func (s *Service) Events(window int) []Event {
	now := time.Now()
	var events []Event
	for i, kind := range []string{"flush", "connect", "flush", "timeout", "flush", "connect", "flush", "flush"} {
		detail := "ok"
		if kind == "timeout" {
			detail = "no data for 30s"
		}
		age := i * 7
		if window > 0 && age > window {
			continue
		}
		events = append(events, Event{At: now.Add(-time.Duration(age) * time.Minute).Format("15:04"), Kind: kind, Detail: detail, Age: age})
	}
	return events
}

// Alert is one alert, acknowledged or not.
func (s *Service) Alert(id string) (Alert, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.alerts {
		if a.ID == id {
			return a, true
		}
	}
	return Alert{}, false
}

// OpenAlerts are the alerts nobody has acknowledged.
func (s *Service) OpenAlerts() []Alert {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.DeleteFunc(slices.Clone(s.alerts), func(a Alert) bool { return a.Acked })
}

func (s *Service) Acknowledge(ids ...string) {
	s.mu.Lock()
	for i := range s.alerts {
		if slices.Contains(ids, s.alerts[i].ID) {
			s.alerts[i].Acked = true
		}
	}
	s.mu.Unlock()
	s.record("acknowledged alert", strings.Join(ids, ", "))
}

// DeleteAlerts removes alerts for good.
func (s *Service) DeleteAlerts(ids ...string) {
	s.mu.Lock()
	s.alerts = slices.DeleteFunc(s.alerts, func(a Alert) bool { return slices.Contains(ids, a.ID) })
	s.mu.Unlock()
	s.record("deleted alert", strings.Join(ids, ", "))
}

// AlertsAt are the open alerts of the devices at one site.
func (s *Service) AlertsAt(site string) []Alert {
	s.mu.Lock()
	defer s.mu.Unlock()
	at := map[string]bool{}
	for _, d := range s.devices {
		if d.Site == site {
			at[d.ID] = true
		}
	}
	return slices.DeleteFunc(slices.Clone(s.alerts), func(a Alert) bool { return a.Acked || !at[a.Device] })
}

// Sites rolls the devices up by site.
func (s *Service) Sites() []SiteSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	by := map[string]*SiteSummary{}
	var order []string
	for _, d := range s.devices {
		site, ok := by[d.Site]
		if !ok {
			site = &SiteSummary{Name: d.Site}
			by[d.Site], order = site, append(order, d.Site)
		}
		site.Devices++
		site.Rate += d.Rate()
		if d.Status == "degraded" {
			site.Degraded++
		}
	}
	out := make([]SiteSummary, 0, len(order))
	for _, name := range order {
		site := *by[name]
		site.Rate /= float64(site.Devices)
		out = append(out, site)
	}
	return out
}

func (s *Service) Site(name string) (SiteSummary, bool) {
	for _, site := range s.Sites() {
		if site.Name == name {
			return site, true
		}
	}
	return SiteSummary{}, false
}

// System is made up too, but it changes: the uptime is real.
func (s *Service) System() SystemInfo {
	up := time.Since(s.started).Round(time.Minute)
	return SystemInfo{
		Version: "2.4.1", Health: "healthy", DiskUsed: 71,
		Uptime: fmt.Sprintf("%dd %dh", int(up.Hours())/24, int(up.Hours())%24), Queue: 128,
	}
}

// Audit returns one window of the audit trail, newest first. It does not count
// the entries it skips, so the table that shows it does not know the total.
func (s *Service) Audit(offset, limit int) []AuditEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	lo := min(offset, len(s.audit))
	hi := len(s.audit)
	if limit > 0 {
		hi = min(lo+limit, len(s.audit))
	}
	return slices.Clone(s.audit[lo:hi])
}

func (s *Service) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

func (s *Service) SetSettings(v Settings) {
	s.mu.Lock()
	s.settings = v
	s.mu.Unlock()
	s.record("saved settings", v.Name)
}

func (s *Service) Retention() RetentionPolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.retention
}

func (s *Service) SetRetention(v RetentionPolicy) {
	s.mu.Lock()
	s.retention = v
	s.mu.Unlock()
	s.record("saved retention", fmt.Sprintf("%d days", v.Days))
}
