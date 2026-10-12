package main

import (
	"fmt"
	"strings"
	"time"
)

// The library ships no formats. An accessor's Format is a function the application writes, and it
// changes only how a value is read: a column still sorts, filters and exports on the value. These
// are the demo's own, for a size and for how long ago a moment was.

// humanBytes is a number of bytes in binary units: 512 B, 1.5 KiB, 3 GiB.
func humanBytes(n int) string {
	if n < 1<<10 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	size := float64(n) / (1 << 10)
	i := 0
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", size), ".0") + " " + units[i]
}

// ago is how long before the demo's "now" a moment was. A real application would use time.Now();
// the demo's is fixed, so it and its tests read the same every time.
func ago(t time.Time) string {
	d := epoch.Sub(t)
	switch {
	case d < 0:
		return "in the future"
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d d ago", int(d.Hours()/24))
}
