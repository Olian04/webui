// Package rank says how well a text matches what a person typed, as a score from 0 to 1, so that the best matches of a search are
// the first. It does not decide what matches: a source of results does, as it can (a database's way, or the library's for rows in
// memory). It only orders what the sources found.
package rank

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// The scores of the ways a text can contain what was typed, best first. A clean match, one that is the whole text, starts it, ends it or
// is a word of its own, is above one that is only a sequence in the middle of a word, and that is above characters that are in
// the text with others between them.
const (
	Exact          = 1.00 // the text is what was typed
	WholePrefix    = 0.95 // it starts the text and is a word of its own: "device" in "device list"
	Prefix         = 0.90 // it starts the text: "dev" in "device"
	WholeSuffix    = 0.85 // it ends the text and is a word of its own: "list" in "device list"
	Suffix         = 0.80 // it ends the text: "ice" in "device"
	WholeWord      = 0.75 // it is a word of its own, inside the text: "list" in "my list here"
	WordStart      = 0.65 // it starts a word inside the text: "dev" in "my device"
	Inside         = 0.40 // it is a sequence inside a word: "evi" in "device"
	Scattered      = 0.20 // its characters are in the text in order, with others between them: "dvc" in "device"
	TitleWeight    = 1.0  // what a hit's title is worth
	DescWeight     = 0.6  // and what its description is worth: a match in it is less than the same in the title
	Floor          = 0.05 // the least a hit gets that a source found: it matched for a reason the title and description do not show
	tokenPenalty   = 0.6  // several words that are all in the text, but not as typed
	minScatterRune = 2    // a single character is in any text, so it says nothing
)

// Score is how well text matches query, from 0, which it does not, to 1, which is exactly. Case is ignored. A query of several
// words that is not in the text as typed scores by its worst word, reduced, when all of them are.
func Score(query, text string) float64 {
	q, t := fold(query), fold(text)
	if q == "" || t == "" {
		return 0
	}
	if s := contains(q, t); s > 0 {
		return s
	}
	if words := strings.Fields(q); len(words) > 1 {
		worst := 1.0
		for _, w := range words {
			s := contains(w, t)
			if s == 0 {
				worst = 0
				break
			}
			worst = min(worst, s)
		}
		if worst > 0 {
			return worst * tokenPenalty
		}
	}
	if utf8.RuneCountInString(q) >= minScatterRune && scattered(q, t) {
		return Scattered
	}
	return 0
}

// Hit is how well a result matches, from its title and its description: the better of the title and the description reduced. A result
// that a source found and neither shows is the Floor, so it is not dropped, and is last.
func Hit(query, title, desc string) float64 {
	return max(Score(query, title)*TitleWeight, Score(query, desc)*DescWeight, Floor)
}

func fold(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// contains is the best way q is in t as it was typed (as one run of characters), or 0 when it is not.
func contains(q, t string) float64 {
	if q == t {
		return Exact
	}
	best := 0.0
	for from := 0; from+len(q) <= len(t); {
		i := strings.Index(t[from:], q)
		if i < 0 {
			break
		}
		at := from + i
		start, end := at == 0, at+len(q) == len(t)
		left := start || !wordRune(lastRune(t[:at]))
		right := end || !wordRune(firstRune(t[at+len(q):]))
		var s float64
		switch {
		case start && right:
			s = WholePrefix
		case start:
			s = Prefix
		case end && left:
			s = WholeSuffix
		case end:
			s = Suffix
		case left && right:
			s = WholeWord
		case left:
			s = WordStart
		default:
			s = Inside
		}
		best = max(best, s)
		_, size := utf8.DecodeRuneInString(t[at:])
		from = at + size
	}
	return best
}

// scattered is whether the characters of q are in t, in order.
func scattered(q, t string) bool {
	rest := t
	for _, r := range q {
		if unicode.IsSpace(r) {
			continue
		}
		i := strings.IndexRune(rest, r)
		if i < 0 {
			return false
		}
		rest = rest[i+utf8.RuneLen(r):]
	}
	return true
}

func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

func firstRune(s string) rune {
	r, _ := utf8.DecodeRuneInString(s)
	return r
}

func lastRune(s string) rune {
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}
