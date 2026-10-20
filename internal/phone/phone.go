// Package phone is what is known of phone numbers: the countries and their calling codes, how a national number is spaced for
// reading, and what an international number is. It is the one place the page's script and the server read it from, so that
// what the script offers is what the server accepts.
package phone

import (
	"encoding/json"
	"strings"
)

// Country is a country that has a calling code of its own.
type Country struct {
	ISO    string // ISO 3166 alpha-2: "SE"
	Name   string // in English
	Code   string // the calling code without the "+": "46"
	Groups []int  // how many digits are in each group of a national number, in order; the rest is one group. Nil: threes
	Trunk  bool   // the national format has a leading "0", which the international one drops
	Main   bool   // of the countries that share Code, the one a number is taken to be of
}

// All is every country, by name.
func All() []Country { return countries }

// Known is whether iso is the code of a country in the list.
func Known(iso string) bool {
	for _, c := range countries {
		if c.ISO == iso {
			return true
		}
	}
	return false
}

// JSON is the countries as the script reads them: an array of {i, n, c, g, t, m}, the keys short because the page is sent
// with it, and the groups, trunk and main left out where they are not set.
func JSON() []byte {
	type entry struct {
		I string `json:"i"`
		N string `json:"n"`
		C string `json:"c"`
		G []int  `json:"g,omitempty"`
		T int    `json:"t,omitempty"`
		M int    `json:"m,omitempty"`
	}
	out := make([]entry, len(countries))
	for i, c := range countries {
		e := entry{I: c.ISO, N: c.Name, C: c.Code, G: c.Groups}
		if c.Trunk {
			e.T = 1
		}
		if c.Main {
			e.M = 1
		}
		out[i] = e
	}
	b, err := json.Marshal(out)
	if err != nil {
		return []byte("[]") // cannot happen: strings and numbers
	}
	return b
}

// Digits are the most that an international number has, with its country code: what E.164 allows. The fewest is a number
// a country code and a few digits make, which is shorter than any number in use but the smallest countries'.
const (
	maxDigits = 15
	minDigits = 7
)

// separators are what a number is written with besides its digits.
const separators = " ().-/"

// International reads a number that starts with "+", written with spaces and the punctuation of one, as E.164: a "+" and its
// digits. ok is false for text that is not one: a number with letters (an extension, "x12", is not part of E.164), or with
// more digits than E.164 has or fewer than a number has.
func International(raw string) (e164 string, ok bool) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "+") {
		return "", false
	}
	var b strings.Builder
	b.WriteByte('+')
	for _, r := range raw[1:] {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case strings.ContainsRune(separators, r):
		default:
			return "", false
		}
	}
	n := b.Len() - 1
	if n < minDigits || n > maxDigits {
		return "", false
	}
	return b.String(), true
}

// Normalize is what to store for a number that was submitted: an international number as E.164, and anything else as it was
// typed, without the spaces around it. A value that is not a number is returned as it is: the check is Valid's.
func Normalize(raw string) string {
	if e164, ok := International(raw); ok {
		return e164
	}
	return strings.TrimSpace(raw)
}

// Valid is whether text can be a phone number: an international one that E.164 can hold, or digits with the punctuation of a
// number, an extension's "x" among it, and at least one digit.
func Valid(raw string) bool {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "+") {
		if _, ok := International(raw); ok {
			return true
		}
		// An extension is not E.164, and the number before it is held to it.
		if before, _, found := strings.Cut(raw, "x"); found {
			_, ok := International(before)
			return ok
		}
		return false
	}
	digit := false
	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9':
			digit = true
		case strings.ContainsRune(separators+"+x", r):
		default:
			return false
		}
	}
	return digit
}
