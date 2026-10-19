package rules

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// A pattern is run twice: by Go, on the server, and by the browser, as the pattern attribute of the input, which is a
// JavaScript regular expression. The two read most of what is written alike and differ in places that do not announce
// themselves: a pattern that Go accepts and a browser does not is ignored by the browser without a word, so the field is
// not checked in the page at all; a pattern that both accept can match different values. portable reads the pattern
// itself, and accepts only the syntax that has been held against both engines (testdata/portable.json is what a
// browser's engine said about each construct, and TestThePortableSubsetIsWhatBothEnginesAgreeOn holds the answers of
// this package against it; the file is made by testdata/gen.mjs). The subset is, outside a class:
//
//	literal characters                         any but  \ ^ $ . | ? * + ( ) [ ] { }  which are escaped with a backslash
//	.                                          any character but a line end, as in a browser (see lineEnds)
//	\d \D \w \W \b \B                          ASCII in both
//	\s \S                                      white space as a browser has it (see white)
//	\t \n \r \f \v  \xHH                       the character
//	\p{L} \P{L}                                a general category by its short name (see categories)
//	(...) (?:...) | * + ? {n} {n,} {n,m}       and a lazy ? after a repetition; ^ and $
//
// and inside a class, [...] or [^...]: the same characters and escapes, ranges a-z between two characters, and a literal
// that is none of ( ) [ ] { } / | - \ , the ones a browser's "v" flag reserves, which are written with a backslash, and
// never twice in a row.
//
// It returns the expression for Go: the pattern, with what a browser reads differently written as what the browser
// reads. The error is a [PatternError].
func portable(expr string) (string, *PatternError) {
	p := &portableReader{expr: expr}
	p.alt(0)
	if p.err != nil {
		return "", p.err
	}
	return p.out.String(), nil
}

type portableReader struct {
	expr string
	pos  int
	out  strings.Builder
	err  *PatternError
}

// The line ends of a browser, which a dot does not match. Go's dot excludes only \n.
const lineEnds = `\n\r\x{2028}\x{2029}`

// white is what a browser's \s matches: its white space and its line ends. Go's \s is [\t\n\f\r ] and nothing else.
var white = []runeRange{{9, 13}, {32, 32}, {0xa0, 0xa0}, {0x1680, 0x1680}, {0x2000, 0x200a}, {0x2028, 0x2029}, {0x202f, 0x202f}, {0x205f, 0x205f}, {0x3000, 0x3000}, {0xfeff, 0xfeff}}

type runeRange struct{ lo, hi rune }

// ranges writes ranges as the inside of a Go class.
func ranges(rs []runeRange) string {
	var b strings.Builder
	for _, r := range rs {
		if r.lo == r.hi {
			fmt.Fprintf(&b, `\x{%x}`, r.lo)
		} else {
			fmt.Fprintf(&b, `\x{%x}-\x{%x}`, r.lo, r.hi)
		}
	}
	return b.String()
}

// notWhite is every character that is not white, for \S inside a class, where Go has no way to negate a part of one.
var notWhite = func() []runeRange {
	var out []runeRange
	next := rune(0)
	for _, r := range white {
		if r.lo > next {
			out = append(out, runeRange{next, r.lo - 1})
		}
		next = r.hi + 1
	}
	return append(out, runeRange{next, utf8.MaxRune})
}()

// categories are the general categories that Go and a browser agree on, for every code point, by their short names. The
// C family is out: what is unassigned is a matter of the version of Unicode of the engine, and so is anything that
// is assigned later in any category, which is a difference of the age of the browser and not of the syntax.
var categories = map[string]bool{
	"L": true, "Lu": true, "Ll": true, "Lt": true, "Lm": true, "Lo": true,
	"M": true, "Mn": true, "Mc": true, "Me": true,
	"N": true, "Nd": true, "Nl": true, "No": true,
	"P": true, "Pc": true, "Pd": true, "Ps": true, "Pe": true, "Pi": true, "Pf": true, "Po": true,
	"S": true, "Sm": true, "Sc": true, "Sk": true, "So": true,
	"Z": true, "Zs": true, "Zl": true, "Zp": true,
	"Cc": true, "Cf": true,
}

// fail records the first problem. what is the construct as written, and says is what is wrong with it.
func (p *portableReader) fail(what, says, fix string) {
	if p.err == nil {
		p.err = &PatternError{Expr: p.expr, Problem: "uses " + quote(what) + says, Fix: fix}
	}
}

func (p *portableReader) more() bool { return p.err == nil && p.pos < len(p.expr) }

func (p *portableReader) peek() byte {
	if p.pos < len(p.expr) {
		return p.expr[p.pos]
	}
	return 0
}

// alt reads up to the end of the pattern, or of the group it is in.
func (p *portableReader) alt(depth int) {
	for p.more() {
		switch c := p.peek(); {
		case c == ')' && depth > 0:
			return
		case c == '|':
			p.pos++
			p.out.WriteByte('|')
		default:
			p.atom(depth)
		}
	}
}

// atom reads one thing and the repetition after it.
func (p *portableReader) atom(depth int) {
	start := p.pos
	repeatable := true
	switch c := p.peek(); c {
	case '^', '$':
		p.pos++
		p.out.WriteByte(c)
		repeatable = false
	case '.':
		p.pos++
		p.out.WriteString(`[^` + lineEnds + `]`)
	case '(':
		p.group(depth)
	case '[':
		p.class()
	case '\\':
		repeatable = p.escape()
	case '*', '+', '?':
		p.fail(string(c), ", with nothing before it to repeat", "Put what is to be repeated before it, or write a backslash before it for a literal one.")
	case '{':
		p.fail("{", ", which a browser does not read as a character unless it is a repetition", `Write \{ for a literal one, and {n}, {n,} or {n,m} for a repetition, after what it repeats.`)
	case '}', ']':
		p.fail(string(c), ", which a browser does not read as a character", `Write \`+string(c)+" for a literal one.")
	case ')':
		p.fail(")", " with no group to close", `Write \) for a literal one.`)
	default:
		r, n := utf8.DecodeRuneInString(p.expr[p.pos:])
		p.pos += n
		p.out.WriteString(string(r))
	}
	if p.err != nil {
		return
	}
	p.repetition(p.expr[start:p.pos], repeatable)
}

// repetition reads the *, +, ?, {n}, {n,} or {n,m} after what was read, and the ? that makes it lazy.
func (p *portableReader) repetition(of string, repeatable bool) {
	c := p.peek()
	if c != '*' && c != '+' && c != '?' && c != '{' {
		return
	}
	at := p.pos
	if c == '{' {
		end := strings.IndexByte(p.expr[p.pos:], '}')
		if end < 0 || !isRepeat(p.expr[p.pos+1:p.pos+end]) {
			p.fail("{", ", which a browser does not read as a character unless it is a repetition", `Write \{ for a literal one, and {n}, {n,} or {n,m} for a repetition.`)
			return
		}
		p.pos += end + 1
	} else {
		p.pos++
	}
	if !repeatable {
		p.fail(of+p.expr[at:p.pos], ", which repeats something that is not a character", "A browser does not allow it, and it would be a repetition of nothing: remove it.")
		return
	}
	p.out.WriteString(p.expr[at:p.pos])
	if p.peek() == '?' { // lazy
		p.pos++
		p.out.WriteByte('?')
	}
	if c := p.peek(); c == '*' || c == '+' || c == '?' || c == '{' {
		p.fail(p.expr[at:p.pos]+string(c), ", which repeats a repetition", "Repeat each thing once. A group repeats what is in it: (?:a+)* says it.")
	}
}

// isRepeat is whether the inside of braces is a count: digits, or digits, a comma and digits or none.
func isRepeat(s string) bool {
	lo, hi, comma := s, "", false
	if i := strings.IndexByte(s, ','); i >= 0 {
		lo, hi, comma = s[:i], s[i+1:], true
	}
	digits := func(d string) bool {
		for _, r := range d {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	}
	return lo != "" && digits(lo) && digits(hi) && (comma || hi == "")
}

func (p *portableReader) group(depth int) {
	open := p.pos
	p.pos++
	if p.peek() == '?' {
		rest := p.expr[open:]
		switch {
		case strings.HasPrefix(rest, "(?:"):
			p.pos += 2
			p.out.WriteString("(?:")
		case strings.HasPrefix(rest, "(?P<"):
			p.fail("(?P<", ", which a browser does not read for a named group", "A group in a pattern has no use for a name: use (?:...), or (...) for a group that is kept.")
			return
		case strings.HasPrefix(rest, "(?<"):
			p.fail("(?<", ", a named group, which a pattern has no use for", "Use (?:...), or (...).")
			return
		default:
			what := rest
			if i := strings.IndexAny(rest, ":)"); i >= 0 {
				what = rest[:i+1]
			}
			p.fail(what, ", a group with flags, which not every browser reads", `Write what the flag does in the pattern itself: [Aa] for a letter of either case. A pattern is matched against the whole value already, so there is no need for (?m) or (?s).`)
			return
		}
	} else {
		p.out.WriteByte('(')
	}
	p.alt(depth + 1)
	if p.err != nil {
		return
	}
	if p.peek() != ')' { // Go has read the pattern, so it is balanced; this is for a reader that is used alone
		p.fail(p.expr[open:], ", a group that is not closed", "Close it with a ).")
		return
	}
	p.pos++
	p.out.WriteByte(')')
}

// escape reads a backslash outside a class, and says whether what it read can be repeated.
func (p *portableReader) escape() bool {
	if p.pos+1 >= len(p.expr) {
		p.fail(`\`, ", with nothing after it", `Write \\ for a backslash.`)
		return false
	}
	r, n := utf8.DecodeRuneInString(p.expr[p.pos+1:])
	at := p.expr[p.pos : p.pos+1+n]
	switch {
	case strings.ContainsRune(`^$.*+?()[]{}|\/`, r):
		p.out.WriteString(at)
		p.pos += 1 + n
		return true
	case r == 'b' || r == 'B':
		p.out.WriteString(at)
		p.pos += 2
		return false
	case r == 's':
		p.out.WriteString(`[` + ranges(white) + `]`)
		p.pos += 2
	case r == 'S':
		p.out.WriteString(`[^` + ranges(white) + `]`)
		p.pos += 2
	case r == 'd' || r == 'D' || r == 'w' || r == 'W' || r == 't' || r == 'n' || r == 'r' || r == 'f' || r == 'v':
		p.out.WriteString(at)
		p.pos += 2
	default:
		if end, ok := p.shared(r, at); ok {
			p.out.WriteString(p.expr[p.pos:end])
			p.pos = end
		}
	}
	return p.err == nil
}

// shared reads the escapes that are the same inside a class and outside, other than the letters, and says where it ended:
// \xHH and \p{L}. For anything else it records why the escape is not one that both engines read.
func (p *portableReader) shared(r rune, at string) (end int, ok bool) {
	switch {
	case r == 'x':
		h := p.expr[p.pos+2 : min(len(p.expr), p.pos+4)]
		if len(h) == 2 && isHex(h) {
			return p.pos + 4, true
		}
		if strings.HasPrefix(h, "{") {
			h = "{"
		}
		p.fail(`\x`+h, ", which a browser reads only with two hexadecimal digits", `Write \xHH, as in \x41, or the character itself.`)
	case r == 'p' || r == 'P':
		return p.property(at)
	case r >= '1' && r <= '9', r == '0':
		p.fail(at, ", which a browser does not read as Go does", "Write the character itself, or \\xHH. A browser reads \\1 as a reference to a group, which Go does not have.")
	case r == 'A' || r == 'z' || r == 'Z':
		p.fail(at, ", which a browser does not have", "A pattern has to match the whole value already, so there is nothing to anchor: write ^ and $, or nothing.")
	case r == 'Q' || r == 'E':
		p.fail(at, ", which a browser does not have", `Write a backslash before each character that has a meaning, as in a\.b.`)
	case r < utf8.RuneSelf && !isAlnum(r):
		p.fail(at, ", a backslash before a character that has no meaning, which a browser does not allow", "Write "+quote(string(r))+" without the backslash.")
	default:
		p.fail(at, ", which is not an escape that both Go and a browser read", "Write the character itself, or \\xHH for one that cannot be typed.")
	}
	return 0, false
}

func isHex(s string) bool {
	for _, r := range s {
		if !isAlnum(r) || strings.ContainsRune("ghijklmnopqrstuvwxyzGHIJKLMNOPQRSTUVWXYZ", r) {
			return false
		}
	}
	return true
}

func isAlnum(c rune) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// property reads \p{L} or \P{L} at the position, which is at the backslash.
func (p *portableReader) property(at string) (end int, ok bool) {
	rest := p.expr[p.pos+2:]
	if !strings.HasPrefix(rest, "{") {
		name := ""
		if rest != "" {
			name = rest[:1]
		}
		p.fail(at+name, ", which a browser does not read without braces", `Write \`+at[1:]+"{"+name+"}.")
		return 0, false
	}
	i := strings.IndexByte(rest, '}')
	if i < 0 {
		p.fail(at+rest, ", a property that is not closed", "Close it with a }.")
		return 0, false
	}
	name := rest[1:i]
	whole := at + "{" + name + "}"
	if !categories[name] {
		p.fail(whole, ", which is not a name that Go and a browser both know", "Use a general category by its short name, such as \\p{L} for a letter, \\p{Lu} for an upper case letter, \\p{N} for a number or \\p{P} for punctuation; a script, such as Greek, is named differently in each, so write its characters as a class instead, as in [α-ω].")
		return 0, false
	}
	return p.pos + 2 + i + 1, true
}

// class reads [...] or [^...] and writes it for Go, with \s and \S as the characters that they are in a browser.
func (p *portableReader) class() {
	open := p.pos
	p.pos++
	p.out.WriteByte('[')
	if p.peek() == '^' {
		p.pos++
		p.out.WriteByte('^')
	}
	empty, last, canRange := true, rune(-1), false
	for p.err == nil {
		if p.pos >= len(p.expr) {
			p.fail(p.expr[open:], ", a class that is not closed", "Close it with a ].")
			return
		}
		c := p.peek()
		if c == ']' && !empty {
			p.pos++
			p.out.WriteByte(']')
			return
		}
		if c == '-' {
			if !canRange || strings.HasPrefix(p.expr[p.pos:], "-]") || p.pos+1 >= len(p.expr) {
				p.fail("-", " in a class, where a browser does not read it as a character unless it is between two characters of a range", `Write \- for a literal one.`)
				return
			}
			p.pos++
			lo := last
			hi, ok := p.classChar()
			if !ok {
				return
			}
			if hi < lo {
				p.fail(fmt.Sprintf("%c-%c", lo, hi), ", a range that runs backwards", "Write the lower character first.")
				return
			}
			p.out.WriteString("-")
			p.writeChar(hi)
			empty, canRange = false, false
			last = -1
			continue
		}
		r, isChar := p.classItem()
		if p.err != nil {
			return
		}
		empty = false
		canRange = isChar
		last = r
	}
}

// classItem reads one character, or one class of characters such as \d, inside a class, and writes it. It says which
// character it was when it was one, for a range to start from.
func (p *portableReader) classItem() (rune, bool) {
	c := p.peek()
	if c == '\\' && p.pos+1 < len(p.expr) {
		switch e := p.expr[p.pos+1]; e {
		case 's':
			p.pos += 2
			p.out.WriteString(ranges(white))
			return 0, false
		case 'S':
			p.pos += 2
			p.out.WriteString(ranges(notWhite))
			return 0, false
		case 'd', 'D', 'w', 'W':
			p.pos += 2
			p.out.WriteString(`\` + string(e))
			return 0, false
		case 'p', 'P':
			end, ok := p.property(p.expr[p.pos : p.pos+2])
			if ok {
				p.out.WriteString(p.expr[p.pos:end])
				p.pos = end
			}
			return 0, false
		}
	}
	r, ok := p.classChar()
	if ok {
		p.writeChar(r)
	}
	return r, ok
}

// writeChar writes a character of a class so that Go reads it as the same character.
func (p *portableReader) writeChar(r rune) {
	switch {
	case r < 0x20 || r == 0x7f:
		fmt.Fprintf(&p.out, `\x{%x}`, r)
	case strings.ContainsRune(`\]^-[`, r):
		p.out.WriteByte('\\')
		p.out.WriteRune(r)
	default:
		p.out.WriteRune(r)
	}
}

// reserved are the characters that a browser's "v" flag does not allow in a class unless they are escaped, and the ones that
// it reads two of as an operator.
const (
	reservedAlone = `()[]{}/|`
	reservedTwice = "&!#$%*+,.:;<=>?@^`~"
)

// classChar reads a character of a class, as itself or as an escape, and says which.
func (p *portableReader) classChar() (rune, bool) {
	if p.pos >= len(p.expr) {
		p.fail(p.expr, ", a class that is not closed", "Close it with a ].")
		return 0, false
	}
	r, n := utf8.DecodeRuneInString(p.expr[p.pos:])
	if r != '\\' {
		switch {
		case r == ']' || r == '-':
			p.fail(string(r), " in a class, where a browser does not read it as a character", `Write \`+string(r)+" for a literal one.")
			return 0, false
		case strings.ContainsRune(reservedAlone, r):
			p.fail(string(r), " in a class, where a browser reads it as part of a syntax of its own", `Write \`+string(r)+" for a literal one.")
			return 0, false
		case strings.ContainsRune(reservedTwice, r) && strings.HasPrefix(p.expr[p.pos+n:], string(r)):
			p.fail(string(r)+string(r), " in a class, where a browser reads two of them as an operator", `Write \`+string(r)+" for each, or put a character between them.")
			return 0, false
		}
		p.pos += n
		return r, true
	}
	if p.pos+1 >= len(p.expr) {
		p.fail(`\`, ", with nothing after it", `Write \\ for a backslash.`)
		return 0, false
	}
	e, en := utf8.DecodeRuneInString(p.expr[p.pos+1:])
	at := p.expr[p.pos : p.pos+1+en]
	switch {
	case strings.ContainsRune(`^$.*+?()[]{}|\/-`, e):
		p.pos += 1 + en
		return e, true
	case e == 't' || e == 'n' || e == 'r' || e == 'f' || e == 'v':
		p.pos += 2
		return map[rune]rune{'t': '\t', 'n': '\n', 'r': '\r', 'f': '\f', 'v': '\v'}[e], true
	case strings.ContainsRune("dDwWsSpP", e):
		p.fail(at, ", a class of characters, as an end of a range", "A range runs between two characters, as in a-z.")
		return 0, false
	case e == 'x':
		if end, ok := p.shared(e, at); ok {
			v, _ := strconv.ParseUint(p.expr[p.pos+2:end], 16, 8) // two hexadecimal digits: isHex has seen them
			p.pos = end
			return rune(v), true
		}
		return 0, false
	}
	p.shared(e, at)
	if p.err == nil { // not reached: shared reads only what is handled above
		p.fail(at, ", which a browser does not read as a character", "Write the character itself.")
	}
	return 0, false
}
