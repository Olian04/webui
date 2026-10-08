package args

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Olian04/webui/internal/ir"
)

// Problem is one thing wrong with an argument struct. The caller attaches the
// page coordinates; this package knows only the type.
type Problem struct {
	Detail string
	Fix    string
}

// ViewSep separates a leaf's ID from the name of one of its view parameters in
// the address ("devices.offset"). Argument names may not contain it, so a view
// parameter can never collide with a page argument.
const ViewSep = "."

// ViewKey is the address parameter for one piece of a leaf's view state.
func ViewKey(id, param string) string { return id + ViewSep + param }

// FilterKey is the address parameter for the filter on one column of a table:
// "devices.filter.status". The column is named by its sort key.
func FilterKey(id, column string) string { return ViewKey(id, "filter."+column) }

// RangeKeys are the address parameters for the bounds of a numeric column's
// filter: "devices.min.count" and "devices.max.count".
func RangeKeys(id, column string) (lower, upper string) {
	return ViewKey(id, "min."+column), ViewKey(id, "max."+column)
}

// FromKey is the address parameter that remembers where the user came from, so
// a form can return there. It is the library's, and it contains ViewSep, so it
// is carried through the page's own links like any other view state. Its value
// is an address, which may itself carry one: a chain of pages unwinds in order.
const FromKey = "webui" + ViewSep + "from"

// Slug is the name a label has in an address: lower-case letters and digits,
// each run of anything else a single dash, none at the ends. "Mean rate / s" is
// "mean-rate-s". It is empty when the label has no letter or digit to name it by.
func Slug(label string) string {
	var b strings.Builder
	dash := false
	for _, r := range label {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(unicode.ToLower(r))
		} else {
			dash = true
		}
	}
	return b.String()
}

// IsViewKey reports whether an address parameter is view state, not an argument.
func IsViewKey(key string) bool { return strings.Contains(key, ViewSep) }

// tag is the struct tag that renames an argument. `webui:"-"` hides a field.
const tag = "webui"

// Name derives a field's argument name: its lower-cased Go name, or the
// `webui` tag when present. The empty string means the field is not an
// argument (unexported or tagged "-").
func Name(f reflect.StructField) string {
	if !f.IsExported() {
		return ""
	}
	if v, ok := f.Tag.Lookup(tag); ok {
		if v == "-" {
			return ""
		}
		if v != "" {
			return v
		}
	}
	return strings.ToLower(f.Name)
}

// KindOf maps a Go type to its argument kind. ok is false for anything a URL
// cannot carry as one value.
func KindOf(t reflect.Type) (kind ir.ValueKind, ok bool) {
	switch t.Kind() {
	case reflect.String:
		return ir.KindString, true
	case reflect.Bool:
		return ir.KindBool, true
	case reflect.Int:
		return ir.KindInt, true
	case reflect.Int64:
		return ir.KindInt64, true
	case reflect.Float64:
		return ir.KindFloat, true
	default:
		return 0, false
	}
}

// Placeholders lists the {name} segments of a path template, in order.
func Placeholders(template string) []string {
	var names []string
	for _, seg := range strings.Split(template, "/") {
		if len(seg) > 2 && seg[0] == '{' && seg[len(seg)-1] == '}' {
			names = append(names, seg[1:len(seg)-1])
		}
	}
	return names
}

// Spec resolves an argument struct against a path template. Every problem is
// collected rather than reported one at a time. When the type is not a struct
// no later check can run, so only that problem is returned.
func Spec(t reflect.Type, template string) ([]ir.ArgSpec, []Problem) {
	if t.Kind() != reflect.Struct {
		return nil, []Problem{{
			Detail: fmt.Sprintf("the argument type %s is not a struct", t),
			Fix:    "Use a struct, or webui.NoArgs for a page without arguments.",
		}}
	}

	var (
		specs    []ir.ArgSpec
		problems []Problem
		owner    = map[string]string{}
	)
	inPath := map[string]bool{}
	for _, name := range Placeholders(template) {
		inPath[name] = true
	}

	for i := range t.NumField() {
		f := t.Field(i)
		name := Name(f)
		if name == "" {
			continue
		}
		kind, ok := KindOf(f.Type)
		if !ok {
			problems = append(problems, Problem{
				Detail: fmt.Sprintf("field %s has unsupported type %s", f.Name, f.Type),
				Fix:    "Arguments must be string, bool, int, int64 or float64 — a URL carries one value per name.",
			})
			continue
		}
		if strings.Contains(name, ViewSep) {
			problems = append(problems, Problem{
				Detail: fmt.Sprintf("field %s maps to the argument %q, which contains %q", f.Name, name, ViewSep),
				Fix:    fmt.Sprintf("Drop the %q from the name. It is reserved for table and tab state, such as devices.offset.", ViewSep),
			})
			continue
		}
		if first, dup := owner[name]; dup {
			problems = append(problems, Problem{
				Detail: fmt.Sprintf("two fields both map to the argument %q (%s and %s)", name, first, f.Name),
				Fix:    "Rename one field, or give it a distinct name with a `webui:\"...\"` tag.",
			})
			continue
		}
		owner[name] = f.Name
		specs = append(specs, ir.ArgSpec{Name: name, Field: f.Name, Kind: kind, InPath: inPath[name]})
	}

	for _, name := range Placeholders(template) {
		if _, ok := owner[name]; ok {
			continue
		}
		problems = append(problems, Problem{
			Detail: fmt.Sprintf("the path declares {%s} but %s has no field for it", name, typeName(t)),
			Fix: fmt.Sprintf("Add a field named %s to %s, or change the placeholder to match an existing field.",
				capitalise(name), typeName(t)),
		})
	}
	return specs, problems
}

func typeName(t reflect.Type) string {
	if n := t.Name(); n != "" {
		return n
	}
	return t.String()
}

func capitalise(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}
