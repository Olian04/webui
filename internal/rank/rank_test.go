package rank

import (
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestHowWellATextMatchesWhatWasTyped(t *testing.T) {
	t.Parallel()

	// Best first: each of these is above the next.
	order := []struct{ query, text string }{
		{"device", "Device"},      // exact, whatever the case
		{"device", "Device list"}, // a word that starts it
		{"dev", "Device"},         // a start
		{"list", "Device list"},   // a word that ends it
		{"ice", "Device"},         // an end
		{"list", "My list here"},  // a word in the middle
		{"dev", "My device"},      // the start of a word in the middle
		{"evi", "Device"},         // inside a word
		{"dvc", "Device"},         // its characters, with others between
	}
	prev := 2.0
	for _, tc := range order {
		got := Score(tc.query, tc.text)
		if !(got > 0 && got < prev) {
			t.Errorf("Score(%q, %q) = %v, want it above 0 and below %v", tc.query, tc.text, got, prev)
		}
		prev = got
	}
	assert.Equal(t, Score("device", "device"), Exact)
	assert.Equal(t, Score("dev", "device"), Prefix)
	assert.Equal(t, Score("evi", "device"), Inside)
	assert.Equal(t, Score("dvc", "device"), Scattered)
}

func TestWhatDoesNotMatchScoresNothing(t *testing.T) {
	t.Parallel()

	for _, tc := range [][2]string{{"x", "device"}, {"", "device"}, {"device", ""}, {"cde", "device"}, {"a", "b"}} {
		assert.Equal(t, Score(tc[0], tc[1]), 0.0)
	}
	// A single character is in many texts and says nothing in order, so only as itself.
	assert.Equal(t, Score("z", "a b c"), 0.0)
}

func TestTheBestOccurrenceCounts(t *testing.T) {
	t.Parallel()

	// "list" is inside a word first, and a word of its own after: the word is what is scored.
	assert.Equal(t, Score("list", "playlist list"), WholeSuffix)
	assert.Equal(t, Score("a", "banana a"), WholeSuffix)
}

func TestSeveralWordsThatAreAllThereScoreBelowTheMatchOfThePhrase(t *testing.T) {
	t.Parallel()

	phrase := Score("eth 0", "dev-eth 0")
	words := Score("0 eth", "dev-eth 0") // the same words, not as typed
	if !(words > 0 && words < phrase) {
		t.Errorf("words %v, phrase %v", words, phrase)
	}
	assert.Equal(t, Score("eth nothing", "dev-eth 0"), Score("eth nothing", "dev-eth 0")) // not all of them: scattered or nothing, never a word match
	assert.True(t, Score("eth nothing", "dev-eth 0") <= Scattered)
}

func TestAHitIsScoredByItsTitleAndLessByItsDescriptionAndNeverBelowTheFloor(t *testing.T) {
	t.Parallel()

	title := Hit("gate", "gateway", "")
	desc := Hit("gate", "x", "gateway")
	assert.True(t, title > desc)
	assert.Equal(t, desc, DescWeight*Prefix)
	assert.Equal(t, Hit("zzz", "x", "y"), Floor) // a source found it for a reason that neither shows
	assert.Equal(t, Hit("gate", "gate", "gateway"), Exact)
}

func TestAccentedAndWideCharactersAreWholeCharacters(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Score("malm", "Malmö"), Prefix)
	assert.Equal(t, Score("ö", "Malmö"), Suffix)
	assert.Equal(t, Score("göteborg", "Göteborg"), Exact)
	assert.Equal(t, Score("本", "日本語"), Inside)
}
