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
}

// Rate is occurrences per second.
func (d Device) Rate() float64 { return float64(d.Count) / d.Duration }

type Alert struct {
	ID, Severity, Device, Message string
	Acked                         bool
}

type Event struct{ At, Kind, Detail string }

type Settings struct {
	Name string
	Port int
}

type RetentionPolicy struct{ Days int }

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
}

var service = newService()

func newService() *Service {
	s := &Service{settings: Settings{Name: "eu-north-1", Port: 8125}, retention: RetentionPolicy{Days: 30}}
	sites := []string{"Stockholm", "Malmö", "Göteborg"}
	statuses := []string{"healthy", "healthy", "degraded", "quiet"}
	for i := range 37 {
		s.devices = append(s.devices, Device{
			ID: fmt.Sprintf("dev_%06x", 0x27c38b+i*977), IP: fmt.Sprintf("10.0.%d.%d", i/8, 10+i),
			Status: statuses[i%len(statuses)], Site: sites[i%len(sites)],
			Count: 40 + (i*53)%900, Duration: 60 + float64(i%7)*30,
		})
	}
	severities := []string{"critical", "warning", "info"}
	for i := range 11 {
		s.alerts = append(s.alerts, Alert{
			ID: fmt.Sprintf("alt_%03d", i), Severity: severities[i%3], Device: s.devices[i*3].ID,
			Message: fmt.Sprintf("Ingest lag above %ds", 5+i),
		})
	}
	return s
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
	defer s.mu.Unlock()
	for i := range s.devices {
		if s.devices[i].ID == id {
			s.devices[i].IP = ip
		}
	}
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

// deviceKeys are the orders the list offers, by the Key its columns declare.
var deviceKeys = map[string]func(x, y Device) int{
	"id":     func(x, y Device) int { return cmp.Compare(x.ID, y.ID) },
	"ip":     func(x, y Device) int { return cmp.Compare(x.IP, y.IP) },
	"status": func(x, y Device) int { return cmp.Compare(x.Status, y.Status) },
	"site":   func(x, y Device) int { return cmp.Compare(x.Site, y.Site) },
	"count":  func(x, y Device) int { return cmp.Compare(x.Count, y.Count) },
	"rate":   func(x, y Device) int { return cmp.Compare(x.Rate(), y.Rate()) },
}

// deviceFilters are the filters the list offers, by the same Key as its sorts.
// Status has a fixed set of options and arrives as the options chosen; the text
// columns arrive as the text typed. The numeric columns are not here: they arrive
// as bounds, in deviceNumbers.
var deviceFilters = map[string]func(d Device, values []string) bool{
	"id":     containing(func(d Device) string { return d.ID }),
	"ip":     containing(func(d Device) string { return d.IP }),
	"site":   containing(func(d Device) string { return d.Site }),
	"status": oneOf(func(d Device) string { return d.Status }),
}

// deviceNumbers are the numeric columns, which a table filters by range.
var deviceNumbers = map[string]func(d Device) float64{
	"count": func(d Device) float64 { return float64(d.Count) },
	"rate":  func(d Device) float64 { return d.Rate() },
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

// Events are made up on the spot: the last few minutes of a device.
func (s *Service) Events() []Event {
	now := time.Now()
	var events []Event
	for i, kind := range []string{"flush", "connect", "flush", "timeout", "flush", "connect"} {
		detail := "ok"
		if kind == "timeout" {
			detail = "no data for 30s"
		}
		events = append(events, Event{At: now.Add(-time.Duration(i) * 7 * time.Minute).Format("15:04"), Kind: kind, Detail: detail})
	}
	return events
}

// OpenAlerts are the alerts nobody has acknowledged.
func (s *Service) OpenAlerts() []Alert {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.DeleteFunc(slices.Clone(s.alerts), func(a Alert) bool { return a.Acked })
}

func (s *Service) Acknowledge(ids ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.alerts {
		if slices.Contains(ids, s.alerts[i].ID) {
			s.alerts[i].Acked = true
		}
	}
}

func (s *Service) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

func (s *Service) SetSettings(v Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = v
}

func (s *Service) Retention() RetentionPolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.retention
}

func (s *Service) SetRetention(v RetentionPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retention = v
}
