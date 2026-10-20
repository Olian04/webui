package webui

import (
	"errors"
	"fmt"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/rules"
)

// Email projects a string field of M that holds an email address. The browser draws an email input, which has the right keyboard
// on a phone and refuses a value that is not an address, in the words that the server uses; and the server checks it again.
// It is held to the HTML standard's grammar, which is what the browser holds it to, and is looser than the RFCs: "a@b" is an
// address. Otherwise it is a [String], with the rules and the Placeholder of one, but not a Secret or a text area, which are inputs of their own.
type Email[M any] struct {
	// Label is the column header and the input's label. In a table it also names the
	// column in the address and in a [Query].
	Label string

	// Load reads the address from the model. An empty one is no value.
	Load func(M) string

	// Store writes the submitted address to the model. Without it the field is read-only.
	Store func(*M, string)

	// Rules are checked in the browser and again on the server.
	Rules StringRules

	// Placeholder is shown in the input while it is empty.
	Placeholder string

	// NoSort leaves the sort link off the column's header, in a table: it cannot be sorted by,
	// and a sort by it in the address is ignored. It does nothing in a form.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table: it cannot be filtered by,
	// and a filter on it in the address is ignored. It does nothing in a form.
	NoFilter bool
}

func (a Email[M]) label() string { return a.Label }

func (a Email[M]) exclusions() (noSort, noFilter bool) { return a.NoSort, a.NoFilter }

func (Email[M]) toolbarItem(M) {}

// Phone projects a string field of M that holds a phone number. The browser draws a telephone input, which has a keypad on a phone and
// checks nothing; the server checks that it is digits with the punctuation that numbers are written with ("+46 8 123 45", "(08) 123-45",
// "0701234567 x12"), and at least one digit. There is no grammar for phone numbers, so it is not stricter than that.
// Otherwise it is a [String].
type Phone[M any] struct {
	// Label is the column header and the input's label. In a table it also names the
	// column in the address and in a [Query].
	Label string

	// Load reads the number from the model. An empty one is no value.
	Load func(M) string

	// Store writes the submitted number to the model. Without it the field is read-only.
	Store func(*M, string)

	// Rules are checked in the browser and again on the server.
	Rules StringRules

	// Placeholder is shown in the input while it is empty.
	Placeholder string

	// NoSort leaves the sort link off the column's header, in a table: it cannot be sorted by,
	// and a sort by it in the address is ignored. It does nothing in a form.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table: it cannot be filtered by,
	// and a filter on it in the address is ignored. It does nothing in a form.
	NoFilter bool
}

func (a Phone[M]) label() string { return a.Label }

func (a Phone[M]) exclusions() (noSort, noFilter bool) { return a.NoSort, a.NoFilter }

func (Phone[M]) toolbarItem(M) {}

// Website projects a string field of M that holds a web address, to be typed: the browser draws a web address input and refuses a value
// that is not an address. Read, it is text; [URL] is the accessor that links. The server holds the value to more than the browser does,
// which accepts any address with a scheme, "javascript:" among them: it is an http or https address with a host, since what is stored
// is likely to be linked, and the rules popover says so. Otherwise it is a [String].
type Website[M any] struct {
	// Label is the column header and the input's label. In a table it also names the
	// column in the address and in a [Query].
	Label string

	// Load reads the address from the model. An empty one is no value.
	Load func(M) string

	// Store writes the submitted address to the model. Without it the field is read-only.
	Store func(*M, string)

	// Rules are checked in the browser and again on the server.
	Rules StringRules

	// Placeholder is shown in the input while it is empty.
	Placeholder string

	// NoSort leaves the sort link off the column's header, in a table: it cannot be sorted by,
	// and a sort by it in the address is ignored. It does nothing in a form.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table: it cannot be filtered by,
	// and a filter on it in the address is ignored. It does nothing in a form.
	NoFilter bool
}

func (a Website[M]) label() string { return a.Label }

func (a Website[M]) exclusions() (noSort, noFilter bool) { return a.NoSort, a.NoFilter }

func (Website[M]) toolbarItem(M) {}

// ColorPicker projects a string field of M that holds a color, "#rrggbb", chosen with the browser's picker. A picker cannot be empty: a field
// whose Load returns nothing shows black, and a submission always holds a color, so it has no Required, and no rule of the length
// or the text of a string. The server checks that what is submitted is "#" and six hex digits.
type ColorPicker[M any] struct {
	// Label is the column header and the input's label. In a table it also names the
	// column in the address and in a [Query].
	Label string

	// Load reads the color from the model, as "#rrggbb".
	Load func(M) string

	// Store writes the chosen color to the model. Without it the field is read-only.
	Store func(*M, string)

	// NoSort leaves the sort link off the column's header, in a table: it cannot be sorted by,
	// and a sort by it in the address is ignored. It does nothing in a form.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table: it cannot be filtered by,
	// and a filter on it in the address is ignored. It does nothing in a form.
	NoFilter bool
}

func (a ColorPicker[M]) label() string { return a.Label }

func (a ColorPicker[M]) exclusions() (noSort, noFilter bool) { return a.NoSort, a.NoFilter }

func (ColorPicker[M]) toolbarItem(M) {}

// validateTextRules checks what the rules of a text say of themselves: that the pattern can be read by both engines, that the lengths
// agree, and that no value could satisfy them all.
func validateTextRules(v *bodyValidator, at string, r StringRules) {
	if p := r.Pattern; p != nil {
		if _, err := rules.Compile(p.Expr, p.Message); err != nil {
			fix := "Rewrite the pattern in RE2 syntax."
			var pe *rules.PatternError
			if errors.As(err, &pe) {
				fix = pe.Fix
			}
			v.add(fmt.Sprintf("%s: %v", at, err), fix)
		}
	}
	if r.MinLen < 0 || r.MaxLen < 0 || (r.MaxLen > 0 && r.MinLen > r.MaxLen) {
		v.add(at+" has inconsistent length rules", "MinLen and MaxLen must be non-negative with MinLen <= MaxLen.")
	}
	for _, p := range rules.Unsatisfiable(lowerStringRules(r)) {
		v.add(at+": "+p[0], p[1])
	}
}

// lowerFormatted is a string field of a kind of text that has an input of its own, as the IR holds it.
func lowerFormatted[M any](name, label string, format ir.Format, r StringRules, placeholder string, load func(M) string, store func(*M, string)) ir.Field {
	f := head(name, label, ir.KindString)
	f.Rules, f.Placeholder = lowerStringRules(r), placeholder
	f.Rules.Format = format
	f.Get = func(m any) string { return load(m.(M)) }
	f.Set = setter(store, func(raw string) (string, error) { return raw, nil })
	f.Unset = unsetter(store)
	return f
}
