package phone

import (
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

func TestEveryCountryIsDescribedOnce(t *testing.T) {
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
}

func TestANumberIsOfTheCountryWhoseCodeIsTheLongestThatItStartsWith(t *testing.T) {
	t.Parallel()

	for digits, iso := range map[string]string{"46701234567": "SE", "358401234567": "FI", "14155552671": "US", "4915123456789": "DE"} {
		c, ok := CountryOf(digits)
		assert.True(t, ok)
		assert.Equal(t, c.ISO, iso)
	}
	_, ok := CountryOf("0701234567")
	assert.False(t, ok)
	_, ok = CountryOf("")
	assert.False(t, ok)

	c, national, ok := Split("+46 70 123 45 67")
	assert.True(t, ok)
	assert.Equal(t, [2]string{c.ISO, national}, [2]string{"SE", "701234567"})
	_, _, ok = Split("070 123 45 67")
	assert.False(t, ok)
	_, _, ok = Split("+46 70 123 x12")
	assert.False(t, ok)
}

func TestADigitsOfANationalNumberAreSpacedAsTheCountryHasThem(t *testing.T) {
	t.Parallel()

	se, _ := ByISO("SE")
	assert.Equal(t, Spaced(se, "701234567"), "70 123 45 67")
	assert.Equal(t, Spaced(se, "7012"), "70 12")
	assert.Equal(t, Spaced(se, ""), "")
	assert.Equal(t, Spaced(se, "70123456789"), "70 123 45 67 89") // the rest is one group
	af, _ := ByISO("AF")                                          // no groups known: threes, and no lone digit at the end
	assert.Equal(t, Spaced(af, "7012345"), "701 234 5"[:0]+"701 2345")
	assert.Equal(t, Spaced(af, "701234"), "701 234")
}

func TestACountryAndANationalNumberMakeAnE164NumberAsTheNationalFormatAllows(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ iso, in, want string }{
		{"SE", "070 123 45 67", "+46701234567"}, // the trunk 0 is dropped
		{"SE", "70 123 45 67", "+46701234567"},
		{"SE", "(070) 123-45.67", "+46701234567"},
		{"US", "415 555 2671", "+14155552671"},           // a country with no trunk 0 keeps a 0 that it has
		{"SE", "+49 151 2345 6789", "+49 151 2345 6789"}, // a whole number has its own code, whatever was chosen
		{"", "070 123 45 67", "070 123 45 67"},           // no country: as typed
		{"ZZ", "070 123 45 67", "070 123 45 67"},         // a country that is not one
		{"SE", "070 123 x12", "070 123 x12"},             // an extension is not composed, and is held to the rules as it is
		{"SE", "call me", "call me"},
		{"SE", "   ", ""},
		{"SE", "---", "---"},
	} {
		assert.Equal(t, Compose(c.iso, c.in), c.want)
	}
}

func TestACountryIsFoundInTheLanguagesOfTheVisitor(t *testing.T) {
	t.Parallel()

	for header, want := range map[string]string{
		"sv-SE,sv;q=0.9,en;q=0.8": "SE",
		"en-US,en;q=0.9":          "US",
		"en;q=0.8, de-DE;q=0.9":   "DE", // the likelier first
		"en, fr-ZZ, nb-NO":        "NO", // a region that is not in the list is passed over
		"en":                      "",
		"":                        "",
		"zh-Hans-CN":              "CN",
		"pt_BR":                   "BR",
		"*":                       "",
		"en-US;q=oops":            "US",
	} {
		assert.Equal(t, FromLanguages(header), want)
	}
}
