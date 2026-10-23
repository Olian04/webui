// Package phone is what is known of phone numbers: the countries and their calling codes, how a national number is spaced for
// reading, what an international number is, and how a country and a national number make one. It is all the server's: a form
// is drawn with the countries in it and a number is read here, so that what the page offers is what the server accepts and
// script is left with nothing to know.
package phone

import (
	"sort"
	"strconv"
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

// ByISO is the country that has the code iso.
func ByISO(iso string) (Country, bool) {
	for _, c := range countries {
		if c.ISO == iso {
			return c, true
		}
	}
	return Country{}, false
}

// CountryOf is the country that a number is of, by its code: the longest code that its digits start with, which is what makes "358" Finland and not "3" and a code that
// is not one, and of the countries that share a code the main one. It is false for digits that start with no code.
func CountryOf(digits string) (Country, bool) {
	for n := 3; n >= 1; n-- {
		if len(digits) < n {
			continue
		}
		var found Country
		var ok bool
		for _, c := range countries {
			if c.Code == digits[:n] && (!ok || c.Main) {
				found, ok = c, true
			}
		}
		if ok {
			return found, true
		}
	}
	return Country{}, false
}

// digitsOf is the digits of text, with nothing else.
func digitsOf(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Split is an international number as the country it is of and the digits after its code, which is how a form shows it: the country in a selector and the rest in an
// input. It is false for a number that is not international, or is of no country in the list.
func Split(e164 string) (country Country, national string, ok bool) {
	number, isInternational := International(e164)
	if !isInternational {
		return Country{}, "", false
	}
	digits := digitsOf(number)
	c, found := CountryOf(digits)
	if !found {
		return Country{}, "", false
	}
	return c, digits[len(c.Code):], true
}

// Spaced is the digits of a national number in groups, spaced for reading: as the country says, with the rest in one, or in threes for a country that does not
// say. A group of one is joined to the one before it, so that nothing ends in a lone digit.
func Spaced(c Country, digits string) string {
	var out []string
	at := 0
	if len(c.Groups) > 0 {
		for _, size := range c.Groups {
			if at >= len(digits) {
				break
			}
			end := min(at+size, len(digits))
			out = append(out, digits[at:end])
			at = end
		}
		if at < len(digits) {
			out = append(out, digits[at:])
		}
		return strings.Join(out, " ")
	}
	for at < len(digits) {
		end := min(at+3, len(digits))
		out = append(out, digits[at:end])
		at = end
	}
	if n := len(out); n > 1 && len(out[n-1]) == 1 {
		out[n-2] += out[n-1]
		out = out[:n-1]
	}
	return strings.Join(out, " ")
}

// Compose is the number that a country and the text of a national number make: "+" and the code and the digits, without the trunk 0 that the national format has
// and the international one drops. Text that begins with "+" is a whole number, with its own code, whatever country was chosen. Text that has anything in it that is
// not a digit or the punctuation of a number (letters, an extension), or no country, or no digits, is returned as it was typed, to be held to the rules as it is.
func Compose(iso, national string) string {
	national = strings.TrimSpace(national)
	if national == "" || strings.HasPrefix(national, "+") {
		return national
	}
	c, ok := ByISO(iso)
	if !ok {
		return national
	}
	for _, r := range national {
		if (r < '0' || r > '9') && !strings.ContainsRune(separators, r) {
			return national
		}
	}
	digits := digitsOf(national)
	if c.Trunk && strings.HasPrefix(digits, "0") {
		digits = digits[1:]
	}
	if digits == "" {
		return national
	}
	return "+" + c.Code + digits
}

// FromLanguages is the country that a visitor's languages name, for a form that has no number to start at: the first of the languages, the likeliest first, that has a
// region that is in the list ("sv-SE" is Sweden). It is the Accept-Language header, as the browser sent it, and the answer is "" when it names none.
func FromLanguages(accept string) string {
	type tag struct {
		region string
		q      float64
		at     int
	}
	var tags []tag
	for i, part := range strings.Split(accept, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		q := 1.0
		if v, found := strings.CutPrefix(strings.TrimSpace(params), "q="); found {
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				q = parsed
			}
		}
		parts := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
		for _, sub := range parts[min(1, len(parts)):] { // the subtags after the language
			if len(sub) == 2 {
				tags = append(tags, tag{region: strings.ToUpper(sub), q: q, at: i})
				break
			}
		}
	}
	sort.SliceStable(tags, func(i, j int) bool { return tags[i].q > tags[j].q })
	for _, t := range tags {
		if Known(t.region) {
			return t.region
		}
	}
	return ""
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
