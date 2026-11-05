//go:build ignore

// Command fontawesome_gen writes the file the renderer derives from a
// Font Awesome Free download: icons.txt, the names a Nav may use, each with the
// code point that the icon is at in the font (aliases have their icon's).
//
//	go run fontawesome_gen.go <fontawesome-free-X-desktop>
//
// The font, fonts/fa-solid-900.woff, is the same release's webfonts/fa-solid-900.woff2 as a WOFF 1.0 file, which this package reads with the standard
// library (WOFF2 would want Brotli): fontTools' TTFont(src) with its flavor set to "woff" and saved does it. The license is its LICENSE.txt. Both are
// copied by hand.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type icon struct {
	Unicode string   `json:"unicode"`
	Free    []string `json:"free"`
	Aliases struct {
		Names []string `json:"names"`
	} `json:"aliases"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run fontawesome_gen.go <fontawesome-free-desktop-dir>")
		os.Exit(2)
	}
	raw, err := os.ReadFile(filepath.Join(os.Args[1], "metadata", "icons.json"))
	check(err)
	var all map[string]icon
	check(json.Unmarshal(raw, &all))

	codes := map[string]string{} // class name -> code point, aliases included
	for name, ic := range all {
		if !slices.Contains(ic.Free, "solid") {
			continue
		}
		codes[name] = ic.Unicode
		for _, alias := range ic.Aliases.Names {
			codes[alias] = ic.Unicode
		}
	}
	names := make([]string, 0, len(codes))
	for name := range codes {
		names = append(names, name)
	}
	slices.Sort(names)

	var list strings.Builder
	list.WriteString("\n")
	for _, name := range names {
		fmt.Fprintf(&list, "%s %s\n", name, codes[name])
	}
	check(os.WriteFile("icons.txt", []byte(list.String()), 0o644))
	fmt.Println(len(names), "icons")
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
