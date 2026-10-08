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

// DeviceFilter and Order are what the list asks of Devices.
type DeviceFilter struct{ Site, Status, Q string }

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

// Devices returns one page of the devices matching f, and how many match.
func (s *Service) Devices(f DeviceFilter, o Order) ([]Device, int) {
	s.mu.Lock()
	all := slices.Clone(s.devices)
	s.mu.Unlock()

	needle := strings.ToLower(f.Q)
	all = slices.DeleteFunc(all, func(d Device) bool {
		return (f.Site != "" && d.Site != f.Site) || (f.Status != "" && d.Status != f.Status) ||
			(needle != "" && !strings.Contains(strings.ToLower(d.ID+" "+d.IP+" "+d.Site), needle))
	})

	if by, ok := deviceKeys[o.Sort]; ok {
		slices.SortStableFunc(all, func(x, y Device) int {
			c := by(x, y)
			if o.Desc {
				return -c
			}
			return c
		})
	}
	lo := min(o.Offset, len(all))
	hi := len(all)
	if o.Limit > 0 {
		hi = min(lo+o.Limit, len(all))
	}
	return all[lo:hi], len(all)
}

// deviceKeys are the orders the list offers, by the key its columns declare.
var deviceKeys = map[string]func(x, y Device) int{
	"id":     func(x, y Device) int { return cmp.Compare(x.ID, y.ID) },
	"ip":     func(x, y Device) int { return cmp.Compare(x.IP, y.IP) },
	"status": func(x, y Device) int { return cmp.Compare(x.Status, y.Status) },
	"count":  func(x, y Device) int { return cmp.Compare(x.Count, y.Count) },
	"rate":   func(x, y Device) int { return cmp.Compare(x.Rate(), y.Rate()) },
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

// OpenAlerts are the alerts nobody has acknowledged, of one severity or all.
func (s *Service) OpenAlerts(severity string) []Alert {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.DeleteFunc(slices.Clone(s.alerts), func(a Alert) bool {
		return a.Acked || (severity != "" && a.Severity != severity)
	})
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
