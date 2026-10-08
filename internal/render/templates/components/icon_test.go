package components

import (
	"os"
	"regexp"
	"testing"

	"github.com/Olian04/webui/internal/render/assets"
)

// Every icon the design names must be one the icon font has: a typo would draw
// nothing. The constants are read from the source.
func TestEveryIconNameIsInTheIconFont(t *testing.T) {
	src, err := os.ReadFile("icon.templ")
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, m := range regexp.MustCompile(`(?m)^\s+Icon\w+\s+IconName = "(.+)"$`).FindAllStringSubmatch(string(src), -1) {
		found++
		if !assets.HasIcon(m[1]) {
			t.Errorf("icon %q is not a Font Awesome Free solid icon", m[1])
		}
	}
	if found == 0 {
		t.Fatal("found no icon names; the pattern is stale")
	}
}
