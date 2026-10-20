package phone

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestInternationalNumbersAreNormalisedToE164(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"+46 70 123 45 67":      "+46701234567",
		"  +46 (0)70-123.45/67": "+460701234567",
		"+1 415 555 2671":       "+14155552671",
		"+4915123456789":        "+4915123456789",
	} {
		got, ok := International(in)
		assert.True(t, ok)
		assert.Equal(t, got, want)
		assert.Equal(t, Normalize(in), want)
	}
	// Anything that is not one is as it was typed, without the space around it.
	assert.Equal(t, Normalize(" 070-123 45 67 "), "070-123 45 67")
	assert.Equal(t, Normalize("+46 70 123 45 67 x12"), "+46 70 123 45 67 x12")
	assert.Equal(t, Normalize("+123"), "+123")
}

func TestWhatCanBeAPhoneNumber(t *testing.T) {
	t.Parallel()

	for _, good := range []string{"+46 8 123 456", "(08) 123-45.67", "0701234567 x12", "+46701234567 x12", "+1 415 555 2671"} {
		assert.True(t, Valid(good))
	}
	for _, bad := range []string{"", "call me", "---", "12a45", "+123", "+1234567890123456", "+46 70 abc 45 67", "+12 x34"} {
		assert.False(t, Valid(bad))
	}
}

func TestEveryCountryIsDescribedOnceAndTheScriptCanReadThemAll(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	mains := map[string]int{}
	shared := map[string]int{}
	for _, c := range All() {
		assert.False(t, seen[c.ISO])
		seen[c.ISO] = true
		assert.Equal(t, len(c.ISO), 2)
		assert.True(t, c.Name != "" && c.Code != "" && !strings.HasPrefix(c.Code, "0"))
		assert.True(t, len(c.Code) <= 3)
		for _, g := range c.Groups {
			assert.True(t, g > 0)
		}
		shared[c.Code]++
		if c.Main {
			mains[c.Code]++
		}
	}
	// A code that several countries share names the one that is the main of them.
	for code, n := range shared {
		if n > 1 {
			assert.Equal(t, mains[code], 1)
		}
	}
	assert.True(t, Known("SE") && Known("US") && !Known("ZZ") && !Known("se"))

	var read []map[string]any
	assert.NoError(t, json.Unmarshal(JSON(), &read))
	assert.Equal(t, len(read), len(All()))
}
