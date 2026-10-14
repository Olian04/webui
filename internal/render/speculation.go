package render

import (
	"encoding/json"
	"strings"
)

const (
	// speculationName is the logical name of the rules that tell a browser which pages to fetch ahead.
	speculationName = "speculation.json"

	// speculationMime is the type a browser requires of rules it is pointed at by a header.
	speculationMime = "application/speculationrules+json"
)

// notAhead are the links that are not fetched ahead: a download, a link that opens elsewhere, and the
// links of a panel that the page's script follows in place (sorting, paging, filtering, tabs), which
// lead to the page that is on screen.
var notAhead = []string{
	"[download]", "[target]",
	"[data-leaf] th a", "[data-leaf] .pager a", "[data-leaf] details.filter a", "[data-leaf] nav.tabs a",
}

// speculationRules is the rules, as a browser reads them: fetch a link of the app when the pointer has
// rested on it for a moment, so that the page is there when it is followed. Only a fetch, never a
// prerender: pages are no-store, which a browser does not prerender. The links that are not fetched
// ahead are notAhead; everything outside the mount prefix is not the app's.
func speculationRules(prefix string) ([]byte, error) {
	type m = map[string]any
	return json.Marshal(m{"prefetch": []m{{
		"where": m{"and": []m{
			{"href_matches": patternEscape(prefix) + "/*"},
			{"not": m{"selector_matches": notAhead}},
		}},
		"eagerness": "moderate",
	}}})
}

// patternEscape is s as a literal in a URL pattern, where these characters mean something.
func patternEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`\+*?:(){}[]!`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// SpeculationRules is the value of the Speculation-Rules header of a full page: the address of the
// rules, in quotes as the header is written.
func (r *Renderer) SpeculationRules() string { return `"` + r.AssetHref(speculationName) + `"` }
